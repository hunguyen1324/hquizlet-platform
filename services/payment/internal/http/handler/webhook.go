package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/sepay"
	"github.com/hunguyen1324/hquizlet-platform/services/payment/internal/service"
)

type WebhookHandler struct {
	webhookSvc *service.WebhookService
}

func NewWebhookHandler(webhookSvc *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookSvc: webhookSvc}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Verify Apikey header
	if !sepay.VerifyWebhook(r.Header.Get("Authorization")) {
		log.Printf("[payment] webhook: invalid Apikey")
		WriteJSON(w, http.StatusOK, model.WebhookResponse{Success: false})
		return
	}

	// 2. Parse body
	var payload model.SePayWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[payment] webhook: invalid JSON: %v", err)
		WriteJSON(w, http.StatusOK, model.WebhookResponse{Success: false})
		return
	}

	log.Printf("[payment] webhook: received id=%d gateway=%s code=%s type=%s amount=%d",
		payload.ID, payload.Gateway, payload.Code, payload.TransferType, payload.TransferAmount)

	// 3. Only process incoming transfers
	if payload.TransferType != "in" {
		log.Printf("[payment] webhook: skipping outgoing transfer (type=%s)", payload.TransferType)
		WriteJSON(w, http.StatusOK, model.WebhookResponse{Success: true})
		return
	}

	// 4. Check for DEP code
	if payload.Code == "" {
		log.Printf("[payment] webhook: no code in payload, skipping")
		WriteJSON(w, http.StatusOK, model.WebhookResponse{Success: true})
		return
	}

	// 5. Credit deposit
	result := h.webhookSvc.CreditDepositIfPaid(r.Context(), payload.Code, payload.TransferAmount, payload.ID)
	switch result {
	case service.ResultCredited:
		log.Printf("[payment] webhook: credited order %s amount=%d", payload.Code, payload.TransferAmount)
	case service.ResultAlreadyProcessed:
		log.Printf("[payment] webhook: already processed order %s", payload.Code)
	case service.ResultAmountMismatch:
		log.Printf("[payment] webhook: AMOUNT MISMATCH order %s actual=%d", payload.Code, payload.TransferAmount)
	case service.ResultOrderNotFound:
		log.Printf("[payment] webhook: order not found for code %s", payload.Code)
	}

	// Always return 200 to prevent SePay from retrying
	WriteJSON(w, http.StatusOK, model.WebhookResponse{Success: true})
}
