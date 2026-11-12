package subtitles

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 1 << 20

var timing = regexp.MustCompile(`^(\d{2,}):([0-5]\d):([0-5]\d)[,.](\d{3})\s+-->\s+(\d{2,}):([0-5]\d):([0-5]\d)[,.](\d{3})$`)

func Normalize(raw string) (string, error) {
	if len(raw) > MaxBytes || !utf8.ValidString(raw) || strings.ContainsRune(raw, 0) {
		return "", errors.New("SRT phải là UTF-8 và không quá 1 MiB")
	}
	text := strings.TrimSpace(strings.TrimPrefix(strings.ReplaceAll(raw, "\r\n", "\n"), "\ufeff"))
	if text == "" {
		return "", nil
	}
	blocks := regexp.MustCompile(`\n\s*\n`).Split(text, -1)
	if len(blocks) > 10000 {
		return "", errors.New("SRT có quá nhiều đoạn")
	}
	for _, block := range blocks {
		lines := strings.Split(block, "\n")
		if len(lines) < 3 {
			return "", errors.New("SRT cần số thứ tự, thời gian và nội dung")
		}
		if n, e := strconv.Atoi(strings.TrimSpace(lines[0])); e != nil || n < 1 {
			return "", errors.New("Số thứ tự SRT không hợp lệ")
		}
		m := timing.FindStringSubmatch(strings.TrimSpace(lines[1]))
		if m == nil {
			return "", errors.New("Mốc thời gian SRT không hợp lệ")
		}
		ms := func(start int) int64 {
			var v [4]int64
			for i := range v {
				v[i], _ = strconv.ParseInt(m[start+i], 10, 64)
			}
			return ((v[0]*60+v[1])*60+v[2])*1000 + v[3]
		}
		if ms(5) <= ms(1) || strings.TrimSpace(strings.Join(lines[2:], "\n")) == "" {
			return "", errors.New("Thời gian kết thúc SRT phải sau thời gian bắt đầu")
		}
	}
	return text + "\n", nil
}
