package markdown

import (
	"github.com/exzco/bbs/internal/flarum"
	"strings"
	"testing"
)

func TestLegacyMarkdownRendering(t *testing.T) {
	source := flarum.DecodeContent(`<r><p><STRONG><s>**</s>加粗<e>**</e></STRONG></p></r>`)
	rendered, _ := Render(source, nil)
	if !strings.Contains(rendered, "<strong>加粗</strong>") {
		t.Fatalf("lost formatting: %s", rendered)
	}
	for _, source := range []string{`<script>alert(1)</script>`, `[x](javascript:alert%281%29)`, flarum.DecodeContent(`<t>&lt;img src=x onerror=alert(1)&gt;</t>`)} {
		rendered, _ := Render(source, nil)
		if strings.Contains(rendered, "<script") || strings.Contains(rendered, "javascript:") || strings.Contains(rendered, "<img src=x") {
			t.Fatalf("unsafe output: %s", rendered)
		}
	}
}
