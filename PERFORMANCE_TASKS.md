# PERFORMANCE_TASKS.md – Kế hoạch tối ưu tốc độ hquizlet-platform

> Mục tiêu: giảm thời gian tải trang và độ trễ API mà không thêm hạ tầng nặng (không Kafka).
> Dùng Redis/NATS sẵn có. Làm lần lượt từ Task 1 → 6, mỗi task một PR riêng.
> Đoạn code bên dưới là **mẫu**, cần chỉnh theo cấu trúc thực tế của repo.

## Quy tắc chung (đọc trước khi làm)

- **KHÔNG cache** nội dung quiz/thẻ được bảo vệ (`protected_quiz`, `quiz-audio`, endpoint trả `Cache-Control: private, no-store`).
- Chỉ cache: metadata, danh sách công khai, kết quả tìm kiếm, template, kết quả verify token.
- Cache theo user thì key **bắt buộc có user ID**.
- Không commit secret/token vào repo. Dùng `.env`.
- Mỗi task phải có test hoặc số đo trước/sau (xem phần Đo lường).

---

## Task 0 – Đo baseline (làm trước, ~30 phút)

thưc hiện thêm 
Bảo mật (làm trước)

Không có HTTPS. nginx.conf ghi rõ "dev, HTTP only" và không có chứng chỉ. Token đăng nhập đang đi qua mạng dạng rõ. Cần domain, Let's Encrypt (hoặc Cloudflare), HSTS.
Lưu lượng DB giữa 2 server không mã hoá. sslmode=disable, Redis và MinIO dùng http, trên dải IPv6 công khai của Hurricane Electric. Firewall ufw có giúp nhưng không mã hoá. Nên dựng WireGuard hoặc Tailscale giữa 3 server (như DEPLOY.md đã gợi ý) rồi bind cổng vào địa chỉ mạng riêng đó, và bật TLS cho Postgres.
Gateway chưa có rate limit. Trong code còn dòng "Basic rate limit could be added here". Endpoint đăng nhập và join live quiz đang để ngỏ; có thể làm tạm bằng limit_req của nginx.
Một số thông tin dùng chung: MinIO root key được dùng làm access key cho các service, ADMIN_TOKEN là chuỗi tĩnh, CI SSH bằng root. Nên tạo user MinIO riêng theo service, user SSH không phải root.
Container chạy bằng root (Dockerfile không có USER).


- [ ] Chạy stack: `docker compose -f infra/docker/docker-compose.yml up --build`
- [ ] Đo API bằng `hey` hoặc `wrk`, ghi lại p50/p95:
  ```bash
  hey -n 500 -c 20 -H "Authorization: Bearer $TOKEN" http://localhost/api/v1/study-sets
  ```
- [ ] Đo frontend bằng Lighthouse (Chrome DevTools) cho trang chủ và trang study set; ghi LCP, TTFB, tổng kích thước JS.
- [ ] Lưu kết quả vào `docs/perf/baseline.md`.

**Hoàn thành khi:** có file baseline để so sánh.

---

## Task 1 – Cache verify token ở Gateway (ưu tiên cao)

**Vấn đề:** mỗi request gọi `GET /internal/auth/verify` → auth query Postgres (`GetSessionIdentity`).

**File:** `services/gateway/main.go` (hàm `verifyIdentity`)

**Cách làm:**

1. Thêm cache in-memory có TTL (đủ dùng cho 1 gateway; chuyển Redis nếu chạy nhiều replica).
2. Key = SHA-256 của token (không lưu token thô). TTL 30–60 giây.
3. Chỉ cache kết quả thành công (`Authenticated == true`).
4. Khi gateway nhận `POST /auth/logout`, `/auth/logout-all`, `/auth/refresh` thì xoá key tương ứng.

```go
// services/gateway/tokencache.go (mẫu)
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type cachedIdentity struct {
	identity verifiedIdentity
	expires  time.Time
}

type tokenCache struct {
	mu  sync.RWMutex
	m   map[string]cachedIdentity
	ttl time.Duration
}

func newTokenCache(ttl time.Duration) *tokenCache {
	c := &tokenCache{m: make(map[string]cachedIdentity), ttl: ttl}
	go c.janitor()
	return c
}

func tokenKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (c *tokenCache) Get(token string) (verifiedIdentity, bool) {
	c.mu.RLock()
	v, ok := c.m[tokenKey(token)]
	c.mu.RUnlock()
	if !ok || time.Now().After(v.expires) {
		return verifiedIdentity{}, false
	}
	return v.identity, true
}

func (c *tokenCache) Set(token string, id verifiedIdentity) {
	c.mu.Lock()
	c.m[tokenKey(token)] = cachedIdentity{id, time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *tokenCache) Delete(token string) {
	c.mu.Lock()
	delete(c.m, tokenKey(token))
	c.mu.Unlock()
}

func (c *tokenCache) janitor() {
	for range time.Tick(time.Minute) {
		now := time.Now()
		c.mu.Lock()
		for k, v := range c.m {
			if now.After(v.expires) {
				delete(c.m, k)
			}
		}
		c.mu.Unlock()
	}
}
```

Trong `verifyIdentity`: kiểm tra `cache.Get(token)` trước khi gọi auth; sau khi auth trả OK thì `cache.Set`.

**Đánh đổi:** token bị revoke có thể còn hiệu lực tối đa `ttl` giây ở gateway. Chấp nhận được với TTL ≤ 60s; ghi rõ trong `docs/security/`.

**Hoàn thành khi:**
- [ ] `go test ./...` trong `services/gateway` pass, có test cache hit/miss/expire/delete.
- [ ] p95 của API có auth giảm so với baseline.
- [ ] Token sai/hết hạn vẫn trả 401.

---

## Task 2 – Bỏ verify lần 2 ở Study service

**Vấn đề:** `services/study/internal/middleware/verify_bearer.go` gọi auth verify thêm một lần nữa sau gateway.

**Cách làm (HMAC nội bộ):**

1. Thêm biến môi trường `INTERNAL_SIGNING_KEY` (dài, ngẫu nhiên) cho gateway và study trong `docker-compose.yml` và `.env.example`.
2. Gateway, sau khi verify, thêm header `X-Internal-Sig` = HMAC-SHA256 của `userID|role|unixTimestamp` và `X-Internal-Ts`.
3. Study middleware: nếu có chữ ký hợp lệ và timestamp lệch ≤ 30s thì tin `X-User-ID`/`X-User-Role`, **bỏ qua** gọi auth. Nếu không có chữ ký thì fallback về logic verify cũ (giữ an toàn cho gọi trực tiếp).
4. Áp dụng tương tự cho các service khác nếu cùng pattern (`class`, `payment`).

**Hoàn thành khi:**
- [ ] Gọi trực tiếp study với `X-User-ID` giả (không chữ ký) vẫn bị 401.
- [ ] Request qua gateway chỉ gọi auth tối đa 1 lần (kiểm tra bằng log hoặc counter).
- [ ] Test cho chữ ký sai, hết hạn, thiếu header.

---

## Task 3 – Nginx phục vụ build production thay vì Vite dev server

**Vấn đề:** `infra/nginx/nginx.conf` proxy `/` tới `web:5173` (Vite dev, không nén, không cache). Thư mục `web-dist/` đã có sẵn.

**Cách làm:**

1. Build: `cd apps/web && npm ci && npm run build` (ra `dist/`).
2. Tạo `infra/docker/web-prod.Dockerfile`: multi-stage, stage build Node → copy `dist` vào image nginx (hoặc mount volume).
3. Sửa `nginx.conf` (production):

```nginx
gzip on;
gzip_comp_level 5;
gzip_min_length 1024;
gzip_types text/css application/javascript application/json image/svg+xml;

# File có hash trong tên: cache 1 năm
location /assets/ {
    root /usr/share/nginx/html;
    add_header Cache-Control "public, max-age=31536000, immutable";
    access_log off;
}

# SPA: index.html không cache lâu để deploy mới có hiệu lực ngay
location / {
    root /usr/share/nginx/html;
    try_files $uri /index.html;
    add_header Cache-Control "no-cache";
}
```

4. Giữ nguyên block SSE (`proxy_buffering off`) và `/api/`. Giữ file dev riêng (`nginx.dev.conf`) cho HMR.
5. (Tuỳ chọn) bật `http2` khi có TLS; brotli nếu image hỗ trợ.

**Hoàn thành khi:**
- [ ] `curl -I -H "Accept-Encoding: gzip" http://localhost/assets/<file>.js` có `Content-Encoding: gzip` và `Cache-Control` immutable.
- [ ] Lighthouse LCP và tổng kích thước tải giảm so với baseline.
- [ ] SSE live quiz vẫn hoạt động.

---

## Task 4 – Sửa cache file tĩnh `/files/` (MinIO)

**Vấn đề:** `proxy_cache_valid 200 7d;` không có tác dụng vì thiếu `proxy_cache_path` và `proxy_cache`.

**Cách làm:** trong khối `http {}`:

```nginx
proxy_cache_path /var/cache/nginx/files levels=1:2 keys_zone=files_cache:50m
                 max_size=1g inactive=7d use_temp_path=off;
```

Trong `location /files/`:

```nginx
proxy_cache files_cache;
proxy_cache_key $uri;
proxy_cache_valid 200 7d;
proxy_cache_use_stale error timeout updating;
add_header X-Cache-Status $upstream_cache_status;
```

Mount volume cho `/var/cache/nginx` trong compose.

**Lưu ý:** chỉ áp dụng cho asset công khai (avatar, ảnh). **Không** cache tài liệu import hoặc file riêng tư; tách prefix nếu cần.

**Hoàn thành khi:** request thứ hai tới cùng file trả `X-Cache-Status: HIT`.

---

## Task 5 – Khôi phục phiên nhanh ở Frontend

**Vấn đề:** `apps/web/src/features/auth/AuthContext.tsx` gọi `authApi.me()` rồi mới có `user`, gây màn hình trắng/nhấp nháy.

**Cách làm:**

1. Khi đăng nhập/`me` thành công, lưu thêm `hquizlet.user` (chỉ thông tin hiển thị: id, name, image, role) vào localStorage.
2. Khởi tạo `useState<User | null>` từ giá trị đã lưu → render ngay.
3. Vẫn gọi `me()` ở nền để xác thực; nếu 401 thì `clearSession()` (xoá cả token và user).
4. Xoá `hquizlet.user` khi logout.

```tsx
const USER_KEY = "hquizlet.user";
const [user, setUser] = useState<User | null>(() => {
  try { return JSON.parse(localStorage.getItem(USER_KEY) ?? "null"); }
  catch { return null; }
});
```

**Lưu ý bảo mật:** không lưu dữ liệu nhạy cảm; UI dựa vào `user` cache chỉ để hiển thị, quyền thật luôn do server quyết định.

**Hoàn thành khi:** reload trang không còn trạng thái "chưa đăng nhập" thoáng qua; test vitest cho logout xoá sạch storage.

---

## Task 6 – Cache dữ liệu phía Frontend (TanStack Query)

**Vấn đề:** các màn hình trong `apps/web/src/features/*` dùng `useEffect` + fetch, chuyển trang là tải lại.

**Cách làm:**

1. `cd apps/web && npm i @tanstack/react-query`
2. Bọc app trong `QueryClientProvider` (`main.tsx`):
   ```tsx
   const queryClient = new QueryClient({
     defaultOptions: { queries: { staleTime: 30_000, gcTime: 5 * 60_000, refetchOnWindowFocus: false } },
   });
   ```
3. Chuyển lần lượt các màn hình đọc nhiều: `dashboard`, `study-sets` (danh sách và chi tiết), `folders`, `classes`, `search`.
4. Dùng `useMutation` + `invalidateQueries` cho thao tác tạo/sửa/xoá.
5. Với `SearchDropdown`: debounce 250–300ms và huỷ request cũ (`AbortController`).
6. **Không** cache nội dung của `ProtectedQuizPlayer` ở tầng này (đặt `gcTime: 0`, hoặc không dùng query cho endpoint đó).

**Hoàn thành khi:**
- [ ] Quay lại trang đã xem hiển thị tức thì (không spinner).
- [ ] Số request trùng lặp trong tab Network giảm.
- [ ] `npm run build` và `npx vitest run` pass.

---

## Task 7 – Cache phía server cho danh sách/tìm kiếm công khai (làm sau cùng)

**Cách làm:**

1. Dùng Redis đã có trong compose (`REDIS_URL`) cho study service.
2. Cache các endpoint đọc công khai: danh sách study set công khai, kết quả tìm kiếm, `/v1/templates/*`.
3. Key: `study:public-list:{page}:{size}`, `study:search:{sha256(query)}`; TTL 30–120 giây.
4. Xoá/làm mới key khi có tạo/sửa/xoá study set liên quan (hoặc chấp nhận TTL ngắn).
5. Thêm header `Cache-Control: public, max-age=30` cho các endpoint công khai thật sự.
6. Kiểm tra chỉ mục Postgres cho truy vấn tìm kiếm/danh sách (`EXPLAIN ANALYZE`); thêm index nếu thiếu (migration trong `services/study/migrations/`).

**Hoàn thành khi:** endpoint đã cache trả nhanh ở lần gọi thứ hai; không endpoint nào có dữ liệu riêng tư/được bảo vệ bị cache.

---

## Đo lường sau khi xong

- [ ] Chạy lại Task 0, ghi vào `docs/perf/after.md`.
- [ ] So sánh: p95 API, TTFB, LCP, kích thước JS tải lần đầu.
- [ ] Mục tiêu tham khảo: p95 API có auth giảm ≥ 30%, LCP giảm ≥ 30%.

## Thứ tự và ước lượng

| Task | Nội dung | Ưu tiên | Ước lượng |
|---|---|---|---|
| 0 | Đo baseline | Bắt buộc | 0,5 giờ |
| 3 | Nginx production + gzip | Cao | 2–3 giờ |
| 1 | Cache verify token ở gateway | Cao | 2–3 giờ |
| 2 | Bỏ verify lần 2 ở study | Cao | 3–4 giờ |
| 4 | Cache `/files/` | Trung bình | 1 giờ |
| 5 | Khôi phục phiên nhanh | Trung bình | 1 giờ |
| 6 | TanStack Query | Trung bình | 1–2 ngày |
| 7 | Redis cache + index | Thấp | 1–2 ngày |


