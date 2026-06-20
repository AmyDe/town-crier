package digest

import (
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/vocabulary"
)

// senderAddress is the verified ACS sender address all digest emails are sent from.
const senderAddress = "hello@towncrierapp.uk"

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

// buildDigestHTML renders the full digest email body: a header, one block per
// watch zone (each with its application cards), an optional Saved Applications
// section, a CTA button, and a footer. All user-supplied content is
// HTML-encoded.
func buildDigestHTML(zoneSections []watchZoneDigest, savedApplications []notifications.DigestNotification, totalCount int) string {
	var zoneBlocks strings.Builder
	for _, section := range zoneSections {
		var cards strings.Builder
		for _, n := range section.notifications {
			cards.WriteString(buildNotificationCard(n))
		}
		fmt.Fprintf(&zoneBlocks,
			`<tr><td style="padding:16px 0 8px 0;font-size:14px;color:#666;text-transform:uppercase;letter-spacing:0.5px;">
  📍 %s
</td></tr>
%s`, htmlEncode(section.name), cards.String())
	}

	if len(savedApplications) > 0 {
		var savedCards strings.Builder
		for _, n := range savedApplications {
			savedCards.WriteString(buildNotificationCard(n))
		}
		fmt.Fprintf(&zoneBlocks,
			`<tr><td style="padding:16px 0 8px 0;font-size:14px;color:#666;text-transform:uppercase;letter-spacing:0.5px;">
  ★ Saved Applications
</td></tr>
%s`, savedCards.String())
	}

	plural := "s"
	if totalCount == 1 {
		plural = ""
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"></head>
<body style="margin:0;padding:0;background:#f0f0f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
<table width="100%%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px;">
<table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;">
  <tr><td style="background:#1a1a2e;padding:24px;text-align:center;">
    <div style="font-size:20px;font-weight:700;color:#ffffff;">Town Crier</div>
    <div style="color:#888;font-size:13px;margin-top:4px;">Live Planning Update</div>
  </td></tr>
  <tr><td style="padding:24px;">
    <table width="100%%" cellpadding="0" cellspacing="0">
      %s
    </table>
    <table width="100%%" cellpadding="0" cellspacing="0" style="margin-top:24px;">
      <tr><td align="center">
        <a href="https://towncrierapp.uk/applications" style="display:inline-block;background:#4a6cf7;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;">View All in App</a>
      </td></tr>
    </table>
  </td></tr>
  <tr><td style="padding:16px 24px;text-align:center;color:#999;font-size:12px;border-top:1px solid #eee;">
    %d new application%s · <a href="https://towncrierapp.uk/settings" style="color:#999;">Unsubscribe</a>
  </td></tr>
</table>
</td></tr></table>
</body></html>`, zoneBlocks.String(), totalCount, plural)
}

// buildNotificationCard renders one application card for the digest body. A
// decision update prepends the UK display-label badge; a zone notification that
// is also saved appends a "★ saved" indicator. Each card line links to the
// application detail page so iOS Universal Links open the app.
func buildNotificationCard(n notifications.DigestNotification) string {
	addressLine := htmlEncode(n.ApplicationAddress)
	if n.EventType == notifications.EventDecisionUpdate {
		if label := vocabulary.UKDisplayString(n.Decision); label != "" {
			addressLine = fmt.Sprintf(
				`<span style="display:inline-block;background:#eef1ff;color:#1a1a2e;font-size:11px;font-weight:700;letter-spacing:0.5px;padding:2px 6px;border-radius:4px;margin-right:6px;">[%s]</span>%s`,
				htmlEncode(label), addressLine)
		}
	}

	savedIndicator := ""
	if n.WatchZoneID != nil && n.HasSavedSource() {
		savedIndicator = `<span data-saved-indicator style="display:inline-block;background:#fff3cd;color:#664d03;font-size:11px;font-weight:600;letter-spacing:0.3px;padding:2px 6px;border-radius:4px;margin-left:6px;">★ saved</span>`
	}

	appURL := buildApplicationDetailURL(n.ApplicationUID)
	openLink := fmt.Sprintf(`<a href="%s" style="text-decoration:none;color:inherit;">`, appURL)
	const closeLink = "</a>"

	appType := "Planning Application"
	if n.ApplicationType != nil && *n.ApplicationType != "" {
		appType = *n.ApplicationType
	}

	return fmt.Sprintf(`<tr><td style="padding:0 0 8px 0;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="background:#f8f9fa;border-radius:6px;">
    <tr><td style="padding:12px;">
      %s<div style="font-weight:600;color:#1a1a2e;">%s%s</div>%s
      %s<div style="color:#4a6cf7;font-size:13px;">%s</div>%s
      %s<div style="color:#666;font-size:13px;margin-top:4px;">%s</div>%s
    </td></tr>
  </table>
</td></tr>`,
		openLink, addressLine, savedIndicator, closeLink,
		openLink, htmlEncode(appType), closeLink,
		openLink, htmlEncode(truncate(n.ApplicationDescription, 120)), closeLink)
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
