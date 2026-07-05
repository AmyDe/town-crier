package notifydispatch

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
)

// PlatformDispatcher delivers a recipient bucket's pushes across both platforms:
// the iOS payload to the iOS tokens via APNs, the Android payload to the Android
// tokens via FCM. It holds both senders and returns the union of tokens either
// reported invalid so a caller loads devices once and prunes once — preserving
// the coalescer's load-devices-once / single-union-prune properties while adding
// the platform split.
//
// A per-platform send error is logged and skipped: one platform's failure must
// never block the other's delivery or the union prune. Both apns.PushSender and
// fcm.PushSender satisfy the (structurally identical) pushSender contract.
type PlatformDispatcher struct {
	apns   pushSender
	fcm    pushSender
	logger *slog.Logger
}

// NewPlatformDispatcher wires the dispatcher over an APNs and an FCM sender
// (either the real client or its NoOp twin).
func NewPlatformDispatcher(apnsSender, fcmSender pushSender, logger *slog.Logger) *PlatformDispatcher {
	return &PlatformDispatcher{apns: apnsSender, fcm: fcmSender, logger: logger}
}

// Send delivers iosPayload to iosTokens (via APNs) and androidPayload to
// androidTokens (via FCM), returning the union of tokens either sender reported
// invalid. A platform with no tokens or a nil payload is skipped, so a caller
// need only build the payload for a platform it actually has tokens for. Send
// always returns a nil error: per-platform faults are swallowed (logged) so the
// union prune still runs for the platform that succeeded.
func (d *PlatformDispatcher) Send(
	ctx context.Context,
	iosTokens []string, iosPayload json.RawMessage,
	androidTokens []string, androidPayload json.RawMessage,
) ([]string, error) {
	invalid := make([]string, 0)
	invalid = d.deliver(ctx, "apns", d.apns, iosTokens, iosPayload, invalid)
	invalid = d.deliver(ctx, "fcm", d.fcm, androidTokens, androidPayload, invalid)
	if len(invalid) == 0 {
		return nil, nil
	}
	return invalid, nil
}

// deliver sends one platform's payload to its tokens, appending any tokens the
// sender reports invalid to the running union. An empty token slice or nil
// payload is a no-op; a send error is logged and swallowed.
func (d *PlatformDispatcher) deliver(
	ctx context.Context,
	platformName string,
	sender pushSender,
	tokens []string,
	payload json.RawMessage,
	invalid []string,
) []string {
	if len(tokens) == 0 || payload == nil {
		return invalid
	}
	rejected, err := sender.Send(ctx, tokens, payload)
	if err != nil {
		d.logger.ErrorContext(ctx, "platform dispatch: send failed", "platform", platformName, "error", err)
		return invalid
	}
	return append(invalid, rejected...)
}

// groupTokensByPlatform splits a recipient's device registrations into iOS and
// Android token slices. An unrecognised platform is treated as iOS (the APNs
// default), matching devicetokens.DevicePlatform.String's fallback.
func groupTokensByPlatform(devices []devicetokens.DeviceRegistration) (ios, android []string) {
	for _, dev := range devices {
		if dev.Platform == devicetokens.PlatformAndroid {
			android = append(android, dev.Token)
			continue
		}
		ios = append(ios, dev.Token)
	}
	return ios, android
}
