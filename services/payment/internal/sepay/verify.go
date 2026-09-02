package sepay

import "fmt"

// VerifyWebhook checks the "Authorization: Apikey <key>" header.
func VerifyWebhook(authHeader string) bool {
	if authHeader == "" {
		return false
	}
	return authHeader == fmt.Sprintf("Apikey %s", GetConfig().WebhookAPIKey)
}
