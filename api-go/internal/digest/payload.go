package digest

import (
	"encoding/json"
	"fmt"
)

// apnsDigestPayload is the APNs body for a digest push: {"aps":{...}}. The JSON
// keys follow the APNs aps dictionary spec the iOS client expects.
type apnsDigestPayload struct {
	Aps apnsDigestAps `json:"aps"`
}

type apnsDigestAps struct {
	Alert apnsAlertContent `json:"alert"`
	Sound string           `json:"sound"`
	Badge int              `json:"badge"`
}

type apnsAlertContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// buildDigestPayload renders the APNs digest push body. applicationCount is the
// number of applications in the digest window (rendered in the body copy);
// totalUnreadCount is the user's total unread tally surfaced as the app icon
// badge — the two are deliberately distinct.
func buildDigestPayload(applicationCount, totalUnreadCount int) (json.RawMessage, error) {
	plural := "s"
	if applicationCount == 1 {
		plural = ""
	}
	body := fmt.Sprintf("%d new application%s this week", applicationCount, plural)

	raw, err := json.Marshal(apnsDigestPayload{
		Aps: apnsDigestAps{
			Alert: apnsAlertContent{Title: "Town Crier", Body: body},
			Sound: "default",
			Badge: totalUnreadCount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal digest payload: %w", err)
	}
	return raw, nil
}
