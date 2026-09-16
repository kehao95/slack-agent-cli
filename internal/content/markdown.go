// Package content provides local, best-effort document conversions. It does not
// fetch links or execute HTML. Keep the original HTML for lossless recovery.
package content

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// HTMLToMarkdown preserves common document structure, links, tables and code.
// Slack-specific widgets, layout, styling and permissions are not representable
// in Markdown; callers must label the result as a lossy local conversion.
func HTMLToMarkdown(source string) (string, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("parse document HTML: %w", err)
	}
	return strings.TrimSpace(render(doc, 0)), nil
}

func children(n *html.Node, depth int) string {
	var out strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out.WriteString(render(c, depth))
	}
	return out.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func plain(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var out strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		out.WriteString(plain(c))
	}
	return out.String()
}

func escape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "`", "\\`", "#", "\\#").Replace(s)
}

func destination(value string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "", "https", "http", "mailto", "slack":
		return strings.NewReplacer(" ", "%20", "\n", "%0A", "\r", "%0D", "<", "%3C", ">", "%3E", "(", "%28", ")", "%29").Replace(u.String())
	default:
		return ""
	}
}

func codeFence(s string, minimum int) string {
	longest, current := 0, 0
	for _, r := range s {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	if longest+1 > minimum {
		minimum = longest + 1
	}
	return strings.Repeat("`", minimum)
}

func render(n *html.Node, depth int) string {
	if n.Type == html.TextNode {
		// Collapse HTML whitespace without joining words on either side of tags.
		fields := strings.Fields(n.Data)
		if len(fields) == 0 {
			if n.Data != "" {
				return " "
			}
			return ""
		}
		s := strings.Join(fields, " ")
		if strings.TrimLeft(n.Data, " \t\n\r") != n.Data {
			s = " " + s
		}
		if strings.TrimRight(n.Data, " \t\n\r") != n.Data {
			s += " "
		}
		return escape(s)
	}
	if n.Type != html.ElementNode && n.Type != html.DocumentNode {
		return ""
	}
	switch n.Data {
	case "head", "script", "style", "template", "noscript":
		return ""
	case "br":
		return "  \n"
	case "hr":
		return "\n\n---\n\n"
	case "h1", "h2", "h3", "h4", "h5", "h6":
		level, _ := strconv.Atoi(n.Data[1:])
		return "\n\n" + strings.Repeat("#", level) + " " + strings.TrimSpace(children(n, depth)) + "\n\n"
	case "p", "div", "section", "article":
		return "\n\n" + strings.TrimSpace(children(n, depth)) + "\n\n"
	case "strong", "b":
		return "**" + children(n, depth) + "**"
	case "em", "i":
		return "_" + children(n, depth) + "_"
	case "del", "s", "strike":
		return "~~" + children(n, depth) + "~~"
	case "pre":
		s := plain(n)
		fence := codeFence(s, 3)
		return "\n\n" + fence + "\n" + strings.TrimSuffix(s, "\n") + "\n" + fence + "\n\n"
	case "code":
		s := plain(n)
		fence := codeFence(s, 1)
		return fence + " " + s + " " + fence
	case "a":
		label := children(n, depth)
		link := destination(attr(n, "href"))
		if link == "" {
			return label
		}
		if label == "" {
			label = escape(link)
		}
		return "[" + label + "](" + link + ")"
	case "img":
		link := destination(attr(n, "src"))
		if link == "" {
			return escape(attr(n, "alt"))
		}
		return "![" + escape(attr(n, "alt")) + "](" + link + ")"
	case "blockquote":
		s := strings.TrimSpace(children(n, depth))
		return "\n\n> " + strings.ReplaceAll(s, "\n", "\n> ") + "\n\n"
	case "ul", "ol":
		var out strings.Builder
		out.WriteString("\n")
		index := 1
		if start, err := strconv.Atoi(attr(n, "start")); err == nil && start > 0 {
			index = start
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode || c.Data != "li" {
				continue
			}
			prefix := "- "
			if n.Data == "ol" {
				prefix = strconv.Itoa(index) + ". "
			}
			body := strings.TrimSpace(children(c, depth+1))
			indent := strings.Repeat(" ", len(prefix))
			out.WriteString(prefix + strings.ReplaceAll(body, "\n", "\n"+indent) + "\n")
			index++
		}
		out.WriteString("\n")
		return out.String()
	case "input":
		if attr(n, "type") == "checkbox" {
			for _, a := range n.Attr {
				if a.Key == "checked" {
					return "[x] "
				}
			}
			return "[ ] "
		}
		return ""
	case "table":
		return renderTable(n, depth)
	default:
		return children(n, depth)
	}
}

func renderTable(n *html.Node, depth int) string {
	var rows [][]string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node != n && node.Type == html.ElementNode && node.Data == "table" {
			return
		}
		if node.Type == html.ElementNode && node.Data == "tr" {
			var row []string
			for c := node.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
					cell := strings.TrimSpace(children(c, depth))
					row = append(row, strings.NewReplacer("|", "\\|", "\n", "<br>").Replace(cell))
				}
			}
			if len(row) > 0 {
				rows = append(rows, row)
			}
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	if len(rows) == 0 {
		return ""
	}
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	var out strings.Builder
	out.WriteString("\n\n")
	for i, row := range rows {
		for len(row) < width {
			row = append(row, "")
		}
		out.WriteString("| " + strings.Join(row, " | ") + " |\n")
		if i == 0 {
			out.WriteString("|" + strings.Repeat(" --- |", width) + "\n")
		}
	}
	out.WriteString("\n")
	return out.String()
}
