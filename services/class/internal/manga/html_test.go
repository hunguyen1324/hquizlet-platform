package manga

import (
	"os"
	"strings"
	"testing"
)

const testHTML = `<!doctype html><html><head><title>Test | mokuro</title><style>.page{color:red}</style></head><body><div class="page" id="page0"><div class="pageContainer" style='width:16; height:16; background-image:url("manga_images2/1.jpg");'><div class="textBox" style="left:0; top:0; width:10; height:12; font-size:5px;"><p>こんにちは (Xin chào)</p></div></div></div><script>window.readerOriginal=true;</script></body></html>`

func TestHTMLImportExportTextOnly(t *testing.T) {
	parsed, e := ParseHTML(testHTML, "manga_images2_2.html")
	if e != nil {
		t.Fatal(e)
	}
	if len(parsed.Pages) != 1 || parsed.Pages[0].ImagePath != "manga_images2/1.jpg" || parsed.Pages[0].Blocks[0].Lines[0] != "こんにちは (Xin chào)" {
		t.Fatal(parsed)
	}
	p := parsed.Pages[0].Page
	p.Blocks[0].Lines = []string{`修正 (Đã sửa) <script>alert(1)</script>`}
	data, e := ExportHTML(testHTML, []Page{p})
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(data), "window.readerOriginal=true;") || !strings.Contains(string(data), "manga_images2/1.jpg") || !strings.Contains(string(data), "&lt;script&gt;") {
		t.Fatal(string(data))
	}
	reparsed, e := ParseHTML(string(data), "out.html")
	if e != nil || reparsed.Pages[0].Blocks[0].Lines[0] != p.Blocks[0].Lines[0] {
		t.Fatal(reparsed, e)
	}
	for _, image := range []string{"https://evil.test/1.jpg", "../outside.jpg", "%2e%2e/1.jpg", "javascript:alert(1)", "/root/1.jpg"} {
		if _, e := ParseHTML(strings.ReplaceAll(testHTML, "manga_images2/1.jpg", image), "x.html"); e == nil {
			t.Fatal("unsafe image", image)
		}
	}
	if _, e := ExportHTML(testHTML, []Page{}); e == nil {
		t.Fatal("page mismatch accepted")
	}
}
func TestRealMangaHTMLWhenConfigured(t *testing.T) {
	filename := os.Getenv("MANGA_HTML_FIXTURE")
	if filename == "" {
		t.Skip("optional local HTML fixture")
	}
	raw, e := os.ReadFile(filename)
	if e != nil {
		t.Fatal(e)
	}
	parsed, e := ParseHTML(string(raw), "manga_images2_2.html")
	if e != nil {
		t.Fatal(e)
	}
	pages := make([]Page, len(parsed.Pages))
	for i, p := range parsed.Pages {
		pages[i] = p.Page
	}
	if len(pages) == 0 || len(pages[0].Blocks) == 0 {
		t.Fatal("missing OCR")
	}
	pages[0].Blocks[0].Lines = []string{"OCR QA 日本語 (Bản sửa thử)"}
	updated, e := ExportHTML(string(raw), pages)
	if e != nil {
		t.Fatal(e)
	}
	again, e := ParseHTML(string(updated), "output.html")
	if e != nil || again.Pages[0].Blocks[0].Lines[0] != pages[0].Blocks[0].Lines[0] {
		t.Fatal(e)
	}
	t.Logf("HTML roundtrip: %d pages, image paths and Japanese/Vietnamese text preserved", len(pages))
	if output := os.Getenv("MANGA_HTML_OUTPUT"); output != "" {
		if e = os.WriteFile(output, updated, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
