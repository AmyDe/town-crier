package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"
)

const (
	userAgent = "TownCrier-Webhooks/1.0 (+https://towncrierapp.uk/developers/webhooks)"

	headerEventID   = "X-TownCrier-Event-Id"
	headerEventType = "X-TownCrier-Event-Type"
	headerTimestamp = "X-TownCrier-Timestamp"
	headerSignature = "X-TownCrier-Signature"
)

// Sign returns the X-TownCrier-Signature value: "v1=" followed by the lower
// case hex HMAC-SHA256 of "<timestamp>.<body>" keyed with the whole secret
// string. timestamp is in Unix seconds.
func Sign(secret string, timestamp int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(strconv.AppendInt(nil, timestamp, 10))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

// newRequest builds the signed POST for msg. Its error can quote the URL, so
// it must not be logged or returned to a caller.
func newRequest(ctx context.Context, msg Message, now time.Time) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msg.URL, bytes.NewReader(msg.Body))
	if err != nil {
		return nil, err
	}
	timestamp := now.Unix()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(headerEventID, msg.EventID)
	req.Header.Set(headerEventType, string(msg.EventType))
	req.Header.Set(headerTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(headerSignature, Sign(msg.Secret, timestamp, msg.Body))
	return req, nil
}
