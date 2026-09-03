package main

import (
	"log"
	"net/http"

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

	log.Printf("[payment] listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("[payment] server error: %v", err)
	}
}
