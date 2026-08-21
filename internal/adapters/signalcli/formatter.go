package signalcli

import (
	"fmt"
	"regexp"
	"strings"
)

// Signal has no markdown renderer; with text_mode "styled" it understands
// *italic*, **bold**, `monospace`, ~strikethrough~ and ||spoiler|| markers.
// styledText translates the Telegram-flavored markdown the comms layer and
// model answers produce into that syntax; poll questions stay plain because
// polls carry no styles.

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
		fmt.Fprintf(&b, "\n%s", oneLine(styledText(detail)))
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
		fmt.Fprintf(&b, "\n\n%s", styledText(output))
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
	mdStrike     = regexp.MustCompile(`~~([^~]+)~~`)
)

// escapeStyleChars backslash-escapes the characters Signal's styled-text
// parser treats as markers, so code renders verbatim instead of toggling
// styles or losing characters.
func escapeStyleChars(s string) string {
	r := strings.NewReplacer("*", `\*`, "`", "\\`", "~", `\~`, "|", `\|`)
	return r.Replace(s)
}

// styledInline converts one line of markdown into Signal styled syntax.
// Inline code spans keep their backticks (Signal renders them monospace) but
// their content is escaped; outside code, bold and italic pass through as
// Signal understands the same markers, strikethrough narrows from ~~ to ~,
// and stray ~ and || are escaped so prose cannot toggle styles by accident.
func styledInline(line string) string {
	parts := mdInlineCode.Split(line, -1)
	codes := mdInlineCode.FindAllStringSubmatch(line, -1)
	var b strings.Builder
	for i, part := range parts {
		part = mdImage.ReplaceAllString(part, "$1 ($2)")
		part = mdLink.ReplaceAllString(part, "$1 ($2)")
		part = mdStrike.ReplaceAllString(part, "\x00$1\x00")
		part = mdBoldUnder.ReplaceAllString(part, "**$1**")
		part = strings.ReplaceAll(part, "~", `\~`)
		part = strings.ReplaceAll(part, "||", `\|\|`)
		part = strings.ReplaceAll(part, "\x00", "~")
		b.WriteString(part)
		if i < len(codes) {
			b.WriteString("`" + escapeStyleChars(codes[i][1]) + "`")
		}
	}
	return b.String()
}

// styledText renders markdown content in Signal's styled-text syntax: bold,
// italic and monospace survive as real styles, headers become bold lines,
// fenced code becomes a monospace block, links render as "label (url)" (Signal
// linkifies the raw URL), bullets become •, rules and quote markers go away.
func styledText(s string) string {
	if s == "" {
		return s
	}
	var out []string
	var fence []string
	inFence := false
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				out = append(out, "`"+strings.Join(fence, "\n")+"`")
				fence = nil
			}
			inFence = !inFence
			continue
		}
		if inFence {
			fence = append(fence, escapeStyleChars(line))
			continue
		}
		if mdRule.MatchString(line) {
			continue
		}
		if m := mdHeader.FindString(line); m != "" {
			out = append(out, "**"+styledInline(line[len(m):])+"**")
			continue
		}
		line = mdQuote.ReplaceAllString(line, "")
		line = mdBullet.ReplaceAllString(line, "$1• ")
		out = append(out, styledInline(line))
	}
	if inFence && len(fence) > 0 {
		out = append(out, "`"+strings.Join(fence, "\n")+"`")
	}
	return strings.Join(out, "\n")
}

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
