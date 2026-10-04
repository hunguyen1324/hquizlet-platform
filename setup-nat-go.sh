#!/usr/bin/env bash
# ============================================================================
# setup-nat-go.sh — Cài đặt server DATA (nat-go-zzp40b) cho HQuizlet Platform
#
# Chạy 1 lần duy nhất sau khi cài lại OS trên server này (hoặc server mới).
# Script an toàn chạy lại nhiều lần (idempotent): nếu bước nào đã làm rồi sẽ
# tự bỏ qua, KHÔNG ghi đè .env đã có sẵn (tránh làm mất mật khẩu đang dùng).
#
# Cách dùng:
#   scp setup-nat-go.sh root@vn-hn.cloudcode.io.vn:~/ -P 30149
#   ssh root@vn-hn.cloudcode.io.vn -p 30149
#   chmod +x setup-nat-go.sh
#   ./setup-nat-go.sh
# ============================================================================
set -euo pipefail

# ── Cấu hình — sửa ở đây nếu IP/repo thay đổi ──────────────────────────────
REPO_URL="https://github.com/hunguyen1324/hquizlet-platform.git"
INSTALL_DIR="/opt/hquizlet"
DATA_IPV6="2001:470:24:849::2d2"   # IPv6 của chính server Data (server này)
APP_IPV6="2001:470:24:849::29f"    # IPv6 của server App — chỉ IP này được phép gọi vào port DB

log()  { echo -e "\n\033[1;32m==> $*\033[0m"; }
warn() { echo -e "\033[1;33m[CẢNH BÁO] $*\033[0m"; }

# ── 0. Phải chạy bằng root ──────────────────────────────────────────────────
if [ "$(id -u)" -ne 0 ]; then
  echo "Script này cần chạy bằng root (sudo ./setup-nat-go.sh)"; exit 1
fi

# ── 1. Đợi apt/dpkg rảnh (unattended-upgrades hay tự chạy sau khi cài OS) ──
wait_for_apt_lock() {
  log "Kiểm tra apt/dpkg có đang bị khoá bởi unattended-upgrades không..."
  local waited=0 max_wait=600   # tối đa 10 phút
  while fuser /var/lib/dpkg/lock-frontend >/dev/null 2>&1 \
     || fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do
    if [ "$waited" -ge "$max_wait" ]; then
      warn "Đợi quá 10 phút mà apt vẫn bị khoá. Kiểm tra thủ công: ps aux | grep unattended-upgr"
      exit 1
    fi
    echo "   ...apt đang bị tiến trình khác dùng (thường là unattended-upgrades), đợi 10s (đã đợi ${waited}s)"
    sleep 10
    waited=$((waited + 10))
  done
}
wait_for_apt_lock

# ── 2. Cài Docker (bỏ qua nếu đã có) ────────────────────────────────────────
if command -v docker >/dev/null 2>&1; then
  log "Docker đã được cài (${BASH_REMATCH[0]:-$(docker --version)}), bỏ qua bước cài."
else
  log "Cài Docker..."
  curl -fsSL https://get.docker.com | sh
fi
docker --version
docker compose version

# ── 3. Ưu tiên IPv4 khi ra internet (tránh treo do IPv6 không có default route) ──
if ! grep -q "^precedence ::ffff:0:0/96 100" /etc/gai.conf 2>/dev/null; then
  log "Thêm rule ưu tiên IPv4 vào /etc/gai.conf..."
  echo "precedence ::ffff:0:0/96 100" >> /etc/gai.conf
else
  log "/etc/gai.conf đã có rule ưu tiên IPv4, bỏ qua."
fi

# ── 4. Clone hoặc cập nhật code ─────────────────────────────────────────────
mkdir -p "$INSTALL_DIR"
if [ -d "$INSTALL_DIR/.git" ]; then
  log "Repo đã tồn tại, chạy git pull để cập nhật..."
  git -C "$INSTALL_DIR" pull
else
  log "Clone repo về $INSTALL_DIR..."
  git clone "$REPO_URL" "$INSTALL_DIR"
fi
cd "$INSTALL_DIR"

# ── 5. Tạo .env nếu CHƯA có (không bao giờ ghi đè .env đã tồn tại) ─────────
ENV_FILE="$INSTALL_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  log ".env đã tồn tại ở $ENV_FILE — giữ nguyên, không ghi đè."
  warn "Nếu muốn tạo lại mật khẩu mới, tự xoá file .env rồi chạy lại script này."
else
  log "Chưa có .env, tạo mới với mật khẩu ngẫu nhiên..."
  PG_PASS=$(openssl rand -hex 16)
  REDIS_PASS=$(openssl rand -hex 16)
  MINIO_PASS=$(openssl rand -hex 16)

  cat > "$ENV_FILE" <<EOF
POSTGRES_DB=hquizlet
POSTGRES_USER=hquizlet
POSTGRES_PASSWORD=${PG_PASS}
REDIS_PASSWORD=${REDIS_PASS}
MINIO_ROOT_USER=hquizlet_admin
MINIO_ROOT_PASSWORD=${MINIO_PASS}
STORAGE_BUCKET=hquizlet
STUDY_IMPORT_BUCKET=hquizlet-imports
DATA_BIND_IP=[${DATA_IPV6}]
EOF

  echo
  echo "============================================================"
  echo " ĐÃ TẠO MẬT KHẨU MỚI — COPY 3 DÒNG NÀY SANG .env CỦA SERVER APP:"
  echo "   POSTGRES_PASSWORD=${PG_PASS}"
  echo "   REDIS_PASSWORD=${REDIS_PASS}"
  echo "   MINIO_ROOT_USER=hquizlet_admin"
  echo "   MINIO_ROOT_PASSWORD=${MINIO_PASS}"
  echo "============================================================"
  echo
fi

# ── 6. Firewall — chỉ cho server App gọi vào port DB/cache/broker ──────────
log "Cấu hình ufw..."
ufw default deny incoming >/dev/null
ufw allow 22/tcp >/dev/null
for port in 5432 6379 4222 8222 9000 9001; do
  ufw allow from "$APP_IPV6" to any port "$port" proto tcp >/dev/null
done
ufw --force enable

# ── 7. Chạy data layer ───────────────────────────────────────────────────
log "Khởi động data layer (postgres, redis, nats, minio)..."
docker compose -f infra/docker/docker-compose.data.yml --env-file .env up -d

sleep 5
log "Trạng thái container:"
docker compose -f infra/docker/docker-compose.data.yml --env-file .env ps

log "Log minio-init (kiểm tra bucket đã tạo chưa):"
docker compose -f infra/docker/docker-compose.data.yml --env-file .env logs minio-init

log "XONG. Nếu tất cả container đều Up/healthy và thấy dòng 'Bucket created successfully' ở trên là server Data đã sẵn sàng."
