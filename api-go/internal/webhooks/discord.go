package webhooks

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf16"
)

const (
	maxDiscordLines            = 10
	maxDiscordDescriptionRunes = 150
	// Discord counts the limit as a JavaScript string length, so in UTF-16 code
	// units: a character outside the Basic Multilingual Plane counts twice.
	maxDiscordContentUnits = 2000
	discordSuppressEmbeds  = 4
	discordTestMessage     = "**Town Crier: test message**\nYour webhook works."
)

var discordEscaper = strings.NewReplacer(
	`\`, `\\`, `*`, `\*`, `_`, `\_`, `~`, `\~`, "`", "\\`", `|`, `\|`,
	`[`, `\[`, `]`, `\]`, `(`, `\(`, `)`, `\)`, `<`, `\<`, `>`, `\>`,
)

type discordAllowedMentions struct {
	Parse []string `json:"parse"`
}

// DiscordBody builds the Discord execute-webhook body that summarises payloads,
// which are listed in the order given. The content is at most 2,000 UTF-16 code
// units: at most 10 payloads that fit get a line and the rest are counted. The
// message resolves no mentions and shows no link previews. A test payload
// renders the test message.
func DiscordBody(payloads []Payload) ([]byte, error) {
	if len(payloads) == 0 {
		return nil, errNoPayloads
	}
	body, err := json.Marshal(struct {
		Content         string                 `json:"content"`
		AllowedMentions discordAllowedMentions `json:"allowed_mentions"`
		Flags           int                    `json:"flags"`
	}{
		Content:         discordMessage(payloads),
		AllowedMentions: discordAllowedMentions{Parse: []string{}},
		Flags:           discordSuppressEmbeds,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal discord body: %w", err)
	}
	return body, nil
}

func discordMessage(payloads []Payload) string {
	if payloads[0].Type == EventTest {
		return discordTestMessage
	}
	noun := "planning updates"
	if len(payloads) == 1 {
		noun = "planning update"
	}
	content := fmt.Sprintf("**Town Crier: %d %s**", len(payloads), noun)
	listed := 0
	for _, p := range payloads[:min(len(payloads), maxDiscordLines)] {
		next := content + "\n" + discordLine(p)
		if utf16Units(next+moreSuffix(len(payloads)-listed-1)) > maxDiscordContentUnits {
			break
		}
		content = next
		listed++
	}
	return content + moreSuffix(len(payloads)-listed)
}

func moreSuffix(more int) string {
	if more == 0 {
		return ""
	}
	return fmt.Sprintf("\n…and %d more in the Town Crier app", more)
}

func utf16Units(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

func discordLine(p Payload) string {
	var b strings.Builder
	b.WriteString("• ")
	switch {
	case p.Type == EventApplicationDecision:
		b.WriteString("Decision")
		if p.Application.AppState != nil && *p.Application.AppState != "" {
			fmt.Fprintf(&b, " (%s)", discordValue(*p.Application.AppState))
		}
	case p.WatchZone != nil:
		b.WriteString("New near " + discordValue(p.WatchZone.Name))
	default:
		b.WriteString("New")
	}
	b.WriteString(": ")

	ref := discordValue(p.Application.Reference)
	if p.Application.ShareURL != nil {
		fmt.Fprintf(&b, "[%s](%s)", ref, discordLinkURL(*p.Application.ShareURL))
	} else {
		b.WriteString(ref)
	}
	if address := discordValue(p.Application.Address); address != "" {
		b.WriteString(", " + address)
	}
	if description := singleLine(p.Application.Description); description != "" {
		b.WriteString(". " + discordEscaper.Replace(truncateRunes(description, maxDiscordDescriptionRunes)))
	}
	return b.String()
}

// discordValue makes an interpolated value safe for one line of Discord markdown.
func discordValue(s string) string {
	return discordEscaper.Replace(singleLine(s))
}

// discordLinkURL percent-encodes the characters that can end the target of a
// masked link early.
func discordLinkURL(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune("()<>", r) || unicode.IsSpace(r) {
			// QueryEscape writes a space as "+", which is literal in a URL path.
			b.WriteString(strings.ReplaceAll(url.QueryEscape(string(r)), "+", "%20"))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
