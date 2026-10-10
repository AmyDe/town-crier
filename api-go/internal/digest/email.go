package digest

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/designtokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/vocabulary"
)

// senderAddress is the verified ACS sender address all digest emails are sent from.
const senderAddress = "hello@towncrierapp.uk"

// Email-safe font stacks: email clients cannot fetch the brand's self-hosted
// webfonts.
const (
	headlineFontStack = bodyFontStack
	bodyFontStack     = "-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif"
	monoFontStack     = "'Courier New', monospace"
)

// pageTemplate is the outer page shell. It is light-only by design: email-client
// dark-mode colour inversion is unreliable.
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
      %s
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

// freeTierNoticeText is a factual account-status line for Free-tier weekly
// recipients, not marketing: the digest is a service message, so it must never
// become a link or point at a paywall, plans or pricing page.
const freeTierNoticeText = "You're on the free weekly digest."

// freeTierNoticeTemplate renders the free-tier notice as its own row directly
// above the CTA button, inside the same centered table: small, muted,
// secondary-coloured text, centre-aligned like the button below it.
const freeTierNoticeTemplate = `<tr><td align="center" data-testid="digest-free-tier-notice" style="padding-bottom:12px;font-size:12px;color:%s;">%s</td></tr>`

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

// decisionChipTemplate and savedIndicatorTemplate are the two outlined "stamp"
// pills.
const decisionChipTemplate = `<span style="display:inline-block;background:transparent;border:1px solid %s;color:%s;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:0.08em;padding:2px 6px;border-radius:4px;margin-right:6px;">[%s]</span>`

const savedIndicatorTemplate = `<span data-saved-indicator style="display:inline-block;background:transparent;border:1px solid %s;color:%s;font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:0.08em;padding:2px 6px;border-radius:4px;margin-left:6px;">★ saved</span>`

// watchZoneDigest groups a watch zone's display name with the notifications that
// fell inside it.
type watchZoneDigest struct {
	name          string
	notifications []notifications.DigestNotification
}

// buildDigestSubject renders the digest email subject line: the count leads,
// with no "Planning update:" boilerplate (the sender is already "Town
// Crier", so the prefix is dead text in an inbox preview), followed by the
// zone(s) the updates fall in. A single zone with no saved-only
// notifications names it directly. Saved-only notifications with no zone at
// all (WatchZoneID == nil) get their own fallback, since "near {zone}"
// doesn't apply to them. Anything spanning more than one group — multiple
// zones, or a zone plus a saved-only group — leads with the first zone name
// and folds the remaining groups into "+K more" rather than trying to list
// them all and risk truncation in a notification/inbox preview.
func buildDigestSubject(totalCount int, sections []watchZoneDigest, saved []notifications.DigestNotification) string {
	plural := "s"
	if totalCount == 1 {
		plural = ""
	}

	groups := len(sections)
	if len(saved) > 0 {
		groups++
	}

	switch {
	case groups == 0:
		return fmt.Sprintf("%d update%s near your zones", totalCount, plural)
	case len(sections) == 1 && len(saved) == 0:
		return fmt.Sprintf("%d update%s near %s", totalCount, plural, sections[0].name)
	case len(sections) == 0:
		return fmt.Sprintf("%d update%s on your saved applications", totalCount, plural)
	default:
		more := groups - 1
		return fmt.Sprintf("%d update%s near %s +%d more", totalCount, plural, sections[0].name, more)
	}
}

// buildDigestHTML renders the digest email body. All user-supplied content is
// HTML-encoded, and the markup stays table-based with inline styles because
// email clients support nothing else reliably.
func buildDigestHTML(zoneSections []watchZoneDigest, savedApplications []notifications.DigestNotification, totalCount int, showFreeTierNotice bool) string {
	var zoneBlocks strings.Builder
	for _, section := range zoneSections {
		var cards strings.Builder
		for _, n := range section.notifications {
			cards.WriteString(buildNotificationCard(n))
		}
		fmt.Fprintf(&zoneBlocks, sectionHeaderTemplate, designtokens.TextSecondaryLightHex, "📍", html.EscapeString(section.name))
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

	freeTierNotice := ""
	if showFreeTierNotice {
		freeTierNotice = fmt.Sprintf(freeTierNoticeTemplate, designtokens.TextSecondaryLightHex, html.EscapeString(freeTierNoticeText))
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
		freeTierNotice,
		designtokens.AmberLightHex, designtokens.TextOnAccentLightHex,
		designtokens.TextSecondaryLightHex, designtokens.BorderLightHex,
		totalCount, plural,
		designtokens.TextSecondaryLightHex,
	)
}

// decisionChipHex maps a vocabulary.UKDisplayString label to its status ink
// colour, falling back to the primary text colour for an unrecognised label.
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
// mono reference/date strip, a sans headline (the address, optionally
// prefixed by the outlined decision-label stamp and suffixed by the outlined
// "saved" stamp), the application type, and a truncated description. Each
// line links to the application detail page so iOS Universal Links open the
// app.
func buildNotificationCard(n notifications.DigestNotification) string {
	addressLine := html.EscapeString(n.ApplicationAddress)
	if n.EventType == notifications.EventDecisionUpdate {
		if label := vocabulary.UKDisplayString(n.Decision); label != "" {
			hex := decisionChipHex(label)
			addressLine = fmt.Sprintf(decisionChipTemplate, hex, hex, html.EscapeString(label)) + addressLine
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
		monoFontStack, designtokens.TextSecondaryLightHex, html.EscapeString(n.ApplicationUID),
		monoFontStack, designtokens.TextSecondaryLightHex, n.CreatedAt.Format("2 Jan 2006"))

	headline := fmt.Sprintf(`%s<div style="font-family:%s;font-weight:700;color:%s;font-size:15px;">%s%s</div>%s`,
		openLink, headlineFontStack, designtokens.TextPrimaryLightHex, addressLine, savedIndicator, closeLink)

	typeLine := fmt.Sprintf(`%s<div style="color:%s;font-size:13px;margin-top:4px;">%s</div>%s`,
		openLink, designtokens.TextSecondaryLightHex, html.EscapeString(appType), closeLink)

	descriptionLine := fmt.Sprintf(`%s<div style="color:%s;font-size:13px;margin-top:4px;">%s</div>%s`,
		openLink, designtokens.TextSecondaryLightHex, html.EscapeString(truncate(n.ApplicationDescription, 120)), closeLink)

	return fmt.Sprintf(cardTemplate, designtokens.BackgroundLightHex, docHeader, headline, typeLine, descriptionLine)
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

// truncate caps text at maxLength, replacing the tail with an ellipsis when it
// overflows.
func truncate(text string, maxLength int) string {
	runes := []rune(text)
	if len(runes) <= maxLength {
		return text
	}
	return string(runes[:maxLength-1]) + "…"
}
