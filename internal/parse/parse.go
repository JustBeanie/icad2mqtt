package parse

import (
	"fmt"
	"golang.org/x/net/html"
	"strings"
	"unicode"
)

type Reason string

const (
	NoHeader          Reason = "no_header"
	ConflictingHeader Reason = "duplicate_conflicting_headers"
)

type PageError struct{ Reason Reason }

func (e *PageError) Error() string { return fmt.Sprintf("parse page: %s", e.Reason) }

type Result struct {
	Rows        [][]string
	UpdatedRaw  string
	SkippedRows int
	Warnings    int
}

func Clean(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	var b strings.Builder
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
func text(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(text(c))
	}
	return b.String()
}
func cells(n *html.Node) []string {
	var out []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			out = append(out, Clean(text(c)))
		}
	}
	return out
}
func rows(n *html.Node) [][]string {
	var out [][]string
	if n.Type == html.ElementNode && n.Data == "tr" {
		if x := cells(n); len(x) > 0 {
			out = append(out, x)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out = append(out, rows(c)...)
	}
	return out
}
func hasAttr(n *html.Node, key, value string) bool {
	for _, a := range n.Attr {
		if a.Key == key && a.Val == value {
			return true
		}
	}
	return false
}
func Parse(src string) (Result, error) {
	root, e := html.Parse(strings.NewReader(src))
	if e != nil {
		return Result{}, e
	}
	var header []string
	var all [][]string
	for _, r := range rows(root) {
		all = append(all, r)
		norm := make([]string, len(r))
		for i, v := range r {
			norm[i] = strings.ToLower(v)
		}
		if len(r) >= 6 && norm[0] == "agency" && norm[1] == "date/time" && norm[3] == "address" && norm[5] == "cross streets" {
			if header != nil && strings.Join(header, "|") != strings.Join(norm, "|") {
				return Result{}, &PageError{ConflictingHeader}
			}
			header = norm
		}
	}
	if header == nil {
		return Result{}, &PageError{NoHeader}
	}
	res := Result{}
	for _, r := range all {
		if len(r) == 6 && strings.EqualFold(r[0], "agency") {
			continue
		}
		if len(r) != 6 {
			if len(r) > 6 && strings.Contains(r[1], "/") {
				res.SkippedRows++
			}
			continue
		}
		if r[0] == "" && r[1] == "" {
			continue
		}
		res.Rows = append(res.Rows, r)
	}
	if len(res.Rows) == 0 { // header-only is valid structure
	} // Updated text is deliberately found outside row shape.
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "span" && hasAttr(n, "id", "cdate") {
			res.UpdatedRaw = Clean(text(n))
		}
		if n.Type == html.ElementNode && n.Data == "span" && strings.EqualFold(Clean(text(n)), "Updated:") {
			if n.Parent != nil {
				res.UpdatedRaw = Clean(text(n.Parent))
				res.UpdatedRaw = strings.TrimSpace(strings.TrimPrefix(res.UpdatedRaw, "Updated:"))
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	return res, nil
}
