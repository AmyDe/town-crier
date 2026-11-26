package webhooks

import (
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/acsemail"
)

const (
	emailKindDisabled = "webhook-disabled"
	senderAddress     = "hello@towncrierapp.uk"
	settingsURL       = "https://towncrierapp.uk/settings"

	disabledSubject       = "Your Town Crier webhook is switched off"
	disabledFailuresIntro = "We couldn't deliver alerts to your webhook for a day, so we've switched it off."
	disabledGoneIntro     = "Your webhook told us it no longer exists, so we've switched it off."
	disabledBodyHTML      = "<p>%s Your alerts still arrive in the app as normal. " +
		`To switch it back on, check the address and save it again in Settings: <a href="%[2]s">%[2]s</a></p>`
)

// disabledEmail tells the owner of an endpoint that it was switched off. The
// message names neither the webhook URL nor its host.
func disabledEmail(recipient, endpointID string, reason DisabledReason, at time.Time) acsemail.Message {
	intro := disabledFailuresIntro
	if reason == DisabledGone {
		intro = disabledGoneIntro
	}
	return acsemail.Message{
		Sender:           senderAddress,
		Recipient:        recipient,
		Subject:          disabledSubject,
		HTMLBody:         fmt.Sprintf(disabledBodyHTML, intro, settingsURL),
		RepeatabilityKey: emailKindDisabled + "|" + endpointID + "|" + formatInstant(at),
	}
}
