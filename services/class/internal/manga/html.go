package manga

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/net/html"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type HTMLPage struct {
	Page
	ImagePath string `json:"img_path"`
}
type HTMLVolume struct {
	SourceUUID string     `json:"sourceUuid"`
	Title      string     `json:"title"`
	Volume     string     `json:"volume"`
	Pages      []HTMLPage `json:"pages"`
}

var styleNumber = regexp.MustCompile(`(?i)(?:^|;)\s*([a-z-]+)\s*:\s*(-?[0-9]+(?:\.[0-9]+)?)(?:px)?\s*(?:;|$)`)
var backgroundURL = regexp.MustCompile(`(?i)background-image\s*:\s*url\(\s*["']?([^"')]+)["']?\s*\)`)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
func descendants(n *html.Node, predicate func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if predicate(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
func number(style, key string) (float64, error) {
	// Split properties first so consuming a semicolon cannot skip the next property.
	for _, declaration := range strings.Split(style, ";") {
		pair := strings.SplitN(declaration, ":", 2)
		if len(pair) != 2 || strings.TrimSpace(pair[0]) != key {
			continue
		}
		value := strings.TrimSuffix(strings.TrimSpace(pair[1]), "px")
		return strconv.ParseFloat(value, 64)
	}
	return 0, fmt.Errorf("Thiếu %s trong HTML Mokuro", key)
}
func safeImagePath(raw string) (string, error) {
	decoded, e := url.PathUnescape(raw)
	if e != nil {
		return "", errors.New("Đường dẫn ảnh không hợp lệ")
	}
	u, e := url.Parse(decoded)
	if e != nil || u.IsAbs() || u.Host != "" || strings.HasPrefix(decoded, "/") || strings.ContainsAny(decoded, "\\:\x00") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("Ảnh phải là đường dẫn tương đối trong thư mục truyện")
	}
	for _, s := range strings.Split(decoded, "/") {
		if s == "" || s == "." || s == ".." {
			return "", errors.New("Đường dẫn ảnh không hợp lệ")
		}
	}
	return decoded, nil
}
func parseHTML(source string) (*html.Node, []*html.Node, error) {
	if len(source) == 0 || len(source) > 8<<20 {
		return nil, nil, errors.New("HTML tối đa 8 MiB")
	}
	doc, e := html.Parse(strings.NewReader(source))
	if e != nil {
		return nil, nil, errors.New("Không đọc được HTML")
	}
	pages := descendants(doc, func(n *html.Node) bool { return n.Type == html.ElementNode && hasClass(n, "page") })
	if len(pages) < 1 || len(pages) > 1000 {
		return nil, nil, errors.New("Cần HTML Mokuro có từ 1 đến 1.000 trang")
	}
	return doc, pages, nil
}
func ParseHTML(source, filename string) (HTMLVolume, error) {
	result := HTMLVolume{}
	doc, nodes, e := parseHTML(source)
	if e != nil {
		return result, e
	}
	sum := sha256.Sum256([]byte(source))
	result.SourceUUID = "html-" + hex.EncodeToString(sum[:])
	result.Volume = strings.TrimSuffix(path.Base(filename), path.Ext(filename))
	result.Title = result.Volume
	titles := descendants(doc, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "title" })
	if len(titles) > 0 {
		result.Title = strings.TrimSpace(strings.TrimSuffix(text(titles[0]), "| mokuro"))
	}
	if len([]rune(result.Title)) > 200 {
		result.Title = string([]rune(result.Title)[:200])
	}
	if result.Title == "" {
		result.Title = "Truyện"
	}
	for index, node := range nodes {
		containers := descendants(node, func(n *html.Node) bool { return n.Type == html.ElementNode && hasClass(n, "pageContainer") })
		if len(containers) != 1 {
			return result, fmt.Errorf("Trang %d không có khung ảnh Mokuro hợp lệ", index+1)
		}
		style := attr(containers[0], "style")
		w, e1 := number(style, "width")
		h, e2 := number(style, "height")
		match := backgroundURL.FindStringSubmatch(style)
		if e1 != nil || e2 != nil || len(match) != 2 {
			return result, fmt.Errorf("Trang %d thiếu kích thước hoặc ảnh nền", index+1)
		}
		imagePath, e := safeImagePath(match[1])
		if e != nil {
			return result, e
		}
		p := HTMLPage{Page: Page{Width: int(w), Height: int(h), Blocks: []Block{}}, ImagePath: imagePath}
		for _, box := range descendants(containers[0], func(n *html.Node) bool { return n.Type == html.ElementNode && hasClass(n, "textBox") }) {
			s := attr(box, "style")
			x, e1 := number(s, "left")
			y, e2 := number(s, "top")
			width, e3 := number(s, "width")
			height, e4 := number(s, "height")
			if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
				return result, errors.New("Vị trí khung thoại HTML không hợp lệ")
			}
			font, _ := number(s, "font-size")
			b := Block{Box: [4]float64{x, y, x + width, y + height}, Vertical: height > width, FontSize: font, Lines: []string{}}
			ps := descendants(box, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "p" })
			for _, paragraph := range ps {
				b.Lines = append(b.Lines, text(paragraph))
			}
			if len(b.Lines) == 0 {
				b.Lines = []string{text(box)}
			}
			p.Blocks = append(p.Blocks, b)
		}
		if e = Validate(&p.Page); e != nil {
			return result, fmt.Errorf("Trang %d: %w", index+1, e)
		}
		result.Pages = append(result.Pages, p)
	}
	return result, nil
}

// ExportHTML replaces only text box paragraphs. CSS, reader scripts, image paths
// and layout attributes remain in the original document. Imported code is never
// executed by the web app; this output is delivered as a downloadable file.
func ExportHTML(source string, pages []Page) ([]byte, error) {
	doc, nodes, e := parseHTML(source)
	if e != nil {
		return nil, e
	}
	if len(nodes) != len(pages) {
		return nil, errors.New("Số trang không khớp HTML gốc")
	}
	for i, node := range nodes {
		boxes := descendants(node, func(n *html.Node) bool { return n.Type == html.ElementNode && hasClass(n, "textBox") })
		if len(boxes) != len(pages[i].Blocks) {
			return nil, errors.New("Số khung OCR không khớp HTML gốc")
		}
		for j, box := range boxes {
			var attributes []html.Attribute
			paragraphs := descendants(box, func(n *html.Node) bool { return n.Type == html.ElementNode && n.Data == "p" })
			if len(paragraphs) > 0 {
				attributes = paragraphs[0].Attr
			}
			for box.FirstChild != nil {
				box.RemoveChild(box.FirstChild)
			}
			for _, line := range pages[i].Blocks[j].Lines {
				p := &html.Node{Type: html.ElementNode, Data: "p", Attr: append([]html.Attribute(nil), attributes...)}
				p.AppendChild(&html.Node{Type: html.TextNode, Data: line})
				box.AppendChild(p)
			}
		}
	}
	var b bytes.Buffer
	e = html.Render(&b, doc)
	if e != nil && e != io.EOF {
		return nil, e
	}
	return b.Bytes(), nil
}
