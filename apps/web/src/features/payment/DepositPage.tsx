// DepositPage — Phase 8: Deposit via SePay QR code with polling
import React, { useState, useEffect, useCallback, useRef } from "react";
import { useAuth } from "../auth/AuthContext";
import { paymentApi, walletApi } from "../../lib/api";
import type { PaymentOrder, DepositOrderStatus } from "../../types";

type Props = {
  onBack: () => void;
  onSuccess: () => void;
};

const PRESET_AMOUNTS = [50000, 100000, 200000, 500000, 1000000];

function formatVND(amount: number): string {
  return amount.toLocaleString("vi-VN") + "₫";
}

export function DepositPage({ onBack, onSuccess }: Props) {
  const { token } = useAuth();
  const [currentBalance, setCurrentBalance] = useState<number | null>(null);
  const [selectedAmount, setSelectedAmount] = useState<number>(100000);
  const [customAmount, setCustomAmount] = useState("");
  const [order, setOrder] = useState<PaymentOrder | null>(null);
  const [orderStatus, setOrderStatus] = useState<DepositOrderStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Load current balance
  useEffect(() => {
    void walletApi.getBalance(token).then((b) => setCurrentBalance(b.balance)).catch(() => {});
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
    const amount = customAmount ? parseInt(customAmount, 10) : selectedAmount;
    if (!amount || amount < 10000 || amount > 50000000) {
      setError("Số tiền phải từ 10,000₫ đến 50,000,000₫");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const result = await paymentApi.createOrder(token, amount);
      setOrder(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không tạo được đơn nạp tiền.");
    } finally {
      setLoading(false);
    }
  };

  if (order) {
    return (
      <>
        <section className="page-heading">
          <div>
            <button className="ghost-button back-btn" onClick={() => { if (pollRef.current) clearInterval(pollRef.current); setOrder(null); setOrderStatus(null); }}>← Hủy</button>
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
                <img src={order.qrCodeUrl} alt="QR Code" className="deposit-qr" />
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
            min={10000}
            max={50000000}
            step={10000}
            placeholder="Nhập số tiền (VND)"
            value={customAmount}
            onChange={(e) => setCustomAmount(e.target.value)}
          />
        </div>

        {error && <p className="message message--error">{error}</p>}

        <button className="primary-button" onClick={() => void handleCreateOrder()} disabled={loading}>
          {loading ? "Đang tạo đơn..." : "Thanh toán"}
        </button>
      </section>
    </>
  );
}
