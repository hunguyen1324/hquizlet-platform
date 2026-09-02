package sepay

import (
	"net/url"
)

// BuildVietQrURL returns a VietQR image URL for the given bank account info.
// Uses "des" parameter (not "note") per vietqr.app docs.
func BuildVietQrURL(accountNumber, bankBin string, amountVnd int, note string) string {
	q := url.Values{}
	q.Set("acc", accountNumber)
	q.Set("bank", bankBin)
	q.Set("amount", fmtInt(amountVnd))
	q.Set("des", note)
	q.Set("template", "compact")
	return "https://vietqr.app/img?" + q.Encode()
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
