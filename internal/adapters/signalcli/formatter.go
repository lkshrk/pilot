package signalcli

import (
	"fmt"
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
	return fmt.Sprintf("Approve %s? %s", head, truncate(oneLine(desc), 140))
}

func progressText(taskID, phase string, progress int, detail string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s", taskID, phase)
	if progress > 0 {
		fmt.Fprintf(&b, " (%d%%)", progress)
	}
	if detail != "" {
		fmt.Fprintf(&b, "\n%s", oneLine(detail))
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
		fmt.Fprintf(&b, "\n\n%s", output)
	}
	return b.String()
}

// oneLine collapses newlines so a multi-line value cannot break a single-line
// context such as a poll question.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
