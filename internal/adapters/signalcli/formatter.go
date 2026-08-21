package signalcli

import (
	"fmt"
	"regexp"
	"strings"
)

// Signal renders message bodies as plain text: there is no HTML, no MarkdownV2,
// and no code fences. Anything Telegram would express with markup has to survive
// as readable prose here, so these helpers lean on line breaks and short labels
// rather than emphasis.

// confirmationQuestion is the poll question. Signal caps a poll question, and an
// over-long one is rejected outright, so the description is trimmed rather than
// risking a failed approval prompt.
func confirmationQuestion(taskID, desc, project string) string {
	head := taskID
	if project != "" {
		head = project + " " + taskID
	}
	if desc == "" {
		return "Approve " + head + "?"
	}
	return fmt.Sprintf("Approve %s? %s", head, truncate(oneLine(plainText(desc)), 140))
}

func progressText(taskID, phase string, progress int, detail string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s", taskID, phase)
	if progress > 0 {
		fmt.Fprintf(&b, " (%d%%)", progress)
	}
	if detail != "" {
		fmt.Fprintf(&b, "\n%s", oneLine(plainText(detail)))
	}
	return b.String()
}

func resultText(taskID string, success bool, output, prURL string) string {
	status := "failed"
	if success {
		status = "done"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s", taskID, status)
	if prURL != "" {
		fmt.Fprintf(&b, "\n%s", prURL)
	}
	if output != "" {
		fmt.Fprintf(&b, "\n\n%s", plainText(output))
	}
	return b.String()
}

// oneLine collapses newlines so a multi-line value cannot break a single-line
// context such as a poll question.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var (
	mdHeader     = regexp.MustCompile(`^#{1,6}\s+`)
	mdRule       = regexp.MustCompile(`^\s*(?:[-*_][ \t]*){3,}$`)
	mdBullet     = regexp.MustCompile(`^(\s*)[*+-]\s+`)
	mdQuote      = regexp.MustCompile(`^\s*>\s?`)
	mdBoldStars  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdBoldUnder  = regexp.MustCompile(`__([^_]+)__`)
	mdEmStars    = regexp.MustCompile(`\*([^*\s][^*]*)\*`)
	mdInlineCode = regexp.MustCompile("`([^`]*)`")
	mdImage      = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)[^)]*\)`)
	mdLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)[^)]*\)`)
)

// plainText renders markdown-formatted content as readable plain text. The
// comms layer and model answers compose messages in Telegram-flavored markdown;
// Signal renders bodies verbatim, so the sigils have to go before sending.
func plainText(s string) string {
	if s == "" {
		return s
	}
	var out []string
	inFence := false
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}
		if mdRule.MatchString(line) && !mdBullet.MatchString(line) {
			continue
		}
		line = mdHeader.ReplaceAllString(line, "")
		line = mdQuote.ReplaceAllString(line, "")
		line = mdBullet.ReplaceAllString(line, "$1• ")
		line = mdImage.ReplaceAllString(line, "$1 ($2)")
		line = mdLink.ReplaceAllString(line, "$1 ($2)")
		line = mdBoldStars.ReplaceAllString(line, "$1")
		line = mdBoldUnder.ReplaceAllString(line, "$1")
		line = mdEmStars.ReplaceAllString(line, "$1")
		line = mdInlineCode.ReplaceAllString(line, "$1")
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
