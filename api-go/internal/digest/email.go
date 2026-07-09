package digest

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/designtokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/vocabulary"
)

// senderAddress is the verified ACS sender address all digest emails are sent from.
const senderAddress = "hello@towncrierapp.uk"

// Email-safe font stacks (Public Notice, issue 859). Email clients cannot
// fetch the self-hosted webfonts the rest of the brand uses (Fraunces,
// Inter), so the digest renders the type system's email-safe fallbacks
// directly rather than pointing at a font the client will never load:
// Georgia for headlines (the same fallback `--tc-font-display` names before
// Fraunces on web), the existing system-sans stack for body copy, and
// Courier New for the mono reference/date strip.
const (
	headlineFontStack = "Georgia, 'Times New Roman', serif"
	bodyFontStack     = "-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif"
	monoFontStack     = "'Courier New', monospace"
)

// pageTemplate is the outer Public Notice shell: a paper-toned page holding a
// single 600px card, a lettered masthead over a double rule, the per-zone
// notification blocks, an amber CTA, and a footer with the unsubscribe link.
// It is light-only by design (the color-scheme/supported-color-schemes meta
// pair): email client dark-mode colour inversion is unreliable enough that a
// dark variant is out of scope for v1 (issue 859).
const pageTemplate = `<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="only light">
</head>
<body style="margin:0;padding:0;background:%s;font-family:%s;">
<table width="100%%" cellpadding="0" cellspacing="0" role="presentation"><tr><td align="center" style="padding:24px;">
<table width="600" cellpadding="0" cellspacing="0" role="presentation" style="background:%s;border:1px solid %s;border-top:2px solid %s;">
  <tr><td style="background:%s;padding:24px;text-align:center;">
    <div style="font-size:20px;font-weight:700;color:%s;font-variant:small-caps;letter-spacing:0.06em;">Town Crier</div>
    <div style="color:%s;font-size:12px;margin-top:6px;text-transform:uppercase;letter-spacing:0.08em;">Live Planning Update</div>
  </td></tr>
  <tr><td data-testid="digest-masthead-rule-heavy" style="padding:0;height:3px;line-height:3px;font-size:0;background:%s;">&nbsp;</td></tr>
  <tr><td data-testid="digest-masthead-rule-hairline" style="padding:0;height:1px;line-height:1px;font-size:0;background:%s;">&nbsp;</td></tr>
  <tr><td style="padding:24px;">
    <table width="100%%" cellpadding="0" cellspacing="0" role="presentation">
      %s
    </table>
    <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="margin-top:24px;">
      <tr><td align="center">
        <a href="https://towncrierapp.uk/applications" style="display:inline-block;background:%s;color:%s;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;">Open Town Crier</a>
      </td></tr>
    </table>
  </td></tr>
  <tr><td style="padding:16px 24px;text-align:center;color:%s;font-size:12px;border-top:1px solid %s;">
    %d new application%s · <a href="https://towncrierapp.uk/settings" style="color:%s;">Unsubscribe</a>
  </td></tr>
</table>
</td></tr></table>
</body></html>`

// sectionHeaderTemplate renders a zone or "Saved Applications" section label.
const sectionHeaderTemplate = `<tr><td style="padding:16px 0 8px 0;font-size:14px;color:%s;text-transform:uppercase;letter-spacing:0.08em;">
  %s %s
</td></tr>
`

// cardTemplate is one "filed notice" card: a mono doc-header strip, then the
// headline/type/description lines, each still wrapped in the detail-page
// link. The per-card background sits one shade deeper than the outer card
// (paper, not surface), giving each entry definition against the card body.
const cardTemplate = `<tr><td style="padding:0 0 8px 0;">
  <table width="100%%" cellpadding="0" cellspacing="0" role="presentation" style="background:%s;border-radius:6px;">
    %s
    <tr><td style="padding:12px;">
      %s
      %s
      %s
    </td></tr>
  </table>
</td></tr>`

// docHeaderTemplate is the mono reference/date strip above each card's
// headline, mirroring the docHeader row on ApplicationCard.tsx (reference
// left, date right, both mono, a hairline rule beneath).
const docHeaderTemplate = `<tr><td style="padding:0 0 6px 0;border-bottom:1px solid %s;">
  <table width="100%%" cellpadding="0" cellspacing="0" role="presentation"><tr>
    <td align="left" data-testid="digest-notification-reference" style="font-family:%s;font-size:11px;color:%s;">%s</td>
    <td align="right" data-testid="digest-notification-date" style="font-family:%s;font-size:11px;color:%s;">%s</td>
  </tr></table>
</td></tr>`

// decisionChipTemplate and savedIndicatorTemplate are the two "stamp" pills:
// a solid 1px border in the relevant ink colour, uppercase, letterspaced, and
// a transparent background (replacing the pre-brand filled chip/badge).
const decisionChipTemplate = `<span style="display:inline-block;background:transparent;border:1px solid %s;color:%s;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:0.08em;padding:2px 6px;border-radius:4px;margin-right:6px;">[%s]</span>`

const savedIndicatorTemplate = `<span data-saved-indicator style="display:inline-block;background:transparent;border:1px solid %s;color:%s;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:0.08em;padding:2px 6px;border-radius:4px;margin-left:6px;">★ saved</span>`

// watchZoneDigest groups a watch zone's display name with the notifications that
// fell inside it.
type watchZoneDigest struct {
	name          string
	notifications []notifications.DigestNotification
}

// buildDigestSubject renders the digest email subject line.
func buildDigestSubject(totalCount int) string {
	return fmt.Sprintf("Planning update — %d new applications near you", totalCount)
}

// buildDigestHTML renders the full digest email body as a Public Notice
// masthead-and-card layout (issue 859): a paper-toned page, a lettered
// masthead over a double rule, one filed-notice card per watch zone (plus an
// optional Saved Applications section), an amber CTA, and a footer carrying
// the count and the unsubscribe link. All user-supplied content is
// HTML-encoded. The markup stays table-based with inline styles throughout —
// email-client reality, no external stylesheet, no flexbox/grid.
func buildDigestHTML(zoneSections []watchZoneDigest, savedApplications []notifications.DigestNotification, totalCount int) string {
	var zoneBlocks strings.Builder
	for _, section := range zoneSections {
		var cards strings.Builder
		for _, n := range section.notifications {
			cards.WriteString(buildNotificationCard(n))
		}
		fmt.Fprintf(&zoneBlocks, sectionHeaderTemplate, designtokens.TextSecondaryLightHex, "📍", htmlEncode(section.name))
		zoneBlocks.WriteString(cards.String())
	}

	if len(savedApplications) > 0 {
		var savedCards strings.Builder
		for _, n := range savedApplications {
			savedCards.WriteString(buildNotificationCard(n))
		}
		fmt.Fprintf(&zoneBlocks, sectionHeaderTemplate, designtokens.TextSecondaryLightHex, "★", "Saved Applications")
		zoneBlocks.WriteString(savedCards.String())
	}

	plural := "s"
	if totalCount == 1 {
		plural = ""
	}

	return fmt.Sprintf(pageTemplate,
		designtokens.BackgroundLightHex, bodyFontStack,
		designtokens.SurfaceLightHex, designtokens.BorderLightHex, designtokens.TextPrimaryLightHex,
		designtokens.BackgroundLightHex,
		designtokens.TextPrimaryLightHex,
		designtokens.TextSecondaryLightHex,
		designtokens.TextPrimaryLightHex,
		designtokens.BorderLightHex,
		zoneBlocks.String(),
		designtokens.AmberLightHex, designtokens.TextOnAccentLightHex,
		designtokens.TextSecondaryLightHex, designtokens.BorderLightHex,
		totalCount, plural,
		designtokens.TextSecondaryLightHex,
	)
}

// decisionChipHex maps a UK-facing decision label (vocabulary.UKDisplayString's
// output) to the status ink colour its outlined stamp renders in, the same
// status-colour buckets the iOS/Android/web cards use for the same four
// decision states. An unrecognised label (not currently possible — the four
// vocabulary outputs are exhaustive) falls back to the primary text colour
// rather than an unstyled chip.
func decisionChipHex(label string) string {
	switch label {
	case "Approved":
		return designtokens.StatusPermittedLightHex
	case "Approved with conditions":
		return designtokens.StatusConditionsLightHex
	case "Refused":
		return designtokens.StatusRejectedLightHex
	case "Refusal appealed":
		return designtokens.StatusAppealedLightHex
	default:
		return designtokens.TextPrimaryLightHex
	}
}

// buildNotificationCard renders one application card for the digest body: a
// mono reference/date strip, a serif headline (the address, optionally
// prefixed by the outlined decision-label stamp and suffixed by the outlined
// "saved" stamp), the application type, and a truncated description. Each
// line links to the application detail page so iOS Universal Links open the
// app.
func buildNotificationCard(n notifications.DigestNotification) string {
	addressLine := htmlEncode(n.ApplicationAddress)
	if n.EventType == notifications.EventDecisionUpdate {
		if label := vocabulary.UKDisplayString(n.Decision); label != "" {
			hex := decisionChipHex(label)
			addressLine = fmt.Sprintf(decisionChipTemplate, hex, hex, htmlEncode(label)) + addressLine
		}
	}

	savedIndicator := ""
	if n.WatchZoneID != nil && n.HasSavedSource() {
		savedIndicator = fmt.Sprintf(savedIndicatorTemplate, designtokens.AmberLightHex, designtokens.AmberLightHex)
	}

	appURL := buildApplicationDetailURL(n.ApplicationUID)
	openLink := fmt.Sprintf(`<a href="%s" style="text-decoration:none;color:inherit;">`, appURL)
	const closeLink = "</a>"

	appType := "Planning Application"
	if n.ApplicationType != nil && *n.ApplicationType != "" {
		appType = *n.ApplicationType
	}

	docHeader := fmt.Sprintf(docHeaderTemplate,
		designtokens.BorderLightHex,
		monoFontStack, designtokens.TextSecondaryLightHex, htmlEncode(n.ApplicationUID),
		monoFontStack, designtokens.TextSecondaryLightHex, formatNotificationDate(n.CreatedAt))

	headline := fmt.Sprintf(`%s<div style="font-family:%s;font-weight:700;color:%s;font-size:15px;">%s%s</div>%s`,
		openLink, headlineFontStack, designtokens.TextPrimaryLightHex, addressLine, savedIndicator, closeLink)

	typeLine := fmt.Sprintf(`%s<div style="color:%s;font-size:13px;margin-top:4px;">%s</div>%s`,
		openLink, designtokens.TextSecondaryLightHex, htmlEncode(appType), closeLink)

	descriptionLine := fmt.Sprintf(`%s<div style="color:%s;font-size:13px;margin-top:4px;">%s</div>%s`,
		openLink, designtokens.TextSecondaryLightHex, htmlEncode(truncate(n.ApplicationDescription, 120)), closeLink)

	return fmt.Sprintf(cardTemplate, designtokens.BackgroundLightHex, docHeader, headline, typeLine, descriptionLine)
}

// formatNotificationDate renders a notification's CreatedAt as the compact
// day/short-month/year the Public Notice mono metadata strip uses elsewhere
// (mirrors ApplicationCard.tsx's formatDate on web).
func formatNotificationDate(t time.Time) string {
	return t.Format("2 Jan 2006")
}

// buildApplicationDetailURL builds the application detail URL, keeping the
// slashes in a PlanIt uid (e.g. "19/00123/FUL") as path separators while
// percent-encoding every other reserved character per segment.
func buildApplicationDetailURL(applicationUID string) string {
	segments := strings.Split(applicationUID, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return "https://towncrierapp.uk/applications/" + strings.Join(segments, "/")
}

// htmlEncode HTML-encodes user-supplied text for safe inclusion in the email
// body.
func htmlEncode(text string) string {
	return html.EscapeString(text)
}

// truncate caps text at maxLength, replacing the tail with an ellipsis when it
// overflows.
func truncate(text string, maxLength int) string {
	if len([]rune(text)) <= maxLength {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxLength-1]) + "…"
}
