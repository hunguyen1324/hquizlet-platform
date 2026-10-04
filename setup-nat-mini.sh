#!/usr/bin/env bash
# ============================================================================
# setup-nat-mini.sh — Cài đặt server APP (nat-mini-zzp40b) cho HQuizlet Platform
#
# Chạy 1 lần sau khi cài lại OS trên server này (hoặc server mới).
# An toàn chạy lại nhiều lần: bước nào đã xong sẽ tự bỏ qua, KHÔNG ghi đè
# .env nếu đã có.
#
# Cách dùng:
#   scp -P 30256 setup-nat-mini.sh root@vn-hn.cloudcode.io.vn:~/
#   ssh root@vn-hn.cloudcode.io.vn -p 30256
#   chmod +x setup-nat-mini.sh
#   ./setup-nat-mini.sh
#
# LƯU Ý: script sẽ DỪNG LẠI ở bước .env nếu file .env chưa tồn tại, vì các
# giá trị mật khẩu (POSTGRES/REDIS/MINIO) phải copy thủ công từ server Data
# sang — script không tự đoán được. Chạy script 1 lần để nó tạo khung .env
# từ mẫu, điền xong rồi chạy lại script để nó tiếp tục pull & up.
# ============================================================================
set -euo pipefail

REPO_URL="https://github.com/hunguyen1324/hquizlet-platform.git"
INSTALL_DIR="/opt/hquizlet"
GHCR_USER="hunguyen1324"

log()  { echo -e "\n\033[1;32m==> $*\033[0m"; }
warn() { echo -e "\033[1;33m[CẢNH BÁO] $*\033[0m"; }
err()  { echo -e "\033[1;31m[LỖI] $*\033[0m"; }

if [ "$(id -u)" -ne 0 ]; then
  echo "Script này cần chạy bằng root (sudo ./setup-nat-mini.sh)"; exit 1
fi

# ── 1. Đợi apt/dpkg rảnh ─────────────────────────────────────────────────
log "Kiểm tra apt/dpkg có đang bị khoá không..."
waited=0; max_wait=600
while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 \
   || fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
  if [ "$waited" -ge "$max_wait" ]; then
    err "Đợi quá 10 phút mà apt vẫn bị khoá. Kiểm tra: ps aux | grep unattended-upgr"
    exit 1
  fi
  echo "   ...apt đang bận, đợi 10s (đã đợi ${waited}s)"
  sleep 10
  waited=$((waited + 10))
done

# ── 2. Cài Docker ────────────────────────────────────────────────────────
if command -v docker >/dev/null 2>&1; then
  log "Docker đã cài, bỏ qua."
else
  log "Cài Docker..."
  curl -fsSL https://get.docker.com | sh
fi
docker --version
docker compose version

# ── 3. Ưu tiên IPv4 ra internet ──────────────────────────────────────────
if ! grep -q "^precedence ::ffff:0:0/96 100" /etc/gai.conf 2>/dev/null; then
  log "Thêm rule ưu tiên IPv4 vào /etc/gai.conf..."
  echo "precedence ::ffff:0:0/96 100" >> /etc/gai.conf
else
  log "/etc/gai.conf đã có rule ưu tiên IPv4, bỏ qua."
fi

# ── 4. Clone/cập nhật code ───────────────────────────────────────────────
mkdir -p "$INSTALL_DIR"
if [ -d "$INSTALL_DIR/.git" ]; then
  log "Repo đã có, chạy git pull..."
  git -C "$INSTALL_DIR" pull
else
  log "Clone repo về $INSTALL_DIR..."
  git clone "$REPO_URL" "$INSTALL_DIR"
fi
cd "$INSTALL_DIR"

# ── 5. Firewall — chỉ mở 22/80/443, không mở port service nội bộ ────────
log "Cấu hình ufw..."
ufw default deny incoming >/dev/null
ufw allow 22/tcp >/dev/null
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null
ufw --force enable

# ── 6. Kiểm tra .env ─────────────────────────────────────────────────────
ENV_FILE="$INSTALL_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
  if [ -f "$INSTALL_DIR/.env.app.example" ]; then
    cp "$INSTALL_DIR/.env.app.example" "$ENV_FILE"
    warn "Chưa có .env — đã tạo khung từ .env.app.example tại $ENV_FILE"
  else
    err "Chưa có .env và cũng không thấy .env.app.example trong repo."
    err "Tạo thủ công $ENV_FILE rồi chạy lại script này."
    exit 1
  fi
  echo
  warn "CẦN ĐIỀN GIÁ TRỊ THẬT vào $ENV_FILE trước khi chạy tiếp:"
  echo "   - DATA_HOST, POSTGRES_PASSWORD, REDIS_PASSWORD, MINIO_ROOT_USER,"
  echo "     MINIO_ROOT_PASSWORD  → copy đúng giá trị từ .env bên server Data"
  echo "   - JWT_SECRET, ADMIN_TOKEN, CLASS_INTERNAL_TOKEN, WATERMARK_SECRET"
  echo "     → sinh bằng: openssl rand -hex 32"
  echo "   - SEPAY_*  → token/key MỚI (đã rotate trên my.sepay.vn)"
  echo
  echo "Sửa xong, chạy lại: ./setup-nat-mini.sh"
  exit 0
fi

# Kiểm tra còn sót placeholder chưa điền không
if grep -qE "COPY_TU_SERVER_DATA|RANDOM_HEX_32|TOKEN_MOI_SAU_KHI_ROTATE|KEY_MOI_SAU_KHI_ROTATE" "$ENV_FILE"; then
  err ".env vẫn còn placeholder chưa điền:"
  grep -nE "COPY_TU_SERVER_DATA|RANDOM_HEX_32|TOKEN_MOI_SAU_KHI_ROTATE|KEY_MOI_SAU_KHI_ROTATE" "$ENV_FILE" | sed 's/=.*/=<CHƯA ĐIỀN>/'
  err "Điền xong các dòng trên rồi chạy lại script."
  exit 1
fi
log ".env đã đủ, không còn placeholder."

# ── 7. Đăng nhập GHCR (cần để pull image private) ────────────────────────
if docker system info 2>/dev/null | grep -q "ghcr.io" || \
   [ -f "$HOME/.docker/config.json" ] && grep -q "ghcr.io" "$HOME/.docker/config.json" 2>/dev/null; then
  log "Có vẻ đã đăng nhập GHCR trước đó, bỏ qua (nếu pull lỗi 401/403, chạy: docker login ghcr.io -u ${GHCR_USER})"
else
  log "Cần đăng nhập GHCR để pull image (dùng token GitHub có quyền read:packages)..."
  docker login ghcr.io -u "$GHCR_USER"
fi

# ── 8. Chạy app layer ─────────────────────────────────────────────────────
log "Pull & khởi động app layer..."
echo "IMAGE_TAG=latest" > .image_tag.env
docker compose -f infra/docker/docker-compose.app.yml --env-file .env --env-file .image_tag.env pull
docker compose -f infra/docker/docker-compose.app.yml --env-file .env --env-file .image_tag.env up -d

sleep 5
log "Trạng thái container:"
docker compose -f infra/docker/docker-compose.app.yml --env-file .env --env-file .image_tag.env ps

log "Kiểm tra healthz..."
sleep 3
curl -fsS http://localhost/api/healthz && echo || warn "Chưa gọi được /api/healthz — đợi thêm vài chục giây rồi thử lại: curl http://localhost/api/healthz"

log "XONG. Nếu tất cả container Up/healthy và healthz trả về OK, app layer đã sẵn sàng."
