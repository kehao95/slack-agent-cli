package content

import (
	"strings"
	"testing"
)

func TestHTMLToMarkdownDocument(t *testing.T) {
	got, err := HTMLToMarkdown(`<html><head><style>secret-style</style></head><body><h1>Report &amp; notes</h1><p>Hello <strong>team</strong>. <a href="https://example.com/a(b)">Source</a></p><ul><li><input type="checkbox" checked>Done</li></ul><table><tr><th>Name</th><th>Value</th></tr><tr><td>A</td><td>1</td></tr></table><script>secret-script</script></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# Report & notes", "**team**", "[Source](https://example.com/a%28b%29)", "- [x] Done", "| Name | Value |", "| A | 1 |"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "secret-") {
		t.Fatalf("non-content HTML leaked: %q", got)
	}
}

func TestHTMLToMarkdownKeepsCodeAndDropsActiveLinks(t *testing.T) {
	got, err := HTMLToMarkdown("<p><a href='javascript:alert(1)'>label</a></p><pre>  a\n```\n b</pre>")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "javascript:") || !strings.Contains(got, "label") || !strings.Contains(got, "````\n  a\n```\n b\n````") {
		t.Fatalf("unsafe link or changed code: %q", got)
	}
}
