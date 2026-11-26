package webhooks

import (
	"strings"
	"testing"
)

func TestDisabledEmail(t *testing.T) {
	t.Parallel()
	const rest = " Your alerts still arrive in the app as normal. To switch it back on, check the address and save it again in Settings: " +
		`<a href="https://towncrierapp.uk/settings">https://towncrierapp.uk/settings</a>`
	for _, tc := range []struct {
		reason DisabledReason
		first  string
	}{
		{DisabledFailures, "We couldn&#39;t deliver alerts to your webhook for a day, so we&#39;ve switched it off."},
		{DisabledGone, "Your webhook told us it no longer exists, so we&#39;ve switched it off."},
	} {
		t.Run(string(tc.reason), func(t *testing.T) {
			t.Parallel()
			got := disabledEmail("owner@example.org", "endpoint-1", tc.reason, drainNow)

			if got.Sender != "hello@towncrierapp.uk" || got.Recipient != "owner@example.org" ||
				got.Subject != "Your Town Crier webhook is switched off" {
				t.Errorf("envelope = %+v", got)
			}
			if !strings.Contains(got.HTMLBody, tc.first+rest) {
				t.Errorf("body = %s", got.HTMLBody)
			}
			if strings.ContainsAny(got.Subject+got.HTMLBody, "—–") {
				t.Error("copy carries a dash")
			}
			if got.RepeatabilityKey != "webhook-disabled|endpoint-1|2026-11-26T15:00:00Z" {
				t.Errorf("repeatability key = %q", got.RepeatabilityKey)
			}
		})
	}
}
