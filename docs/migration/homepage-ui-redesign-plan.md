# Kế Hoạch Làm Lại Giao Diện Trang Chủ (Homepage UI)

**PM:** Claude  
**Mục tiêu:** Làm lại Dashboard thành trang chủ đầy đủ tính năng, giao diện chuẩn Quizlet — có Navbar, Sidebar, và các section: Học tiếp, Gần đây, Câu hỏi chế độ Học, Gợi ý, Chơi và ôn tập.  
**Scope:** Chỉ `apps/web` (React + Vite). Không đụng backend.  
**Ngôn ngữ UI:** Tiếng Việt (giữ nguyên như thiết kế mẫu).

---

## Phân Tích Giao Diện Mục Tiêu

Từ HTML mẫu (clone Quizlet), trang chủ gồm 5 vùng:

| Vùng | Component | Dữ liệu cần |
|---|---|---|
| Navbar (sticky top) | `Navbar` | user session, search query |
| Sidebar (sticky left) | `Sidebar` | user id, folder list |
| Section "Học tiếp" | `ContinueLearningCarousel` | study sets có progress |
| Section "Gần đây" | `RecentList` | study sets gần nhất |
| Section "Câu hỏi chế độ Học" | `LearnModePreview` | câu hỏi mới nhất chưa trả lời |
| Section "Gợi ý phiên kế tiếp" | `SuggestedCarousel` | study sets chưa học |
| Section "Chơi và ôn tập" | `PlaySection` | link tới flashcard/learn/match |

---

## Kiến Trúc Component

```
src/
  components/
    layout/
      Navbar.tsx           ← NEW (sticky, search, user menu, "+ Tạo")
      Sidebar.tsx          ← NEW (nav links, thư mục, bắt đầu tại đây)
      AppShell.tsx         ← NEW (wrapper: Navbar + Sidebar + main)
    home/
      ContinueLearningCarousel.tsx   ← NEW
      RecentList.tsx                 ← NEW
      LearnModePreview.tsx           ← NEW
      SuggestedCarousel.tsx          ← NEW
      PlaySection.tsx                ← NEW
      HomePage.tsx                   ← NEW (assembles all sections)
    ui/
      index.tsx            ← EXTEND (thêm Card, Badge, ProgressBar, Carousel)
  features/
    dashboard/
      Dashboard.tsx        ← REPLACE bằng redirect sang HomePage
```

---

## Chi Tiết Từng Task

### TASK 1 — Cài Tailwind CSS
**File:** `apps/web/package.json`, `apps/web/vite.config.ts`, `apps/web/tailwind.config.js`  
**Lý do:** Thiết kế mẫu dùng Tailwind utility classes (`rounded-xl`, `bg-card`, v.v.). Repo hiện tại dùng CSS thuần, nhưng migrate sang Tailwind sẽ nhanh hơn rất nhiều so với viết lại CSS thuần tương đương.  
**Việc cần làm:**
1. `npm install -D tailwindcss autoprefixer postcss`
2. `npx tailwindcss init -p`
3. Thêm `@tailwind base/components/utilities` vào `styles.css`
4. Config `content` glob để scan `src/**/*.tsx`
5. Giữ nguyên CSS classes cũ (không xóa) để các feature khác không bị vỡ

**Thời gian ước tính:** 1 giờ  
**Risk:** Thấp — Tailwind không conflict với CSS cũ nếu cấu hình đúng

---

### TASK 2 — Design Tokens & CSS Variables
**File:** `apps/web/src/styles.css`  
**Việc cần làm:**  
Thêm CSS variables chuẩn (giống shadcn/ui pattern) để Navbar/Sidebar dùng đúng màu dark/light mode:

```css
:root {
  --background: #ffffff;
  --foreground: #0f172a;
  --card: #ffffff;
  --card-foreground: #0f172a;
  --muted: #f1f5f9;
  --muted-foreground: #64748b;
  --border: #e2e8f0;
  --primary: #2563eb;
  --primary-foreground: #ffffff;
  --accent: #f1f5f9;
  --accent-foreground: #0f172a;
}
.dark {
  --background: #0f172a;
  --foreground: #f8fafc;
  --card: #1e293b;
  --muted: #1e293b;
  --border: #334155;
  --accent: #1e293b;
}
```

**Thời gian:** 30 phút

---

### TASK 3 — Component: `Navbar`
**File:** `src/components/layout/Navbar.tsx`

**UI elements:**
- Logo "HQuizlet" (link về `/`)
- Search bar (placeholder "Tìm kiếm")
- Button "+ Tạo" (amber/yellow, dropdown: Thẻ ghi nhớ / Nhóm học)
- Bell icon (notifications — tạm thời chỉ icon, chưa functional)
- User avatar (click → dropdown: Profile, Settings, Dark mode, Sign out)

**Props:**
```typescript
type NavbarProps = {
  user: { name: string; image?: string; id: string };
  onSearch?: (q: string) => void;
  onCreateSet: () => void;
  onLogout: () => void;
};
```

**Thời gian:** 2 giờ

---

### TASK 4 — Component: `Sidebar`
**File:** `src/components/layout/Sidebar.tsx`

**UI elements:**
- Nav links: Trang chủ, Thư viện của bạn, Nhóm học, Thông báo
- Section "Thư mục của bạn": button "+ Thư mục mới" (open dialog)
- Section "Bắt đầu tại đây": link Thẻ ghi nhớ (→ `/create-set`)

**Props:**
```typescript
type SidebarProps = {
  userId: string;
  currentView: string;
  onNavigate: (view: AppView) => void;
  onCreateFolder: () => void;
};
```

**Thời gian:** 1.5 giờ

---

### TASK 5 — Component: `AppShell`
**File:** `src/components/layout/AppShell.tsx`

Layout wrapper — ghép Navbar + Sidebar + main content:

```
┌─────────────────────────────────────────────┐
│  NAVBAR (sticky, h-16)                      │
├───────────┬─────────────────────────────────┤
│  SIDEBAR  │  MAIN CONTENT                   │
│  (w-60)   │  (flex-1, px-4 py-8)            │
│  (sticky) │                                 │
└───────────┴─────────────────────────────────┘
```

**Thời gian:** 30 phút

---

### TASK 6 — Shared UI: `Card`, `Badge`, `ProgressBar`, `Carousel`
**File:** `src/components/ui/index.tsx` (extend)

Các primitives cần thêm:

```typescript
// Card — bo góc xl, border, shadow
export function Card({ children, className }: ...)

// Badge — pill màu sky cho "0%", màu blue cho "Mới"
export function Badge({ children, variant }: ...)

// ProgressBar — track muted, fill sky-400
export function ProgressBar({ value }: { value: number })

// Carousel — scroll ngang, snap, chevron prev/next
export function Carousel({ children }: ...)
```

**Thời gian:** 2 giờ

---

### TASK 7 — Section: `ContinueLearningCarousel`
**File:** `src/components/home/ContinueLearningCarousel.tsx`

Hiển thị study sets đang học (đã có progress > 0 hoặc gần nhất):

**Card mỗi set:**
- Title (tối đa 2 dòng)
- "X thẻ • username"
- Badge "0%" (sky)
- ProgressBar
- Link "Xem chi tiết" + Button "Tiếp tục →"

**Data source:** `studySetApi.list()` → sort by `updated_at` DESC, lấy top 5

**Thời gian:** 2 giờ

---

### TASK 8 — Section: `RecentList`
**File:** `src/components/home/RecentList.tsx`

Danh sách gần đây — dạng list (không carousel):
- Icon book
- Title
- "X thẻ • username • Category"
- Divider giữa các items

**Data source:** Cùng API call với carousel, chỉ render khác layout

**Thời gian:** 1 giờ

---

### TASK 9 — Section: `LearnModePreview`
**File:** `src/components/home/LearnModePreview.tsx`

Card preview câu hỏi trắc nghiệm — màu sky-50 border sky-200:
- Tên study set
- Câu hỏi (term)
- 4 lựa chọn (multiple choice, disabled — chỉ hiện đáp án đúng)
- Button "Tiếp tục học →"

**Data source:** Dùng study set gần nhất, lấy flashcard đầu tiên làm câu hỏi, random 3 flashcard khác làm sai.

**Thời gian:** 2 giờ

---

### TASK 10 — Section: `SuggestedCarousel`
**File:** `src/components/home/SuggestedCarousel.tsx`

Carousel card nhỏ hơn (w-64):
- Title (2 dòng)
- "X thẻ • username"
- Badge "Mới" (sky, trending-up icon) hoặc không có badge

**Data source:** `studySetApi.list()` → lấy tất cả, sort by `created_at` DESC, bỏ qua những set đã có trong ContinueLearning

**Thời gian:** 1.5 giờ

---

### TASK 11 — Section: `PlaySection`
**File:** `src/components/home/PlaySection.tsx`

3 card cố định: Flashcards / Học / Ghép thẻ — mỗi card có:
- Icon (màu gradient: sky→blue / violet→purple / emerald→teal)
- Title + mô tả ngắn
- Button "Ôn tập / Học ngay / Chơi" → navigate tới mode tương ứng của study set gần nhất

**Thời gian:** 1 giờ

---

### TASK 12 — `HomePage` Assembler
**File:** `src/features/dashboard/HomePage.tsx` (hoặc `src/components/home/HomePage.tsx`)

Lắp ghép tất cả sections theo thứ tự:
1. `ContinueLearningCarousel` — "Học tiếp"
2. `RecentList` — "Gần đây"
3. `LearnModePreview` — "Câu hỏi chế độ Học"
4. `SuggestedCarousel` — "Gợi ý cho phiên học kế tiếp"
5. `PlaySection` — "Chơi và ôn tập"

Một API call duy nhất fetch toàn bộ study sets, truyền data xuống từng section — tránh N requests.

**Thời gian:** 1 giờ

---

### TASK 13 — Tích Hợp vào `main.tsx`
**File:** `src/main.tsx`

Thay `Dashboard` bằng `HomePage` bọc trong `AppShell`. Điều kiện:
- Nếu user chưa đăng nhập → `AuthScreen` (giữ nguyên)
- Nếu đã đăng nhập → `AppShell` → `HomePage` (hoặc các view khác)

Cũng update routing logic: các view như `editor`, `study`, `folders`, v.v. vẫn render trong `AppShell` (có Navbar + Sidebar).

**Thời gian:** 1 giờ

---

### TASK 14 — Responsive & Polish
**File:** CSS / Tailwind classes trong các component

- Sidebar ẩn trên mobile (`hidden md:flex`)
- Navbar hamburger menu trên mobile
- Carousel scroll trên mobile bằng touch
- Dark mode via `prefers-color-scheme` + CSS variables từ Task 2

**Thời gian:** 1.5 giờ

---

## Thứ Tự Thực Hiện (Dependency Order)

```
TASK 1 (Tailwind)
  ↓
TASK 2 (Design tokens)
  ↓
TASK 6 (UI primitives: Card, Badge, Carousel...)
  ↓
TASK 3 (Navbar)  ──┐
TASK 4 (Sidebar) ──┼→ TASK 5 (AppShell)
                   ↓
TASK 7 (ContinueLearning) ──┐
TASK 8 (RecentList)         │
TASK 9 (LearnModePreview)   ├→ TASK 12 (HomePage)
TASK 10 (SuggestedCarousel) │           ↓
TASK 11 (PlaySection) ──────┘       TASK 13 (main.tsx)
                                        ↓
                                    TASK 14 (Polish)
```

---

## Tổng Thời Gian Ước Tính

| Nhóm | Tasks | Giờ |
|---|---|---|
| Setup | 1, 2 | 1.5h |
| Layout shell | 3, 4, 5 | 4h |
| UI primitives | 6 | 2h |
| Home sections | 7, 8, 9, 10, 11 | 7.5h |
| Assembly & integration | 12, 13 | 2h |
| Polish | 14 | 1.5h |
| **Tổng** | | **~18.5h** |

Nếu làm song song (2 dev): ~10h thực tế.

---

## Quy Tắc Không Được Phá Vỡ

1. **Không đụng backend** — tất cả thay đổi chỉ trong `apps/web/src`
2. **Không xóa CSS cũ** — thêm Tailwind bên cạnh, giữ `.set-card`, `.primary-button`, v.v. để các feature hiện tại không vỡ
3. **Không thay đổi API contract** — `studySetApi`, `flashcardApi`, v.v. giữ nguyên interface
4. **Không thêm router library** — tiếp tục dùng pattern `view state` của `main.tsx`
5. **Giữ AuthProvider** — `AppShell` phải nằm trong `AuthProvider`

---

## Tieu Chí Hoàn Thành (Definition of Done)

- [ ] `http://localhost:5173` sau khi login hiển thị đúng 5 sections như thiết kế mẫu
- [ ] Navbar sticky, search input hoạt động (filter list)
- [ ] Sidebar hiển thị đúng nav links, ẩn trên mobile
- [ ] Carousel scroll ngang, có nút prev/next
- [ ] Dark mode chuyển đúng màu
- [ ] Các feature cũ (StudyDetail, Editor, Folders, Live, Classes...) vẫn mở được từ sidebar/navbar
- [ ] Không có TypeScript error
- [ ] Không có console error khi chạy dev
