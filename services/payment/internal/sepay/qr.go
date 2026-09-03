package sepay

import (
	"fmt"
	"net/url"
)

// BuildVietQrURL returns a VietQR image URL using SePay's VietQR app format.
func BuildVietQrURL(accountNumber, bankCode, holderName string, amountVnd int, note string) string {
	q := url.Values{}
	q.Set("bank", bankCode)
	q.Set("acc", accountNumber)
	q.Set("amount", fmtInt(amountVnd))
	q.Set("des", note)
	q.Set("template", "compact")
	q.Set("showinfo", "true")
	if holderName != "" {
		q.Set("holder", holderName)
	}
	return fmt.Sprintf("https://vietqr.app/img?%s", q.Encode())
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	s := make([]byte, 0, 20)
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	return string(s)
}
