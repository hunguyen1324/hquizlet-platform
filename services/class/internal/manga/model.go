package manga

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

type Block struct {
	Box         [4]float64      `json:"box"`
	Vertical    bool            `json:"vertical"`
	FontSize    float64         `json:"font_size"`
	Lines       []string        `json:"lines"`
	LinesCoords json.RawMessage `json:"lines_coords,omitempty"`
}
type Page struct {
	Width  int     `json:"img_width"`
	Height int     `json:"img_height"`
	Blocks []Block `json:"blocks"`
}

func Validate(p *Page) error {
	if p.Width < 1 || p.Height < 1 || p.Width > 20000 || p.Height > 20000 || int64(p.Width)*int64(p.Height) > 100000000 || len(p.Blocks) > 1000 {
		return errors.New("Kích thước trang hoặc số khối OCR không hợp lệ")
	}
	total := 0
	for i := range p.Blocks {
		b := &p.Blocks[i]
		for _, v := range b.Box {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return errors.New("Vị trí OCR không hợp lệ")
			}
		}
		// OCR can overshoot an image edge by one or two pixels. Normalize only
		// that rounding error; larger out-of-bounds regions remain invalid.
		if b.Box[0] >= -2 && b.Box[0] < 0 {
			b.Box[0] = 0
		}
		if b.Box[1] >= -2 && b.Box[1] < 0 {
			b.Box[1] = 0
		}
		if b.Box[2] > float64(p.Width) && b.Box[2] <= float64(p.Width)+2 {
			b.Box[2] = float64(p.Width)
		}
		if b.Box[3] > float64(p.Height) && b.Box[3] <= float64(p.Height)+2 {
			b.Box[3] = float64(p.Height)
		}
		if b.Box[0] < 0 || b.Box[1] < 0 || b.Box[2] <= b.Box[0] || b.Box[3] <= b.Box[1] || b.Box[2] > float64(p.Width)+2 || b.Box[3] > float64(p.Height)+2 || len(b.Lines) > 100 || b.FontSize < 0 || b.FontSize > 1000 || math.IsNaN(b.FontSize) || math.IsInf(b.FontSize, 0) {
			return errors.New("Khối OCR nằm ngoài trang hoặc quá lớn")
		}
		for _, line := range b.Lines {
			total += len(line)
			if !utf8.ValidString(line) || utf8.RuneCountInString(line) > 2000 || strings.ContainsRune(line, 0) {
				return errors.New("Nội dung OCR không hợp lệ")
			}
		}
		// Polygon coordinates are not used by the web editor; never retain arbitrary nested JSON.
		b.LinesCoords = nil
	}
	if total > 512*1024 {
		return errors.New("Chữ OCR tối đa 512 KiB mỗi trang")
	}
	return nil
}
