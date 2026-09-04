package model

import (
	"database/sql"
	"time"
)

// WalletTransaction represents a single entry in the append-only wallet ledger.
// Balance = SUM(credit) - SUM(debit), never stored as a column.
type WalletTransaction struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"userId"`
	Type      string    `json:"type"` // deposit, purchase, refund, adjustment
	AmountVnd int       `json:"amountVnd"`
	Direction string    `json:"direction"` // credit, debit
	RefID     string    `json:"refId"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
}

// WalletTransactionListItem is the API response shape for listing transactions.
type WalletTransactionListItem struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Direction string `json:"direction"`
	AmountVnd int    `json:"amountVnd"`
	Label     string `json:"label"`
	Note      string `json:"note,omitempty"`
	CreatedAt string `json:"createdAt"`
}

// PaymentOrder represents a deposit order created via SePay.
type PaymentOrder struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"userId"`
	SepayOrderCode    string     `json:"sepayOrderCode"`
	AmountVnd         int        `json:"amountVnd"`
	Status            string     `json:"status"` // PENDING, PAID, CANCELLED, EXPIRED
	QRCodeURL         string     `json:"qrCodeUrl"`
	ExpiredAt         *time.Time `json:"expiredAt"`
	WebhookReceivedAt *time.Time `json:"webhookReceivedAt"`
	CreatedAt         time.Time  `json:"createdAt"`
}

// StudySetPrice stores the price for a paid study set.
// No row = free.
type StudySetPrice struct {
	StudySetID  int64     `json:"studySetId"`
	PricingType string    `json:"pricingType"` // free, one_time
	PriceVnd    int       `json:"priceVnd"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Entitlement represents access to a paid study set.
type Entitlement struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"userId"`
	StudySetID int64      `json:"studySetId"`
	GrantedVia string     `json:"grantedVia"` // purchase, free, admin_grant
	TxID       *int64     `json:"txId"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// StudySetOwnerInfo is the minimal info returned by Study service internal API.
type StudySetOwnerInfo struct {
	OwnerUserID int64  `json:"ownerUserId"`
	IsPublic    bool   `json:"isPublic"`
	Title       string `json:"title"`
}

// --- Request/Response types ---

type CreateDepositOrderRequest struct {
	AmountVnd int `json:"amountVnd"`
}

type CreateDepositOrderResponse struct {
	OrderID           int64  `json:"orderId"`
	OrderCode         string `json:"orderCode"`
	BankAccountNumber string `json:"bankAccountNumber"`
	BankAccountHolder string `json:"bankAccountHolder"`
	BankName          string `json:"bankName"`
	AmountVnd         int    `json:"amountVnd"`
	QRCodeURL         string `json:"qrCodeUrl"`
}

type DepositOrderStatusResponse struct {
	OrderID   int64  `json:"orderId"`
	Status    string `json:"status"`
	AmountVnd int    `json:"amountVnd"`
	CreatedAt string `json:"createdAt"`
	QRCodeURL string `json:"qrCodeUrl"`
}

type PendingDepositOrderResponse struct {
	OrderID   int64  `json:"orderId"`
	OrderCode string `json:"orderCode"`
	AmountVnd int    `json:"amountVnd"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	ExpiredAt string `json:"expiredAt,omitempty"`
}

type WalletBalanceResponse struct {
	Balance int `json:"balance"`
}

type WalletTransactionListResponse struct {
	Items []WalletTransactionListItem `json:"items"`
	Total int                         `json:"total"`
}

type SePayWebhookPayload struct {
	ID              int64  `json:"id"`
	Gateway         string `json:"gateway"`
	TransactionDate string `json:"transactionDate"`
	Code            string `json:"code"`
	TransferType    string `json:"transferType"`
	TransferAmount  int    `json:"transferAmount"`
	Content         string `json:"content"`
	Description     string `json:"description"`
	ReferenceCode   string `json:"referenceCode"`
}

type WebhookResponse struct {
	Success bool `json:"success"`
}

type PurchaseRequest struct {
	StudySetID int64 `json:"studySetId"`
}

type PurchaseResponse struct {
	Balance  int `json:"balance"`
	PriceVnd int `json:"priceVnd"`
}

type StudySetAccessInfo struct {
	PricingType      string  `json:"pricingType"`
	PriceVnd         int     `json:"priceVnd"`
	HasAccess        bool    `json:"hasAccess"`
	RequiresPurchase bool    `json:"requiresPurchase"`
	IsOwner          bool    `json:"isOwner"`
	GrantedVia       *string `json:"grantedVia,omitempty"`
}

type SetStudySetPriceRequest struct {
	PricingType string `json:"pricingType"`
	PriceVnd    int    `json:"priceVnd"`
}

type AdminCreditRequest struct {
	UserID    int64  `json:"userId"`
	AmountVnd int    `json:"amountVnd"`
	Note      string `json:"note"`
}

type AdminOrderListResponse struct {
	Items []PaymentOrder `json:"items"`
	Total int            `json:"total"`
}

type AdminTxListResponse struct {
	Items []WalletTransactionListItem `json:"items"`
	Total int                         `json:"total"`
}

// NullInt64 helper
func NullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
