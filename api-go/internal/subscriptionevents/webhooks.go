package subscriptionevents

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Webhook is one delivery target. Headers are sent verbatim; when BodyField is
// set the payload JSON is wrapped as {"<BodyField>": "<payload JSON>"}.
type Webhook struct {
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
	BodyField string            `json:"bodyField"`
}

// ParseWebhooks parses the SUBSCRIPTION_WEBHOOKS JSON array. An empty string or
// empty array yields no webhooks. Every URL must be https. Errors never echo
// the configured values, which are secrets.
func ParseWebhooks(raw string) ([]Webhook, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var hooks []Webhook
	if err := json.Unmarshal([]byte(raw), &hooks); err != nil {
		return nil, errors.New("subscription webhooks: invalid JSON array")
	}
	for i, h := range hooks {
		u, err := url.Parse(h.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return nil, fmt.Errorf("subscription webhooks: entry %d needs an https url", i)
		}
	}
	return hooks, nil
}
