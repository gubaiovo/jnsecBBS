// Package markdown 提供 Markdown → 安全 HTML 的渲染（goldmark + bluemonday），
// 以及 @提及 的解析与链接化。
package markdown

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		extension.Linkify,
	),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

// policy 与 Flarum 一致的白名单：允许常规排版、代码块、图片、链接；
// 一律剥离 script/iframe/事件属性/危险协议。
var policy = newPolicy()

func newPolicy() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("start", "type").OnElements("ol")
	p.AllowElements("details", "summary")
	p.AllowAttrs("class").OnElements("code") // 保留语言标注
	p.RequireNoFollowOnLinks(true)
	return p
}

var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9_\-]{2,20})`)

// Render 渲染 Markdown 并消毒。resolve 存在时把内容里的 @用户名 链接到其主页
// （代码围栏内不处理），并返回实际命中的用户名列表（去重）。
func Render(source string, resolve func(name string) (uint, bool)) (html string, mentions []string) {
	mentioned := map[string]struct{}{}
	if resolve != nil {
		source = linkMentions(source, resolve, mentioned)
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "<p>" + escapeHTML(source) + "</p>", nil
	}
	names := make([]string, 0, len(mentioned))
	for n := range mentioned {
		names = append(names, n)
	}
	return policy.Sanitize(buf.String()), names
}

// MentionedUsernames 只提取文本中存在的用户名，不做替换。
func MentionedUsernames(source string, resolve func(name string) (uint, bool)) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, m := range mentionRe.FindAllStringSubmatch(source, -1) {
		name := m[1]
		if _, dup := seen[name]; dup {
			continue
		}
		if _, ok := resolve(name); ok {
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	return out
}

// linkMentions 逐行替换 @name；``` 围栏内的行跳过。
func linkMentions(source string, resolve func(name string) (uint, bool), mentioned map[string]struct{}) string {
	lines := strings.Split(source, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = mentionRe.ReplaceAllStringFunc(line, func(m string) string {
			name := m[1:]
			if _, ok := resolve(name); !ok {
				return m
			}
			mentioned[name] = struct{}{}
			return "[@" + name + "](/u/" + name + ")"
		})
	}
	return strings.Join(lines, "\n")
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}
