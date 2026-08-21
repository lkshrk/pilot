package signalcli

import (
	"strings"
	"testing"
)

func TestPlainText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "plain passes through", in: "all done, nothing to do", want: "all done, nothing to do"},
		{name: "bold stripped", in: "the **build** passed", want: "the build passed"},
		{name: "underscore bold stripped", in: "__urgent__ fix", want: "urgent fix"},
		{name: "italic stripped", in: "a *subtle* hint", want: "a subtle hint"},
		{name: "inline code stripped", in: "run `make build` first", want: "run make build first"},
		{name: "header stripped", in: "## Summary\ntext", want: "Summary\ntext"},
		{name: "link becomes label and url", in: "see [the PR](https://github.com/x/y/pull/1) now", want: "see the PR (https://github.com/x/y/pull/1) now"},
		{name: "image becomes label and url", in: "![diagram](https://x/y.png)", want: "diagram (https://x/y.png)"},
		{name: "bullets normalised", in: "* first\n- second\n+ third", want: "• first\n• second\n• third"},
		{name: "indented bullet keeps indent", in: "  - nested", want: "  • nested"},
		{name: "horizontal rule dropped", in: "above\n---\nbelow", want: "above\nbelow"},
		{name: "blockquote marker stripped", in: "> quoted line", want: "quoted line"},
		{
			name: "fence markers removed, code kept verbatim",
			in:   "before\n```go\nx := \"**not bold**\"\n```\nafter",
			want: "before\nx := \"**not bold**\"\nafter",
		},
		{
			name: "mixed answer",
			in:   "## Result\nThe `average()` helper is **done**:\n* tests pass\n* [PR #8](https://github.com/x/y/pull/8)",
			want: "Result\nThe average() helper is done:\n• tests pass\n• PR #8 (https://github.com/x/y/pull/8)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plainText(tt.in); got != tt.want {
				t.Errorf("plainText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestResultTextStripsMarkdownOutput(t *testing.T) {
	got := resultText("GH-1", true, "created **`average()`** in `math.js`", "https://github.com/x/y/pull/8")
	if strings.ContainsAny(got, "*`") {
		t.Errorf("result text still carries markdown sigils: %q", got)
	}
}

func TestConfirmationQuestionStripsMarkdown(t *testing.T) {
	got := confirmationQuestion("GH-2", "add a **rate limiter** to `api.go`", "pilot")
	if strings.ContainsAny(got, "*`") {
		t.Errorf("poll question still carries markdown sigils: %q", got)
	}
}
