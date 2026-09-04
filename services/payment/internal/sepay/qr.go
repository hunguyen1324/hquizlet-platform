package sepay

import (
	"fmt"
	"net/url"
)

// BuildVietQrURL returns a VietQR image URL using VietQR's quick link format.
func BuildVietQrURL(accountNumber, bankCode, holderName string, amountVnd int, note string) string {
	q := url.Values{}
	q.Set("amount", fmtInt(amountVnd))
	q.Set("addInfo", note)
	if holderName != "" {
		q.Set("accountName", holderName)
	}
	return fmt.Sprintf("https://img.vietqr.io/image/%s-%s-compact2.png?%s", url.PathEscape(bankCode), url.PathEscape(accountNumber), q.Encode())
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
