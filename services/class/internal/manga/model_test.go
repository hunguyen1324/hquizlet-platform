package manga

import (
	"math"
	"testing"
)

func TestOCRValidation(t *testing.T) {
	p := Page{Width: 20, Height: 20, Blocks: []Block{{Box: [4]float64{-1, 1, 10, 20}, Lines: []string{"こんにちは"}, FontSize: 10}}}
	if err := Validate(&p); err != nil || p.Blocks[0].Box[0] != 0 {
		t.Fatal(err, p)
	}
	for _, box := range [][4]float64{{-3, 0, 10, 20}, {0, 0, 100, 10}, {0, 0, 0, 10}, {0, math.NaN(), 10, 20}} {
		p.Blocks[0].Box = box
		if Validate(&p) == nil {
			t.Fatal("accepted invalid OCR box", box)
		}
	}
}
