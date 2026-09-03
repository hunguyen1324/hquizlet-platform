package sepay

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const defaultBaseURL = "https://userapi.sepay.vn/v2"

type Config struct {
	APIToken      string
	BankAccountID string
	VAAccount     string
	WebhookAPIKey string
	BaseURL       string
}

var (
	once   sync.Once
	config *Config
)

// Init configures the SePay client. Must be called once at startup.
func Init(apitoken, bankAccountID, vaAccount, webhookAPIKey, baseURL string) {
	once.Do(func() {
		if baseURL == "" {
			baseURL = defaultBaseURL
		}
		config = &Config{
			APIToken:      apitoken,
			BankAccountID: bankAccountID,
			VAAccount:     vaAccount,
			WebhookAPIKey: webhookAPIKey,
			BaseURL:       baseURL,
		}
	})
}

// GetConfig returns the initialized config. Panics if Init was not called.
func GetConfig() *Config {
	if config == nil {
		panic("sepay: call Init() before GetConfig()")
	}
	return config
}

type BankAccount struct {
	ID                string `json:"id"`
	AccountNumber     string `json:"account_number"`
	AccountHolderName string `json:"account_holder_name"`
	BankShortName     string `json:"bank_short_name"`
	BankBin           string `json:"bank_bin"`
	VA                string `json:"va,omitempty"`
	VANumber          string `json:"va_number,omitempty"`
	VAHolderName      string `json:"va_holder_name,omitempty"`
}

type Transaction struct {
	ID                 string `json:"id"`
	TransactionDate    string `json:"transaction_date"`
	AccountNumber      string `json:"account_number"`
	VA                 string `json:"va"`
	TransferType       string `json:"transfer_type"`
	AmountIn           int    `json:"amount_in"`
	AmountOut          int    `json:"amount_out"`
	Accumulated        int    `json:"accumulated"`
	TransactionContent string `json:"transaction_content"`
	ReferenceNumber    string `json:"reference_number"`
	Code               string `json:"code"`
	BankBrandName      string `json:"bank_brand_name"`
	BankAccountID      string `json:"bank_account_id"`
	WebhookSuccess     *int   `json:"webhook_success"`
}

var (
	bankAccountMu    sync.Mutex
	bankAccountCache *BankAccount
)

// GetBankAccount fetches the bank account info from SePay and caches it.
func GetBankAccount() (*BankAccount, error) {
	bankAccountMu.Lock()
	defer bankAccountMu.Unlock()
	if bankAccountCache != nil {
		return bankAccountCache, nil
	}
	cfg := GetConfig()
	url := cfg.BaseURL + "/bank-accounts"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("sepay: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sepay: get bank accounts: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sepay: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sepay: bank accounts returned %d: %s", resp.StatusCode, string(body))
	}

	var accounts []BankAccount
	if err := json.Unmarshal(body, &accounts); err != nil {
		var envelope struct {
			Data []BankAccount `json:"data"`
		}
		if err2 := json.Unmarshal(body, &envelope); err2 == nil && len(envelope.Data) > 0 {
			accounts = envelope.Data
		} else {
			var single BankAccount
			if err3 := json.Unmarshal(body, &single); err3 != nil {
				return nil, fmt.Errorf("sepay: parse bank accounts: %w", err)
			}
			accounts = []BankAccount{single}
		}
	}

	for _, a := range accounts {
		if a.ID == cfg.BankAccountID {
			bankAccountCache = &a
			return bankAccountCache, nil
		}
	}

	log.Printf("[payment] WARNING: bank account %s not found in SePay response, using first available", cfg.BankAccountID)
	if len(accounts) > 0 {
		bankAccountCache = &accounts[0]
		return bankAccountCache, nil
	}

	return nil, fmt.Errorf("sepay: no bank accounts found")
}

// FindIncomingTransaction searches SePay transactions for a paid deposit order.
func FindIncomingTransaction(orderCode string, amountVnd int) (*Transaction, error) {
	cfg := GetConfig()
	u, err := url.Parse(cfg.BaseURL + "/transactions")
	if err != nil {
		return nil, fmt.Errorf("sepay: parse transactions url: %w", err)
	}
	q := u.Query()
	q.Set("q", orderCode)
	q.Set("transfer_type", "in")
	q.Set("amount_in_min", fmt.Sprintf("%d", amountVnd))
	q.Set("amount_in_max", fmt.Sprintf("%d", amountVnd))
	q.Set("per_page", "10")
	q.Set("timestamp_format", "iso8601")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("sepay: create transactions request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sepay: list transactions: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sepay: read transactions body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sepay: transactions returned %d: %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Data []Transaction `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("sepay: parse transactions: %w", err)
	}
	for _, tx := range envelope.Data {
		if tx.TransferType == "in" && tx.AmountIn == amountVnd && (tx.Code == orderCode || containsCode(tx.TransactionContent, orderCode)) {
			return &tx, nil
		}
	}
	return nil, nil
}

func containsCode(content, code string) bool {
	return code != "" && strings.Contains(content, code)
}
