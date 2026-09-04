package http

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/handler"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

// Deps holds all dependencies for the payment service router.
type Deps struct {
	DB              *sql.DB
	WalletRepo      *repository.WalletRepo
	OrderRepo       *repository.OrderRepo
	EntitlementRepo *repository.EntitlementRepo
	PriceRepo       *repository.PriceRepo
	OrderSvc        *service.OrderService
	WebhookSvc      *service.WebhookService
	PurchaseSvc     *service.PurchaseService
	AccessSvc       *service.AccessService
	WalletSvc       *service.WalletService
	AdminToken      string
}

// NewRouter builds the payment service HTTP router.
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	// Handlers
	walletH := handler.NewWalletHandler(deps.WalletSvc)
	orderH := handler.NewOrderHandler(deps.OrderSvc)
	webhookH := handler.NewWebhookHandler(deps.WebhookSvc)
	purchaseH := handler.NewPurchaseHandler(deps.PurchaseSvc)
	entitlementH := handler.NewEntitlementHandler(deps.AccessSvc, deps.EntitlementRepo)
	priceH := handler.NewPriceHandler(deps.PriceRepo, deps.AccessSvc)
	adminH := handler.NewAdminHandler(deps.WalletSvc, deps.OrderRepo)

	// Health check (unauthenticated)
	mux.HandleFunc("GET /healthz", healthHandler("payment", deps.DB))

	// ── Webhook (NO auth, NO gateway header stripping) ──────────────────
	// SePay authenticates via Apikey header, gateway forwards raw.
	mux.Handle("POST /v1/payments/webhooks/sepay",
		middleware.StripSpoofedHeaders(webhookH),
	)

	// ── Internal endpoints (for other services) ──────────────────────────
	mux.Handle("GET /internal/payment/entitlements/check",
		middleware.OptionalAuth(entitlementH),
	)

	// ── Authenticated routes ─────────────────────────────────────────────
	// Wallet
	mux.Handle("GET /v1/wallet",
		middleware.Auth(walletH),
	)
	mux.Handle("GET /v1/wallet/transactions",
		middleware.Auth(walletH),
	)

	// Payment orders
	mux.Handle("POST /v1/payments/orders",
		middleware.Auth(orderH),
	)
	mux.Handle("GET /v1/payments/orders/pending",
		middleware.Auth(orderH),
	)
	mux.Handle("DELETE /v1/payments/orders/",
		middleware.Auth(orderH),
	)
	mux.Handle("GET /v1/payments/orders/",
		middleware.Auth(orderH),
	)

	// Entitlements
	mux.Handle("POST /v1/entitlements/purchase",
		middleware.Auth(purchaseH),
	)
	mux.Handle("GET /v1/entitlements/check",
		middleware.OptionalAuth(entitlementH),
	)
	mux.Handle("GET /v1/entitlements",
		middleware.Auth(entitlementH),
	)

	// Study set price (owner only - checked in handler via access service)
	mux.Handle("PUT /v1/study-sets/",
		middleware.Auth(priceH),
	)

	// ── Admin routes ──────────────────────────────────────────────────────
	mux.Handle("GET /v1/admin/payments/orders",
		middleware.Auth(middleware.RequireAdmin(adminH)),
	)
	mux.Handle("GET /v1/admin/wallet/transactions",
		middleware.Auth(middleware.RequireAdmin(adminH)),
	)
	mux.Handle("POST /v1/admin/wallet/credit",
		middleware.Auth(middleware.RequireAdmin(adminH)),
	)

	return middleware.RequestID(middleware.Logging(mux))
}

func healthHandler(serviceName string, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := map[string]string{"service": serviceName, "status": "ok", "database": "ok"}
		status := http.StatusOK
		if err := db.PingContext(r.Context()); err != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "degraded"
			body["database"] = "offline"
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}
