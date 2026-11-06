// WalletPage — Phase 8: Wallet balance + transaction history
import React, { useEffect, useState, useCallback } from "react";
import { useAuth } from "../auth/AuthContext";
import { walletApi } from "../../lib/api";
import type { WalletTransactionItem } from "../../types";
import { WorkspaceEmpty, WorkspaceHeading, WorkspaceIcon } from "../../components/ui/WorkspaceUI";

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
      setTransactions(prev => offset === 0 ? txs.items : [...prev, ...txs.items]);
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
    <div className="wallet-page workspace-page">
      <WorkspaceHeading eyebrow="TÀI KHOẢN" title="Ví của tôi" description="Theo dõi số dư và quản lý chi tiêu học tập của bạn." action={<button className="ghost-button" onClick={onBack}>← Thư viện học phần</button>} />
      <div className="wallet-overview">
        <section className="wallet-balance-card" aria-label="Số dư ví">
          <div className="wallet-balance-top"><span className="wallet-symbol"><WorkspaceIcon name="wallet" size={26} /></span><span>HQuizlet Wallet</span></div>
          <p>Số dư khả dụng</p>
          <strong className="wallet-balance" aria-live="polite">{balance !== null ? formatVND(balance) : "—"}</strong>
          <div className="wallet-balance-footer"><span>Sẵn sàng cho hành trình học tập</span><button className="wallet-deposit-button" onClick={onDeposit}><WorkspaceIcon name="plus" size={18} /> Nạp tiền</button></div>
        </section>
        <aside className="wallet-guide-card">
          <span className="workspace-icon-tile"><WorkspaceIcon name="sparkles" /></span>
          <h2>Đầu tư cho kiến thức</h2><p>Sử dụng số dư trong ví để mở khóa những học phần bạn yêu thích.</p>
          <div className="wallet-guide-step"><span>1</span> Nạp số tiền bạn muốn</div>
          <div className="wallet-guide-step"><span>2</span> Chọn học phần và bắt đầu khám phá</div>
        </aside>
      </div>

      {error && <p className="message message--error" role="alert">{error} <button className="ghost-button" onClick={() => void loadData()}>Thử lại</button></p>}

      <section className="panel wallet-history">
        <div className="workspace-section-heading"><div><h2>Lịch sử giao dịch</h2><p>Mọi thay đổi số dư, ngay tại đây.</p></div><span className="workspace-count">{total} giao dịch</span></div>
        {loading && transactions.length === 0 && <div className="loading-skeleton" aria-busy="true" aria-label="Đang tải giao dịch"><div className="skeleton-row" /><div className="skeleton-row" /><div className="skeleton-row" /></div>}

        {!loading && !error && transactions.length === 0 && <WorkspaceEmpty icon="wallet" title="Chưa có giao dịch nào" description="Lịch sử nạp tiền và mua học phần sẽ được lưu tại đây để bạn tiện theo dõi." action={<button className="secondary-button" onClick={onDeposit}><WorkspaceIcon name="plus" size={17} /> Nạp tiền vào ví</button>} />}

        {transactions.length > 0 && (
          <div className="transaction-list">
            {transactions.map((tx) => (
              <div className="transaction-item" key={tx.id}>
                <span className={`tx-icon ${tx.direction === "credit" ? "credit" : "debit"}`} aria-hidden="true">{tx.direction === "credit" ? "↙" : "↗"}</span>
                <div className="tx-info">
                  <span className="tx-label">{tx.label}</span>
                  <span className="tx-date">{new Date(tx.createdAt).toLocaleDateString("vi-VN")}</span>
                  {tx.note && <span className="tx-note">{tx.note}</span>}
                </div>
                <span className={`tx-amount ${tx.direction === "credit" ? "credit" : "debit"}`}>
                  {tx.direction === "credit" ? "+" : "-"}{formatVND(tx.amountVnd)}
                </span>
              </div>
            ))}
          </div>
        )}

        {transactions.length > 0 && transactions.length < total && (
          <button className="ghost-button wallet-load-more" disabled={loading} onClick={() => setOffset((o) => o + limit)}>
            {loading ? "Đang tải..." : `Tải thêm (${transactions.length}/${total})`}
          </button>
        )}
      </section>
    </div>
  );
}
