// StudySetPaywall — Phase 8: Gates study set content behind payment
import React, { useEffect, useState } from "react";
import { useAuth } from "../auth/AuthContext";
import { entitlementApi } from "../../lib/api";
import type { StudySetAccessInfo } from "../../types";

type Props = {
  studySetId: number;
  isOwner: boolean;
  children: React.ReactNode;
};

function formatVND(amount: number): string {
  return amount.toLocaleString("vi-VN") + "₫";
}

export function StudySetPaywall({ studySetId, isOwner, children }: Props) {
  const { token } = useAuth();
  const [info, setInfo] = useState<StudySetAccessInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [purchasing, setPurchasing] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) {
      setLoading(false);
      return;
    }
    void entitlementApi
      .checkAccess(token, studySetId)
      .then((data) => setInfo(data))
      .catch(() => setInfo({ pricingType: "free", priceVnd: 0, hasAccess: true, requiresPurchase: false, isOwner }))
      .finally(() => setLoading(false));
  }, [token, studySetId, isOwner]);

  if (loading) {
    return <div className="loading-skeleton" aria-busy="true"><div className="skeleton-row" /></div>;
  }

  // Free set or already has access or is owner
  if (!info || info.hasAccess || info.isOwner || info.pricingType === "free") {
    return <>{children}</>;
  }

  // Requires purchase
  const handlePurchase = async () => {
    setPurchasing(true);
    setError("");
    try {
      await entitlementApi.purchase(token, studySetId);
      // Reload access info
      const updated = await entitlementApi.checkAccess(token, studySetId);
      setInfo(updated);
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Mua thất bại";
      if (msg.includes("insufficient")) {
        setError("Số dư không đủ. Vui lòng nạp thêm tiền.");
      } else {
        setError(msg);
      }
    } finally {
      setPurchasing(false);
    }
  };

  return (
    <div className="paywall-overlay">
      <div className="paywall-card">
        <h2>🔒 Nội dung trả phí</h2>
        <p>Học phần này yêu cầu thanh toán để xem nội dung.</p>
        <div className="paywall-price">
          <span>Giá:</span>
          <strong>{formatVND(info.priceVnd)}</strong>
        </div>
        {error && <p className="message message--error">{error}</p>}
        <div className="paywall-actions">
          <button className="primary-button" onClick={() => void handlePurchase()} disabled={purchasing}>
            {purchasing ? "Đang mua..." : `Mua ngay (${formatVND(info.priceVnd)})`}
          </button>
          {error.includes("nạp thêm") && (
            <button className="ghost-button" onClick={() => window.dispatchEvent(new CustomEvent("navigate-wallet"))}>
              Nạp tiền
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
