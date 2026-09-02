// AdminPayments — Phase 8: Admin view for payment orders + wallet transactions + manual credit
import React, { useEffect, useState, useCallback } from "react";
import { useAuth } from "../auth/AuthContext";
import { adminApi } from "../../lib/api";
import type { AdminOrderItem, WalletTransactionItem } from "../../types";

type Tab = "orders" | "transactions" | "credit";

function formatVND(amount: number): string {
  return amount.toLocaleString("vi-VN") + "₫";
}

type Props = {
  onBack: () => void;
};

export function AdminPayments({ onBack }: Props) {
  const { token } = useAuth();
  const [tab, setTab] = useState<Tab>("orders");
  const [orders, setOrders] = useState<AdminOrderItem[]>([]);
  const [transactions, setTransactions] = useState<WalletTransactionItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  // Credit form
  const [creditUserId, setCreditUserId] = useState("");
  const [creditAmount, setCreditAmount] = useState("");
  const [creditNote, setCreditNote] = useState("");
  const [creditLoading, setCreditLoading] = useState(false);
  const [creditResult, setCreditResult] = useState("");

  const loadOrders = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const result = await adminApi.listOrders(token);
      setOrders(result.items ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Lỗi tải danh sách đơn.");
    } finally {
      setLoading(false);
    }
  }, [token]);

  const loadTransactions = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const result = await adminApi.listTransactions(token);
      setTransactions(result.items ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Lỗi tải danh sách giao dịch.");
    } finally {
      setLoading(false);
    }
  }, [token]);

  useEffect(() => {
    if (tab === "orders") void loadOrders();
    if (tab === "transactions") void loadTransactions();
  }, [tab, loadOrders, loadTransactions]);

  const handleCredit = async () => {
    const userId = parseInt(creditUserId, 10);
    const amount = parseInt(creditAmount, 10);
    if (!userId || !amount || amount <= 0) {
      setCreditResult("Vui lòng nhập user ID và số tiền hợp lệ.");
      return;
    }
    setCreditLoading(true);
    setCreditResult("");
    try {
      await adminApi.credit(token, userId, amount, creditNote || "Admin credit");
      setCreditResult(`✅ Đã credit ${formatVND(amount)} cho user #${userId}`);
      setCreditUserId("");
      setCreditAmount("");
      setCreditNote("");
    } catch (err) {
      setCreditResult("❌ " + (err instanceof Error ? err.message : "Lỗi credit"));
    } finally {
      setCreditLoading(false);
    }
  };

  return (
    <>
      <section className="page-heading">
        <div>
          <button className="ghost-button back-btn" onClick={onBack}>← Quay lại</button>
          <p className="eyebrow">Admin</p>
          <h1>Quản lý thanh toán</h1>
        </div>
      </section>

      <div className="mode-tabs">
        <button className={`tab ${tab === "orders" ? "active" : ""}`} onClick={() => setTab("orders")}>Đơn nạp tiền</button>
        <button className={`tab ${tab === "transactions" ? "active" : ""}`} onClick={() => setTab("transactions")}>Giao dịch ví</button>
        <button className={`tab ${tab === "credit" ? "active" : ""}`} onClick={() => setTab("credit")}>Credit thủ công</button>
      </div>

      {error && <p className="message message--error">{error}</p>}

      {tab === "orders" && (
        <section className="panel">
          {loading ? (
            <div className="loading-skeleton" aria-busy="true"><div className="skeleton-row" /><div className="skeleton-row" /></div>
          ) : (
            <table className="admin-table">
              <thead>
                <tr>
                  <th>User ID</th>
                  <th>Mã đơn</th>
                  <th>Số tiền</th>
                  <th>Trạng thái</th>
                  <th>Ngày tạo</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.id}>
                    <td>{o.userId}</td>
                    <td>{o.sepayOrderCode}</td>
                    <td>{formatVND(o.amountVnd)}</td>
                    <td>{o.status}</td>
                    <td>{new Date(o.createdAt).toLocaleDateString("vi-VN")}</td>
                  </tr>
                ))}
                {orders.length === 0 && <tr><td colSpan={5}>Không có đơn nào.</td></tr>}
              </tbody>
            </table>
          )}
        </section>
      )}

      {tab === "transactions" && (
        <section className="panel">
          {loading ? (
            <div className="loading-skeleton" aria-busy="true"><div className="skeleton-row" /><div className="skeleton-row" /></div>
          ) : (
            <table className="admin-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Loại</th>
                  <th>Hướng</th>
                  <th>Số tiền</th>
                  <th>Ghi chú</th>
                  <th>Ngày</th>
                </tr>
              </thead>
              <tbody>
                {transactions.map((tx) => (
                  <tr key={tx.id}>
                    <td>{tx.id}</td>
                    <td>{tx.type}</td>
                    <td>{tx.direction}</td>
                    <td className={tx.direction === "credit" ? "credit" : "debit"}>
                      {tx.direction === "credit" ? "+" : "-"}{formatVND(tx.amountVnd)}
                    </td>
                    <td>{tx.note || "-"}</td>
                    <td>{new Date(tx.createdAt).toLocaleDateString("vi-VN")}</td>
                  </tr>
                ))}
                {transactions.length === 0 && <tr><td colSpan={6}>Không có giao dịch nào.</td></tr>}
              </tbody>
            </table>
          )}
        </section>
      )}

      {tab === "credit" && (
        <section className="panel">
          <h2>Credit thủ công</h2>
          <div className="admin-form">
            <div className="form-group">
              <label>User ID:</label>
              <input type="number" value={creditUserId} onChange={(e) => setCreditUserId(e.target.value)} placeholder="User ID" />
            </div>
            <div className="form-group">
              <label>Số tiền (VND):</label>
              <input type="number" value={creditAmount} onChange={(e) => setCreditAmount(e.target.value)} placeholder="100000" />
            </div>
            <div className="form-group">
              <label>Ghi chú:</label>
              <input type="text" value={creditNote} onChange={(e) => setCreditNote(e.target.value)} placeholder="Admin credit" />
            </div>
            <button className="primary-button" onClick={() => void handleCredit()} disabled={creditLoading}>
              {creditLoading ? "Đang xử lý..." : "Credit"}
            </button>
            {creditResult && <p className={creditResult.startsWith("✅") ? "message message--success" : "message message--error"}>{creditResult}</p>}
          </div>
        </section>
      )}
    </>
  );
}
