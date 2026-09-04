// DepositPage — Phase 8: Deposit via SePay QR code with polling
import React, { useState, useEffect, useRef } from "react";
import { useAuth } from "../auth/AuthContext";
import { paymentApi, walletApi } from "../../lib/api";
import type { PaymentOrder, DepositOrderStatus, PendingDepositOrder } from "../../types";

type Props = {
  onBack: () => void;
  onSuccess: () => void;
};

const PRESET_AMOUNTS = [50000, 100000, 200000, 500000, 1000000];
const MIN_DEPOSIT_VND = 10000;
const MAX_DEPOSIT_VND = 50000000;
const PENDING_ORDER_MESSAGE = "Bạn đang có quá nhiều đơn nạp đang chờ. Vui lòng hoàn tất hoặc chờ đơn cũ hết hạn rồi thử lại.";

function formatVND(amount: number): string {
  return amount.toLocaleString("vi-VN") + "₫";
}

function formatDateTime(value?: string): string {
  if (!value) return "";
  return new Date(value).toLocaleString("vi-VN", { hour: "2-digit", minute: "2-digit", day: "2-digit", month: "2-digit" });
}

function normalizeDepositError(err: unknown): string {
  if (!(err instanceof Error)) return "Không tạo được đơn nạp tiền.";
  if (err.message.toLowerCase().includes("too many pending orders")) return PENDING_ORDER_MESSAGE;
  return err.message || "Không tạo được đơn nạp tiền.";
}

export function DepositPage({ onBack, onSuccess }: Props) {
  const { token } = useAuth();
  const [currentBalance, setCurrentBalance] = useState<number | null>(null);
  const [selectedAmount, setSelectedAmount] = useState<number>(PRESET_AMOUNTS[0]);
  const [customAmount, setCustomAmount] = useState("");
  const [order, setOrder] = useState<PaymentOrder | null>(null);
  const [orderStatus, setOrderStatus] = useState<DepositOrderStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [qrLoadFailed, setQrLoadFailed] = useState(false);
  const [pendingOrders, setPendingOrders] = useState<PendingDepositOrder[]>([]);
  const [pendingLoading, setPendingLoading] = useState(false);
  const [cancelingOrderId, setCancelingOrderId] = useState<number | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Load current balance
  useEffect(() => {
    void walletApi.getBalance(token).then((b) => setCurrentBalance(b.balance)).catch(() => {});
  }, [token]);

  const loadPendingOrders = async () => {
    setPendingLoading(true);
    try {
      const result = await paymentApi.listPendingOrders(token);
      setPendingOrders(result.items ?? []);
    } catch {
      setPendingOrders([]);
    } finally {
      setPendingLoading(false);
    }
  };

  useEffect(() => {
    void loadPendingOrders();
  }, [token]);

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  // Poll order status when order exists
  useEffect(() => {
    if (!order) return;
    pollRef.current = setInterval(async () => {
      try {
        const status = await paymentApi.getOrderStatus(token, order.orderId);
        setOrderStatus(status);
        if (status.status === "PAID") {
          if (pollRef.current) clearInterval(pollRef.current);
          setTimeout(onSuccess, 2000);
        }
      } catch {
        // Silently retry
      }
    }, 3000);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [order, token, onSuccess]);

  const handleCreateOrder = async () => {
    const customValue = customAmount.trim();
    const amount = customValue ? Number(customValue) : selectedAmount;
    if (!Number.isInteger(amount) || amount < MIN_DEPOSIT_VND || amount > MAX_DEPOSIT_VND) {
      setError("Số tiền phải từ 10,000₫ đến 50,000,000₫");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const result = await paymentApi.createOrder(token, amount);
      setOrder(result);
      setQrLoadFailed(false);
    } catch (err) {
      setError(normalizeDepositError(err));
      if (err instanceof Error && err.message.toLowerCase().includes("too many pending orders")) {
        await loadPendingOrders();
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCancelPendingOrder = async (orderId: number) => {
    setCancelingOrderId(orderId);
    setError("");
    try {
      await paymentApi.cancelOrder(token, orderId);
      setPendingOrders((items) => items.filter((item) => item.orderId !== orderId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không hủy được đơn nạp.");
      await loadPendingOrders();
    } finally {
      setCancelingOrderId(null);
    }
  };

  if (order) {
    return (
      <>
        <section className="page-heading">
          <div>
            <button className="ghost-button back-btn" onClick={() => { if (pollRef.current) clearInterval(pollRef.current); setOrder(null); setOrderStatus(null); setQrLoadFailed(false); }}>← Hủy</button>
            <p className="eyebrow">Nạp tiền</p>
            <h1>Chuyển khoản theo mã QR</h1>
          </div>
        </section>

        <section className="panel deposit-waiting">
          {orderStatus?.status === "PAID" ? (
            <div className="deposit-success">
              <h2>✅ Thanh toán thành công!</h2>
              <p>Số tiền {formatVND(order.amountVnd)} đã được cộng vào ví.</p>
              <button className="primary-button" onClick={onSuccess}>Xem ví</button>
            </div>
          ) : (
            <>
              <div className="qr-section">
                {qrLoadFailed ? (
                  <div className="qr-fallback">
                    <p>Không tải được ảnh QR.</p>
                    <a href={order.qrCodeUrl} target="_blank" rel="noreferrer">Mở QR trong tab mới</a>
                  </div>
                ) : (
                  <img
                    src={order.qrCodeUrl}
                    alt="QR Code"
                    className="deposit-qr"
                    onError={() => setQrLoadFailed(true)}
                  />
                )}
              </div>

              <div className="order-info">
                <div className="info-row">
                  <span>Số tài khoản:</span>
                  <strong>{order.bankAccountNumber}</strong>
                </div>
                <div className="info-row">
                  <span>Chủ tài khoản:</span>
                  <strong>{order.bankAccountHolder}</strong>
                </div>
                <div className="info-row">
                  <span>Ngân hàng:</span>
                  <strong>{order.bankName}</strong>
                </div>
                <div className="info-row">
                  <span>Số tiền:</span>
                  <strong>{formatVND(order.amountVnd)}</strong>
                </div>
                <div className="info-row highlight">
                  <span>Nội dung CK:</span>
                  <strong className="order-code">{order.orderCode}</strong>
                </div>
              </div>

              <div className="poll-status">
                <div className="spinner" />
                <span>Đang chờ thanh toán... (tự cập nhật mỗi 3 giây)</span>
              </div>
            </>
          )}
        </section>
      </>
    );
  }

  return (
    <>
      <section className="page-heading">
        <div>
          <button className="ghost-button back-btn" onClick={onBack}>← Quay lại</button>
          <p className="eyebrow">Nạp tiền</p>
          <h1>Chọn số tiền nạp</h1>
          {currentBalance !== null && <p>Số dư hiện tại: <strong>{formatVND(currentBalance)}</strong></p>}
        </div>
      </section>

      <section className="panel deposit-amounts">
        <div className="amount-grid">
          {PRESET_AMOUNTS.map((amt) => (
            <button
              key={amt}
              className={`amount-btn ${selectedAmount === amt && !customAmount ? "selected" : ""}`}
              onClick={() => { setSelectedAmount(amt); setCustomAmount(""); }}
            >
              {formatVND(amt)}
            </button>
          ))}
        </div>

        <div className="custom-amount">
          <label htmlFor="custom-amount">Số tiền khác:</label>
          <input
            id="custom-amount"
            type="number"
            min={MIN_DEPOSIT_VND}
            max={MAX_DEPOSIT_VND}
            step={10000}
            placeholder="Nhập số tiền (VND)"
            value={customAmount}
            onChange={(e) => {
              setCustomAmount(e.target.value);
              setError("");
            }}
          />
        </div>

        {error && <p className="message message--error" role="alert">{error}</p>}

        {(pendingLoading || pendingOrders.length > 0) && (
          <div className="pending-orders">
            <div className="pending-orders__header">
              <strong>Đơn nạp đang chờ</strong>
              <button className="ghost-button" type="button" onClick={() => void loadPendingOrders()} disabled={pendingLoading}>
                {pendingLoading ? "Đang tải..." : "Làm mới"}
              </button>
            </div>
            {pendingLoading && pendingOrders.length === 0 ? (
              <p>Đang tải đơn đang chờ...</p>
            ) : (
              pendingOrders.map((pendingOrder) => (
                <div className="pending-order" key={pendingOrder.orderId}>
                  <div>
                    <strong>{formatVND(pendingOrder.amountVnd)}</strong>
                    <span>{pendingOrder.orderCode} · Hết hạn {formatDateTime(pendingOrder.expiredAt)}</span>
                  </div>
                  <button
                    className="danger-button"
                    type="button"
                    onClick={() => void handleCancelPendingOrder(pendingOrder.orderId)}
                    disabled={cancelingOrderId === pendingOrder.orderId}
                  >
                    {cancelingOrderId === pendingOrder.orderId ? "Đang hủy..." : "Hủy đơn"}
                  </button>
                </div>
              ))
            )}
          </div>
        )}

        <button className="primary-button" onClick={() => void handleCreateOrder()} disabled={loading}>
          {loading ? "Đang tạo đơn..." : "Thanh toán"}
        </button>
      </section>
    </>
  );
}
