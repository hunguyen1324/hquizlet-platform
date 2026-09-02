// WalletPage — Phase 8: Wallet balance + transaction history
import React, { useEffect, useState, useCallback } from "react";
import { useAuth } from "../auth/AuthContext";
import { walletApi } from "../../lib/api";
import type { WalletTransactionItem } from "../../types";

type Props = {
  onDeposit: () => void;
  onBack: () => void;
};

function formatVND(amount: number): string {
  return amount.toLocaleString("vi-VN") + "₫";
}

export function WalletPage({ onDeposit, onBack }: Props) {
  const { token } = useAuth();
  const [balance, setBalance] = useState<number | null>(null);
  const [transactions, setTransactions] = useState<WalletTransactionItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const loadData = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [bal, txs] = await Promise.all([
        walletApi.getBalance(token),
        walletApi.getTransactions(token, limit, offset),
      ]);
      setBalance(bal.balance);
      setTransactions(offset === 0 ? txs.items : (prev) => [...prev, ...txs.items] as any);
      setTotal(txs.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Không tải được ví.");
    } finally {
      setLoading(false);
    }
  }, [token, offset]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  return (
    <>
      <section className="page-heading">
        <div>
          <button className="ghost-button back-btn" onClick={onBack}>← Quay lại</button>
          <p className="eyebrow">Ví của tôi</p>
          <h1>Số dư: {balance !== null ? formatVND(balance) : "..."}</h1>
        </div>
        <button className="primary-button" onClick={onDeposit}>Nạp tiền</button>
      </section>

      {error && <p className="message message--error">{error} <button className="ghost-button" onClick={() => void loadData()}>Thử lại</button></p>}

      <section className="panel">
        <h2>Lịch sử giao dịch</h2>
        {loading && <div className="loading-skeleton" aria-busy="true"><div className="skeleton-row" /><div className="skeleton-row" /><div className="skeleton-row" /></div>}

        {!loading && transactions.length === 0 && (
          <div className="empty-panel">
            <h2>Chưa có giao dịch nào</h2>
            <p>Nạp tiền để bắt đầu mua study set.</p>
          </div>
        )}

        {!loading && transactions.length > 0 && (
          <div className="transaction-list">
            {transactions.map((tx) => (
              <div className="transaction-item" key={tx.id}>
                <div className="tx-info">
                  <span className="tx-label">{tx.label}</span>
                  <span className="tx-date">{new Date(tx.createdAt).toLocaleDateString("vi-VN")}</span>
                </div>
                <span className={`tx-amount ${tx.direction === "credit" ? "credit" : "debit"}`}>
                  {tx.direction === "credit" ? "+" : "-"}{formatVND(tx.amountVnd)}
                </span>
              </div>
            ))}
          </div>
        )}

        {!loading && transactions.length < total && (
          <button className="ghost-button" onClick={() => setOffset((o) => o + limit)}>
            Tải thêm ({transactions.length}/{total})
          </button>
        )}
      </section>
    </>
  );
}
