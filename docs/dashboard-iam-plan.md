# Kế hoạch: Dashboard (tài khoản / lớp học / admin) + Phân quyền kiểu AWS IAM

> Trạng thái: **bản đề xuất, chưa code**. Mọi quyết định đánh dấu **[Đề xuất]** là mặc định mình khuyên dùng; bạn có thể đổi ở mục 9.
> Phạm vi hiện tại: 2 loại tài khoản **Giáo viên** và **Học sinh**, thêm **Admin** của hệ thống. Thiết kế chừa chỗ để mở rộng **Tổ chức (trường học)** sau này mà không phải làm lại.

---

## 1. Hiện trạng (đã đọc trong repo)

| Khu vực | Hiện tại | Hệ quả cho kế hoạch |
|---|---|---|
| Vai trò toàn hệ thống | `users.role` là chuỗi, chỉ có `user` / `admin` (`services/auth`) | Chưa có khái niệm "giáo viên/học sinh" ở cấp tài khoản |
| Vai trò trong lớp | `class_members.role` ∈ `owner/teacher/student` (CHECK trong migration 002) | Đây đã là "quyền theo phạm vi lớp", tái sử dụng được |
| Kiểm tra quyền | Rải rác, viết cứng: `role != "admin"` (payment), `ErrForbidden` khi không phải owner (class service) | Mỗi service tự quyết → dễ lệch, khó thêm vai trò mới |
| Gateway | Xác thực token, xoá header giả mạo, gắn `X-User-ID`, `X-User-Role` có ký HMAC (`services/gateway/main.go`) | Điểm tốt để truyền thêm danh sách vai trò; **giữ nguyên cơ chế chống giả mạo** |
| Admin | Chỉ có `AdminPayments.tsx` + route `/v1/admin/payments/*`, `/v1/admin/wallet/*` | Chưa có console admin thật |
| Dashboard web | `features/dashboard/Dashboard.tsx` thực chất là **thư viện học phần** (danh sách study set) | Dashboard đúng nghĩa (số liệu, tiến độ) chưa tồn tại |
| Điều hướng web | Không có router: `main.tsx` (357 dòng) dùng `view` state + `VIEW_PATHS` + `pushState` | Dashboard lồng nhau (`/admin/users/…`, `/classes/:id/dashboard`) sẽ rất khó trên cơ chế này → cần thêm router |
| Dữ liệu cho số liệu | Có `activity_events` (class service), `learning_sessions`/progress (study), `orders/wallet` (payment), NATS + outbox | Đủ nguồn để dựng dashboard, thiếu phần **tổng hợp** |
| Thư viện biểu đồ | Chưa có (chỉ React 18) | Cần chọn 1 thư viện |

---

## 2. Mục tiêu & không-mục-tiêu

**Mục tiêu**
1. Mỗi tài khoản có dashboard riêng (khác nhau giữa giáo viên và học sinh).
2. Mỗi lớp có dashboard (giáo viên thấy toàn lớp, học sinh thấy phần của mình).
3. Admin có console rộng hơn, phong cách YouTube Studio (menu trái, tổng quan số liệu, quản lý nội dung/người dùng).
4. Phân quyền kiểu AWS IAM: *principal – role – policy – action – resource*, **mặc định từ chối, Deny luôn thắng Allow**.
5. Mở rộng được sang **Tổ chức** mà không đổi mô hình.

**Không-mục-tiêu (làm sau, đừng làm bây giờ)**
- Trình soạn policy JSON tuỳ ý cho người dùng cuối.
- SSO/SAML, SCIM cho trường học.
- Phân tích nâng cao (cohort, dự đoán học sinh bỏ học).

---

## 3. Thiết kế phân quyền (kiểu AWS IAM, bản gọn cho dự án này)

### 3.1 Khái niệm → tương ứng AWS

| Khái niệm của mình | Tương đương AWS | Ghi chú |
|---|---|---|
| **Principal** | IAM User / Role session | Hiện chỉ là `user`. Chừa `principal_type` để sau thêm `service`/`api_key` |
| **Action** | `s3:GetObject` | Dạng `dịch-vụ:ĐộngTừ`, vd `class:Update` |
| **Resource** | ARN | Dạng `hq:class/123`, `hq:study-set/45`, `hq:org/7` |
| **Policy** | IAM Policy (JSON) | Tập các statement Allow/Deny |
| **Role** | IAM Role | Một bộ policy có tên: `Teacher`, `Student`, `PlatformAdmin`… |
| **Assignment** | Attach role | Gán *role* cho *principal* **trong một phạm vi (scope)** |
| **Scope** | Account/OU/Resource | `platform` → `org` → `class`. Gán ở scope cha thì áp xuống con |

### 3.2 Quy tắc đánh giá (giống AWS)
1. Mặc định **từ chối**.
2. Thu thập mọi statement từ các role mà user có **ở scope của resource và các scope cha** (class → org → platform).
3. Có statement **Deny** khớp → **từ chối**.
4. Ngược lại có statement **Allow** khớp → **cho phép**.
5. Còn lại → từ chối.

### 3.3 Dạng policy (JSON, lưu trong code ở giai đoạn đầu)
```json
{
  "version": "2026-10-01",
  "statements": [
    {
      "effect": "Allow",
      "actions": ["class:Read", "class:ViewDashboard", "class:ListMembers"],
      "resources": ["hq:class/*"]
    },
    {
      "effect": "Deny",
      "actions": ["class:Delete"],
      "resources": ["hq:class/*"],
      "condition": { "resourceOwner": "notSelf" }
    }
  ]
}
```
**[Đề xuất]** chỉ hỗ trợ **2 điều kiện** ở bản đầu: `resourceOwner: self|notSelf` và `inScope` (resource thuộc đúng scope đã gán). Đủ cho giáo viên/học sinh; thêm điều kiện sau khi có nhu cầu thật.

### 3.4 Role dựng sẵn (managed, không sửa được qua UI)

| Role | Scope được gán | Mô tả |
|---|---|---|
| `PlatformAdmin` | platform | Toàn quyền hệ thống, xem dashboard admin, quản lý user/role |
| `PlatformSupport` | platform | Chỉ đọc user/lớp/đơn hàng, không sửa tiền, không xoá (để dành cho nhân viên hỗ trợ) |
| `Teacher` | platform | Tạo lớp, tạo/sửa/xoá học phần của mình, tổ chức live quiz, import/export |
| `Student` | platform | Học, làm bài, tham gia lớp bằng mã mời, mua học phần |
| `ClassOwner` | class | Toàn quyền trong lớp (kể cả xoá lớp, đổi mã mời, đổi vai trò) |
| `ClassTeacher` | class | Giáo viên phụ: giao bài, xem tiến độ, quản lý học sinh, **không** xoá lớp |
| `ClassStudent` | class | Xem nội dung được giao, làm bài, xem tiến độ của mình |
| `OrgAdmin` *(sau)* | org | Quản trị trường: tạo lớp trong trường, quản lý giáo viên/học sinh |
| `OrgTeacher`, `OrgStudent` *(sau)* | org | Thành viên trường; kế thừa xuống các lớp thuộc trường |

> Một người có thể là `Teacher` (platform) **và** `ClassStudent` ở lớp của giáo viên khác — đúng thực tế.

### 3.5 Danh mục Action (khởi điểm)

| Nhóm | Actions |
|---|---|
| `study-set` | `Create` `Read` `Update` `Delete` `Publish` `Import` `Export` `SetPrice` |
| `quiz` | `Attempt` `ViewAnswers` `ViewResults` |
| `class` | `Create` `Read` `Update` `Delete` `ResetInvite` `Join` `ViewDashboard` |
| `class-member` | `List` `Add` `Remove` `UpdateRole` `ViewProgress` |
| `class-content` | `AssignStudySet` `UnassignStudySet` `UploadVideo` `ViewVideo` |
| `live` | `Host` `Join` |
| `wallet` / `order` | `wallet:Read` `order:Create` `order:ListAll` `wallet:Credit` |
| `user` | `ReadSelf` `UpdateSelf` `List` `Disable` |
| `iam` | `ListRoles` `ListAssignments` `AssignRole` `RevokeRole` |
| `dashboard` | `ViewSelf` `ViewAdmin` |
| `audit` | `Read` |
| `org` *(sau)* | `Create` `Read` `Update` `InviteMember` `ViewDashboard` |

### 3.6 Nơi đặt và cách các service dùng

**[Đề xuất] không tạo service IAM riêng ngay.** Thực tế gọn nhất:

- **Thư viện dùng chung** `packages/authz` (module Go mới; `go.work` hiện chỉ liệt kê 7 service nên cần thêm dòng `use ./packages/authz`): chứa catalog action, role dựng sẵn, và hàm
  `Can(principal, action, resource) bool`. Mọi service import thư viện này → **một logic duy nhất**, hết kiểu `role != "admin"` rải rác.
- **Nguồn dữ liệu gán quyền:**
  - Quyền cấp **platform** (Teacher/Student/Admin…): bảng mới trong **auth service** (nơi đã có `users`, `sessions`).
  - Quyền cấp **class**: **giữ `class_members` làm nguồn sự thật** (ánh xạ `owner/teacher/student` → `ClassOwner/ClassTeacher/ClassStudent`). Không phải đồng bộ dữ liệu giữa 2 service, rủi ro migration thấp.
  - Quyền cấp **org** (sau): bảng trong auth service, cùng cơ chế.
- **Gateway** chỉ làm kiểm tra thô (vd `/v1/admin/*` cần action `dashboard:ViewAdmin`). **Kiểm tra chi tiết nằm ở từng service** (vì chỉ service đó biết resource thuộc ai).
- Truyền danh sách vai trò platform qua header ký HMAC mới `X-User-Roles` (giữ `X-User-Role` một thời gian cho tương thích). Gateway **vẫn xoá mọi header này từ client** như đang làm.
- Web gọi `GET /v1/me/permissions` để biết ẩn/hiện nút (`useCan("class:Update", {classId})`). **Ẩn nút chỉ để cho đẹp; server luôn kiểm tra lại.**

### 3.7 Cấu trúc dữ liệu mới (auth service)

```sql
-- Role & policy dựng sẵn nằm trong code; bảng này để admin xem + chuẩn bị role tuỳ chỉnh theo tổ chức
CREATE TABLE iam_roles (
  id           BIGSERIAL PRIMARY KEY,
  key          TEXT NOT NULL,                 -- 'Teacher', 'ClassOwner'...
  name         TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  kind         TEXT NOT NULL DEFAULT 'managed' CHECK (kind IN ('managed','custom')),
  org_id       BIGINT NULL,                   -- NULL = toàn hệ thống
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (key, org_id)
);

CREATE TABLE iam_assignments (
  id             BIGSERIAL PRIMARY KEY,
  principal_type TEXT   NOT NULL DEFAULT 'user',
  principal_id   BIGINT NOT NULL,
  role_key       TEXT   NOT NULL,
  scope_type     TEXT   NOT NULL CHECK (scope_type IN ('platform','org')),  -- class lấy từ class_members
  scope_id       BIGINT NULL,
  created_by     BIGINT NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at     TIMESTAMPTZ NULL,
  UNIQUE (principal_type, principal_id, role_key, scope_type, scope_id)
);
CREATE INDEX iam_assignments_principal_idx ON iam_assignments(principal_type, principal_id);

CREATE TABLE audit_log (
  id          BIGSERIAL PRIMARY KEY,
  actor_id    BIGINT NOT NULL,
  action      TEXT   NOT NULL,                -- 'iam:AssignRole', 'user:Disable'...
  resource    TEXT   NOT NULL,                -- 'hq:user/42'
  result      TEXT   NOT NULL CHECK (result IN ('allow','deny')),
  request_id  TEXT,
  ip          INET,
  detail      JSONB,
  at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_at_idx ON audit_log(at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log(actor_id, at DESC);

-- Chừa sẵn cho tổ chức (tạo bảng ở Phase 5, nhưng THIẾT KẾ cột org_id ngay từ bây giờ)
-- organizations(id, name, slug, kind 'school', owner_user_id, created_at)
-- classes.org_id BIGINT NULL  (thêm ở Phase 5; NULL = lớp cá nhân)
```

### 3.8 Đăng ký tài khoản & chống leo thang quyền
- Đăng ký có bước chọn **"Tôi là giáo viên / học sinh"** → gán role `Teacher` hoặc `Student` ở scope platform.
- **Không** cho tự gán `PlatformAdmin`/`OrgAdmin`; chỉ admin gán (có ghi `audit_log`).
- Người không có `iam:AssignRole` không gọi được API gán quyền; test riêng cho việc này.
- **[Đề xuất]** Giai đoạn đầu cho tự chọn Teacher; khi lên trường học, giáo viên của trường được **OrgAdmin mời** (xem câu hỏi mở #2).

---

## 4. Thiết kế Dashboard

### 4.1 Dashboard cá nhân (`/dashboard`) — khác nhau theo vai trò

**Học sinh**
- *Tiếp tục học* (học phần/bài đang dở), chuỗi ngày học, mục tiêu hôm nay
- *Lớp của tôi* + bài được giao còn hạn
- *Kết quả gần đây* (điểm quiz, thẻ cần ôn)
- *Đã mua* / số dư ví

**Giáo viên**
- *Lớp của tôi*: sĩ số, hoạt động 7 ngày
- *Cần chú ý*: học sinh không hoạt động ≥ 7 ngày, bài sắp hết hạn chưa ai làm
- *Học phần của tôi*: lượt dùng, điểm trung bình
- *Hành động nhanh*: tạo lớp, giao bài, bắt đầu live quiz

```
┌ Sidebar ┬──────────────────────────────────────────────┐
│ Tổng quan│  Xin chào, <tên>                    [+ Tạo]  │
│ Lớp học │  ┌──────┐ ┌──────┐ ┌──────┐ ┌──────┐         │
│ Học phần│  │ KPI 1│ │ KPI 2│ │ KPI 3│ │ KPI 4│         │
│ Thư mục │  └──────┘ └──────┘ └──────┘ └──────┘         │
│ Ví      │  ┌ Hoạt động 28 ngày ──┐ ┌ Cần chú ý ───────┐│
│ ...     │  │  (biểu đồ đường)    │ │ • …              ││
│         │  └─────────────────────┘ └──────────────────┘│
└─────────┴──────────────────────────────────────────────┘
```

### 4.2 Dashboard lớp (`/classes/:id/dashboard`)
Cần `class:ViewDashboard`.
- **Giáo viên/Owner:** sĩ số & thành viên hoạt động 7 ngày, tỷ lệ hoàn thành từng bài được giao, điểm trung bình, bảng học sinh (sắp xếp theo tiến độ/điểm/lần cuối hoạt động), dòng thời gian hoạt động, xuất CSV.
- **Học sinh:** tiến độ của mình, bài được giao, bảng xếp hạng (**[Đề xuất]** chỉ hiện nếu giáo viên bật).
- Học sinh **không** thấy điểm/hoạt động của bạn khác (Deny qua `class-member:ViewProgress` + điều kiện `resourceOwner: self`).

### 4.3 Admin console (`/admin`) — kiểu YouTube Studio
Menu trái cố định, vùng chính có chọn khoảng thời gian (7 / 28 / 90 ngày) và so sánh với kỳ trước.

| Mục menu | Nội dung |
|---|---|
| **Tổng quan** | KPI (người dùng mới, người dùng hoạt động ngày/tuần, phiên học, lớp mới, doanh thu) kèm % so kỳ trước + sparkline; biểu đồ theo thời gian; "Nội dung nổi bật"; "Hoạt động mới nhất" |
| **Người dùng** | Tìm kiếm, lọc theo vai trò/trạng thái, xem chi tiết, vô hiệu hoá, gán/thu hồi role |
| **Nội dung** | Tất cả học phần (mọi chủ sở hữu), ẩn/gỡ nội dung vi phạm, thống kê lượt dùng |
| **Lớp học** | Danh sách lớp, sĩ số, hoạt động; can thiệp khi cần |
| **Thanh toán** | Chuyển `AdminPayments.tsx` hiện có vào đây + thống kê doanh thu |
| **Vai trò & quyền** | Xem role dựng sẵn, danh sách ai đang giữ role nào (chỉ xem ở bản đầu) |
| **Nhật ký (Audit)** | Ai làm gì, lúc nào, kết quả; lọc theo người/hành động |
| **Hệ thống** | Trạng thái dịch vụ (tận dụng `/healthz/services`), hàng đợi import, lỗi gần đây |
| **Tổ chức** *(sau)* | Danh sách trường, duyệt/tạo trường |

Admin **không** dùng chung màn hình với người dùng thường: layout riêng (`AdminShell`), route riêng, gateway chặn thô theo `dashboard:ViewAdmin`.

### 4.4 Nguồn số liệu (cách làm cho nhẹ)

- **Không tạo service mới ngay.** Mỗi service thêm vài endpoint đọc số liệu của chính nó:
  - study: `GET /v1/me/study-stats`, `GET /internal/progress/summary` (theo danh sách user/study set)
  - class: `GET /v1/classes/:id/dashboard`, mở rộng `/v1/activity`
  - payment: `GET /v1/admin/payments/stats`
  - auth: `GET /v1/admin/users/stats`
- Web ghép nhiều lời gọi song song cho dashboard cá nhân.
- **Admin cần số liệu chéo service** → **[Đề xuất]** thêm bảng **`metrics_daily`** (mỗi service tự ghi số tổng hợp theo ngày qua sự kiện NATS/outbox đã có) + một endpoint admin đọc. Nhanh hơn nhiều so với `COUNT(*)` trên bảng lớn mỗi lần mở trang.
- Sự kiện cần bổ sung vào luồng hiện có: `study.session_completed`, `quiz.completed` (kèm `studySetId`, `score`, `classId`), `user.registered`, `order.paid`.
- Nếu sau này dữ liệu lớn/cần truy vấn phức tạp mới tách `analytics` service.

### 4.5 Công nghệ giao diện
- **[Đề xuất]** thêm `react-router` (v6) **trước** khi làm dashboard; chuyển dần từng màn, không đập đi làm lại `main.tsx` một lần.
- **[Đề xuất]** `recharts` cho biểu đồ (đủ dùng, ít cấu hình); component dùng chung: `KpiCard`, `TimeSeriesChart`, `DataTable` (phân trang/sắp xếp phía server), `DateRangePicker`, `EmptyState`.
- `useCan()` + `<Can action="…">` để ẩn/hiện theo quyền.

---

## 5. Lộ trình thực hiện

Ước lượng theo **1 người làm**, đơn vị ngày làm việc (S≈0.5–1, M≈2–3, L≈4–6). Mỗi phase **chạy được độc lập** và có tiêu chí nghiệm thu.

### Phase 0 — Chốt thiết kế (1–2 ngày)
- [ ] **P0-01** Bạn trả lời các câu hỏi ở mục 9 (đặc biệt #1–#4).
- [ ] **P0-02** Viết ADR ngắn `docs/architecture/adr-004-authz-model.md` (mô hình ở mục 3).
- [ ] **P0-03** Chốt danh mục action cuối cùng (bảng 3.5) và ma trận role × action (mục 7).

### Phase 1 — Nền tảng phân quyền (L, ~5–6 ngày)
- [ ] **IAM-01** Tạo module `packages/authz` + thêm vào `go.work`: catalog action, role dựng sẵn, `Can()`, khớp resource dạng ARN có wildcard.
- [ ] **IAM-02** Test bảng (table-driven) cho `Can()`: default-deny, Deny thắng Allow, kế thừa scope class→org→platform, điều kiện `resourceOwner`.
- [ ] **IAM-03** Auth service: migration `iam_roles`, `iam_assignments`, `audit_log`; seed role dựng sẵn.
- [ ] **IAM-04** **Backfill an toàn:** `users.role='admin'` → `PlatformAdmin`; user còn lại → `Student` (mặc định) — kèm script chạy thử (dry-run) và báo cáo số lượng.
- [ ] **IAM-05** Đăng ký: thêm lựa chọn Giáo viên/Học sinh (API + `AuthScreen.tsx`) → tạo assignment tương ứng.
- [ ] **IAM-06** Mở rộng `VerifiedIdentity` + gateway: thêm `X-User-Roles` có ký HMAC, vẫn xoá header giả mạo; giữ `X-User-Role` tương thích.
- [ ] **IAM-07** `GET /v1/me/permissions` (danh sách role + action hiệu lực theo scope).
- [ ] **IAM-08** Thay kiểm tra viết cứng bằng `authz.Can()` — **theo thứ tự rủi ro**: payment (`RequireAdmin`) → class service (owner/teacher checks) → study → quiz/live.
- [ ] **IAM-09** Ghi `audit_log` cho: gán/thu hồi role, vô hiệu hoá user, các thao tác admin tiền.
- [ ] **IAM-10** Test e2e: học sinh không gọi được API giáo viên/admin; giáo viên không tự gán admin; header giả mạo bị bỏ qua.

**Nghiệm thu Phase 1:** hành vi hiện tại **không đổi** với người dùng cũ (không ai mất quyền oan); mọi nơi từng kiểm tra `role == "admin"` đều đi qua `authz.Can()`; có test chứng minh học sinh bị chặn.

### Phase 2 — Khung giao diện + Dashboard cá nhân (L, ~5–6 ngày)
- [ ] **UI-01** Thêm `react-router`; chuyển shell (`AppShell`) + 2–3 màn đầu sang route; giữ tương thích đường dẫn cũ (`VIEW_PATHS`).
- [ ] **UI-02** `AuthContext`: nạp `me/permissions`; thêm `useCan`, `<Can>`.
- [ ] **UI-03** Bộ component dashboard dùng chung (`KpiCard`, `TimeSeriesChart`, `DataTable`, `EmptyState`) + chọn `recharts`.
- [ ] **UI-04** study service: `GET /v1/me/study-stats` (chuỗi ngày học, thẻ cần ôn, phiên gần đây).
- [ ] **UI-05** Dashboard **học sinh** (4.1) — dữ liệu thật từ study/class/payment.
- [ ] **UI-06** Dashboard **giáo viên** (4.1); đổi tên màn "Dashboard" hiện tại thành **"Thư viện học phần"** để khỏi nhầm.
- [ ] **UI-07** Trạng thái rỗng/lỗi/đang tải cho mọi widget; responsive điện thoại.

**Nghiệm thu:** học sinh và giáo viên mở `/dashboard` thấy 2 bố cục khác nhau; tải < 1.5 giây với dữ liệu thử; widget lỗi không làm hỏng cả trang.

### Phase 3 — Dashboard lớp (M–L, ~4–5 ngày)
- [ ] **CLS-01** Bổ sung sự kiện `quiz.completed`/`study.session_completed` vào outbox→NATS→`activity_events`.
- [ ] **CLS-02** study: `GET /internal/progress/summary` (theo nhóm user × study set; chỉ nội bộ, qua chữ ký service).
- [ ] **CLS-03** class: `GET /v1/classes/:id/dashboard` (kiểm tra `class:ViewDashboard`; trả dữ liệu **khác nhau** cho giáo viên và học sinh).
- [ ] **CLS-04** Web: trang `/classes/:id/dashboard` + tab trong `ClassDetail.tsx`; bảng học sinh, tiến độ từng bài, xuất CSV.
- [ ] **CLS-05** Công tắc "hiện bảng xếp hạng cho học sinh" (cột mới trên `classes`).
- [ ] **CLS-06** Test: học sinh A **không** đọc được tiến độ học sinh B (API trả 403, không chỉ ẩn UI).

### Phase 4 — Admin console (L, ~6–8 ngày)
- [ ] **ADM-01** `AdminShell` + route `/admin/*` (gateway chặn thô bằng `dashboard:ViewAdmin`).
- [ ] **ADM-02** Bảng `metrics_daily` + job tổng hợp (cron trong service hoặc consumer NATS); backfill 90 ngày từ dữ liệu có sẵn.
- [ ] **ADM-03** `GET /v1/admin/overview?range=28d` (KPI + so kỳ trước + chuỗi thời gian).
- [ ] **ADM-04** Trang **Tổng quan** (KPI + biểu đồ + nội dung nổi bật + hoạt động mới).
- [ ] **ADM-05** **Người dùng**: danh sách/tìm/lọc, chi tiết, vô hiệu hoá, gán/thu hồi role (có xác nhận + audit).
- [ ] **ADM-06** **Nội dung** + **Lớp học**: danh sách toàn hệ thống, gỡ/ẩn nội dung.
- [ ] **ADM-07** Chuyển `AdminPayments` vào console + thống kê doanh thu.
- [ ] **ADM-08** **Vai trò & quyền** (chỉ xem) + **Nhật ký (Audit)** có lọc/phân trang.
- [ ] **ADM-09** **Hệ thống**: trạng thái dịch vụ, hàng đợi import.
- [ ] **ADM-10** Bảo vệ: giới hạn tốc độ API admin, yêu cầu xác nhận lại mật khẩu (hoặc xác nhận 2 bước) cho thao tác nguy hiểm (gán admin, cộng tiền ví).

### Phase 5 — Tổ chức / trường học (L, làm khi có nhu cầu thật)
- [ ] **ORG-01** Bảng `organizations`; `classes.org_id` (NULL = lớp cá nhân); thành viên tổ chức qua `iam_assignments` scope `org`.
- [ ] **ORG-02** Role `OrgAdmin/OrgTeacher/OrgStudent`; **kế thừa** quyền xuống các lớp thuộc tổ chức (chỉ cần bật nhánh scope `org` trong `Can()` — đã thiết kế sẵn ở Phase 1).
- [ ] **ORG-03** Luồng mời giáo viên/học sinh (email/mã), nhập danh sách bằng Excel (tái dùng cơ chế import).
- [ ] **ORG-04** Dashboard tổ chức (tổng hợp mọi lớp) + mục "Tổ chức" trong admin.
- [ ] **ORG-05** Role tuỳ chỉnh theo tổ chức (bật `kind='custom'` trong `iam_roles`) — chỉ làm nếu khách hàng yêu cầu.

### Phase 6 — Hoàn thiện (liên tục)
- [ ] **HRD-01** Script e2e kiểu `infra/scripts/phaseN-e2e.sh` cho phân quyền + dashboard, thêm vào GitHub Actions (cùng pattern `phase*-contract-gate.yml`).
- [ ] **HRD-02** Cập nhật `packages/api-contracts/openapi.yaml` cho mọi endpoint mới. Repo có sẵn `redocly.yaml` nhưng các workflow trong `.github/workflows` chưa chạy lint → thêm một bước `redocly lint` vào CI.
- [ ] **HRD-03** Kiểm thử tải nhẹ cho endpoint dashboard; thêm chỉ mục DB theo EXPLAIN thực tế.
- [ ] **HRD-04** Tài liệu vận hành: cách gán admin đầu tiên, cách thu hồi, cách đọc audit.

---

## 6. API mới (tóm tắt)

| Method | Đường dẫn | Service | Quyền cần |
|---|---|---|---|
| GET | `/v1/me/permissions` | auth | đăng nhập |
| GET | `/v1/me/study-stats` | study | `dashboard:ViewSelf` |
| GET | `/v1/classes/{id}/dashboard` | class | `class:ViewDashboard` |
| GET | `/v1/admin/overview` | auth (tổng hợp) | `dashboard:ViewAdmin` |
| GET | `/v1/admin/users` · `/v1/admin/users/{id}` | auth | `user:List` |
| POST | `/v1/admin/users/{id}/disable` | auth | `user:Disable` |
| GET/POST/DELETE | `/v1/admin/assignments` | auth | `iam:ListAssignments` / `iam:AssignRole` / `iam:RevokeRole` |
| GET | `/v1/admin/roles` | auth | `iam:ListRoles` |
| GET | `/v1/admin/audit` | auth | `audit:Read` |
| GET | `/v1/admin/payments/stats` | payment | `order:ListAll` |
| GET | `/internal/progress/summary` | study (nội bộ) | chữ ký service |

---

## 7. Ma trận quyền (khởi điểm — để bạn duyệt)

| Hành động | Student | Teacher | ClassStudent | ClassTeacher | ClassOwner | PlatformAdmin |
|---|:--:|:--:|:--:|:--:|:--:|:--:|
| Tạo/sửa/xoá học phần **của mình** | ✓ | ✓ | – | – | – | ✓ |
| Sửa/xoá học phần **người khác** | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| Tạo lớp | ✗ | ✓ | – | – | – | ✓ |
| Tham gia lớp bằng mã mời | ✓ | ✓ | – | – | – | ✓ |
| Xem nội dung được giao trong lớp | – | – | ✓ | ✓ | ✓ | ✓ |
| Giao/bỏ giao học phần cho lớp | – | – | ✗ | ✓ | ✓ | ✓ |
| Xem tiến độ **cả lớp** | – | – | ✗ | ✓ | ✓ | ✓ |
| Xem tiến độ **của mình** | – | – | ✓ | ✓ | ✓ | ✓ |
| Thêm/xoá thành viên, đổi vai trò trong lớp | – | – | ✗ | ✓ (không đổi owner) | ✓ | ✓ |
| Đổi mã mời / xoá lớp | – | – | ✗ | ✗ | ✓ | ✓ |
| Tổ chức live quiz | ✗ | ✓ | ✗ | ✓ | ✓ | ✓ |
| Xem dashboard admin | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| Gán/thu hồi role, vô hiệu hoá user | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |
| Cộng tiền vào ví, xem mọi đơn hàng | ✗ | ✗ | ✗ | ✗ | ✗ | ✓ |

> Học sinh được tạo học phần cá nhân để tự học (giống Quizlet). Nếu bạn muốn học sinh **không** được tạo/bán học phần, đổi ở câu hỏi #3.

---

## 8. Kiểm thử & rủi ro

**Kiểm thử bắt buộc**
- Test bảng cho `authz.Can()` (mọi ô trong ma trận mục 7 = 1 test case).
- Test "không leo thang": học sinh/giáo viên gọi API admin → 403; thử giả mạo `X-User-Roles`/`X-Class-Role` từ client → bị gateway xoá (gateway đã có test tương tự cho `X-Class-Role`, mở rộng cho header mới).
- Test cô lập dữ liệu: học sinh A không đọc dữ liệu của B hoặc của lớp khác (kiểm bằng API, không chỉ UI).
- Test backfill trên bản sao dữ liệu thật trước khi chạy production.

| Rủi ro | Cách giảm |
|---|---|
| Backfill gán sai role → người dùng mất quyền | Dry-run + báo cáo; giữ `users.role` cũ song song 1–2 phiên bản; cờ cấu hình để quay lại kiểm tra cũ |
| Quyền cấp class nằm ở `class_members` còn platform ở auth → lệch dữ liệu | Hai nguồn **không trùng phạm vi** (class vs platform/org); `Can()` là nơi duy nhất ghép lại |
| `COUNT(*)` chậm trên dashboard admin | `metrics_daily` + chỉ mục; cache 60 giây |
| Lộ dữ liệu học sinh trong dashboard lớp | Kiểm quyền ở server + test cô lập; không tin vào ẩn UI |
| Thêm `react-router` làm vỡ link cũ | Chuyển dần, giữ `VIEW_PATHS` làm lớp tương thích đến khi xong |
| Làm quá rộng (IAM đầy đủ như AWS) | Chỉ làm đến Phase 1 trước; Phase 5 chỉ khi có trường học thật |

---

## 9. Câu hỏi mở (mình đã chọn sẵn mặc định — bạn xác nhận hoặc đổi)

1. **Ai được tự chọn "Giáo viên" khi đăng ký?** *Mặc định:* ai cũng chọn được (đơn giản); khi lên trường học thì giáo viên của trường do OrgAdmin mời. Phương án khác: giáo viên cần admin duyệt.
2. **Một người vừa là giáo viên vừa là học sinh?** *Mặc định:* được (role platform + role trong từng lớp độc lập).
3. **Học sinh có được tạo/bán học phần không?** *Mặc định:* được tạo cho bản thân; **bán** (`study-set:SetPrice`) chỉ Teacher.
4. **Admin có xem được nội dung riêng tư của người dùng không?** *Mặc định:* xem được để kiểm duyệt, **mọi lần xem đều ghi audit**.
5. **Bảng xếp hạng trong lớp:** *Mặc định:* tắt, giáo viên bật theo từng lớp.
6. **Biểu đồ:** *Mặc định:* `recharts`.
7. **Có cần xác thực 2 bước cho admin?** *Mặc định:* chưa ở Phase 4 đầu; yêu cầu nhập lại mật khẩu cho thao tác nguy hiểm.
8. **Thứ tự ưu tiên nếu phải cắt:** Phase 1 → Phase 2 → Phase 3 → Phase 4 → Phase 5. (Phase 1 là nền, không nên bỏ.)

---

## 10. Việc đầu tiên nên làm
1. Bạn xem mục 3.4, 7 và 9, nói mình chỗ nào muốn đổi.
2. Mình bắt đầu **Phase 1 (IAM-01 → IAM-03)**: thư viện `authz` + test, chưa đụng hành vi hiện tại.
3. Sau mỗi phase, mình chạy e2e và báo kết quả trước khi sang phase tiếp theo.
