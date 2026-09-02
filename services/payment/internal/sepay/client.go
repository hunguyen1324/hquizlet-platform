package sepay

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

const defaultBaseURL = "https://userapi.sepay.vn/v2"

type Config struct {
	APIToken      string
	BankAccountID string
	WebhookAPIKey string
	BaseURL       string
}

var (
	once   sync.Once
	config *Config
)

// Init configures the SePay client. Must be called once at startup.
func Init(apitoken, bankAccountID, webhookAPIKey, baseURL string) {
	once.Do(func() {
		if baseURL == "" {
			baseURL = defaultBaseURL
		}
		config = &Config{
			APIToken:      apitoken,
			BankAccountID: bankAccountID,
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
	VANumber          string `json:"va_number,omitempty"`
	VAHolderName      string `json:"va_holder_name,omitempty"`
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

	// SePay returns a list of bank accounts
	var accounts []BankAccount
	if err := json.Unmarshal(body, &accounts); err != nil {
		// Try single object response
		var single BankAccount
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			return nil, fmt.Errorf("sepay: parse bank accounts: %w", err)
		}
		accounts = []BankAccount{single}
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
