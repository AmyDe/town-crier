package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxSlackLines            = 10
	maxSlackDescriptionRunes = 150
	slackTestMessage         = "*Town Crier: test message*\nYour webhook works."
)

var errNoPayloads = errors.New("webhooks: slack message needs at least one payload")

var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// SlackBody builds the Slack incoming-webhook body {"text": ...} that
// summarises payloads, which are listed in the order given. At most 10 get a
// line and the rest are counted. A test payload renders the test message.
func SlackBody(payloads []Payload) ([]byte, error) {
	if len(payloads) == 0 {
		return nil, errNoPayloads
	}
	body, err := json.Marshal(struct {
		Text string `json:"text"`
	}{slackMessage(payloads)})
	if err != nil {
		return nil, fmt.Errorf("marshal slack body: %w", err)
	}
	return body, nil
}

func slackMessage(payloads []Payload) string {
	if payloads[0].Type == EventTest {
		return slackTestMessage
	}
	noun := "planning updates"
	if len(payloads) == 1 {
		noun = "planning update"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "*Town Crier: %d %s*", len(payloads), noun)
	for _, p := range payloads[:min(len(payloads), maxSlackLines)] {
		b.WriteString("\n")
		b.WriteString(slackLine(p))
	}
	if more := len(payloads) - maxSlackLines; more > 0 {
		fmt.Fprintf(&b, "\n…and %d more in the Town Crier app", more)
	}
	return b.String()
}

func slackLine(p Payload) string {
	var b strings.Builder
	b.WriteString("• ")
	switch {
	case p.Type == EventApplicationDecision:
		b.WriteString("Decision")
		if p.Application.AppState != nil && *p.Application.AppState != "" {
			fmt.Fprintf(&b, " (%s)", slackValue(*p.Application.AppState))
		}
	case p.WatchZone != nil:
		b.WriteString("New near " + slackValue(p.WatchZone.Name))
	default:
		b.WriteString("New")
	}
	b.WriteString(": ")

	ref := slackValue(p.Application.Reference)
	if p.Application.ShareURL != nil {
		fmt.Fprintf(&b, "<%s|%s>", slackValue(*p.Application.ShareURL), ref)
	} else {
		b.WriteString(ref)
	}
	if address := slackValue(p.Application.Address); address != "" {
		b.WriteString(", " + address)
	}
	if description := singleLine(p.Application.Description); description != "" {
		b.WriteString(". " + slackEscaper.Replace(truncateRunes(description, maxSlackDescriptionRunes)))
	}
	return b.String()
}

// slackValue makes an interpolated value safe for one line of mrkdwn.
func slackValue(s string) string {
	return slackEscaper.Replace(singleLine(s))
}

func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return string([]rune(s)[:maxRunes]) + "…"
}
