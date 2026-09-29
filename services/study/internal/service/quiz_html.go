package service

import (
	"bytes"
	"golang.org/x/net/html"
	"strings"
)

// Preserve basic text formatting but never ship embedded source URLs, event handlers,
// scripts, or hidden DOM attributes from imported HTML to a learner.
func safeQuizHTML(raw string) string {
	if !strings.Contains(raw, "<") {
		return raw
	}
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	allowed := map[string]bool{"p": true, "br": true, "b": true, "strong": true, "em": true, "i": true, "u": true, "ul": true, "ol": true, "li": true, "table": true, "tbody": true, "thead": true, "tr": true, "td": true, "th": true, "blockquote": true, "span": true, "div": true, "sup": true, "sub": true}
	blocked := map[string]bool{"head": true, "script": true, "style": true, "iframe": true, "object": true, "embed": true, "audio": true, "video": true, "source": true, "link": true, "meta": true}
	var out bytes.Buffer
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.TextNode {
			out.WriteString(html.EscapeString(n.Data))
			return
		}
		if n.Type == html.ElementNode && blocked[n.Data] {
			return
		}
		if n.Type == html.ElementNode && n.Data == "img" {
			for _, a := range n.Attr {
				if a.Key == "alt" {
					out.WriteString(html.EscapeString(a.Val))
				}
			}
			return
		}
		keep := n.Type == html.ElementNode && allowed[n.Data]
		if keep {
			out.WriteString("<" + n.Data + ">")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
		if keep && n.Data != "br" {
			out.WriteString("</" + n.Data + ">")
		}
	}
	visit(doc)
	return out.String()
}
