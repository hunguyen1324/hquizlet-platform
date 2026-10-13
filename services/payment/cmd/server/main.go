package main

import (
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/config"
	paymenthttp "github.com/hunguyen1324/hquizlet-platform/services/payment/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/sepay"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/store"
)

func main() {
	cfg := config.Load()

	// Initialize SePay client
	sepay.Init(cfg.SePayAPIToken, cfg.SePayBankAccountID, cfg.SePayVAAccount, cfg.SePayWebhookAPIKey, cfg.SePayBaseURL)

	// Open database
	db := store.Open(cfg.DatabaseURL)
	defer db.Close()

	// Run migrations
	if err := migration.Run(db); err != nil {
		log.Fatalf("[payment] migration failed: %v", err)
	}

	if err := authz.StartDailyMetrics(db, "payment", `SELECT d::date AS day,'revenueVnd' AS name,(SELECT COALESCE(SUM(CASE WHEN type='purchase' AND direction='debit' THEN amount_vnd WHEN type='refund' AND direction='credit' THEN -amount_vnd ELSE 0 END),0) FROM wallet_transaction WHERE created_at>=d AND created_at<d+interval '1 day') AS value FROM generate_series(current_date-179,current_date,interval '1 day') d`); err != nil {
		log.Fatal(err)
	}

	// Wire repositories
	walletRepo := repository.NewWalletRepo(db)
	orderRepo := repository.NewOrderRepo(db)
	entitlementRepo := repository.NewEntitlementRepo(db)
	priceRepo := repository.NewPriceRepo(db)

	// Wire services
	webhookSvc := service.NewWebhookService(db, orderRepo, walletRepo)
	orderSvc := service.NewOrderService(orderRepo, webhookSvc)
	purchaseSvc := service.NewPurchaseService(db, entitlementRepo, priceRepo, walletRepo)
	accessSvc := service.NewAccessService(db, entitlementRepo, priceRepo, cfg.StudyServiceURL)
	walletSvc := service.NewWalletService(walletRepo)

	// Wire router
	router := paymenthttp.NewRouter(paymenthttp.Deps{
		DB:              db,
		WalletRepo:      walletRepo,
		OrderRepo:       orderRepo,
		EntitlementRepo: entitlementRepo,
		PriceRepo:       priceRepo,
		OrderSvc:        orderSvc,
		WebhookSvc:      webhookSvc,
		PurchaseSvc:     purchaseSvc,
		AccessSvc:       accessSvc,
		WalletSvc:       walletSvc,
		AdminToken:      cfg.AdminToken,
	})

	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:8081"
	}
	router = authz.VerifyIdentity(authURL, func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, "/v1/") && r.URL.Path != "/v1/payments/webhooks/sepay" && (r.URL.Path != "/v1/entitlements/check" || r.Header.Get("X-User-ID") != "" || r.Header.Get("Authorization") != "")
	})(router)
	log.Printf("[payment] listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("[payment] server error: %v", err)
	}
}
