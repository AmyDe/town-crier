package digest

import (
	"encoding/json"
	"testing"
)

func TestBuildDigestPayload_Shape(t *testing.T) {
	t.Parallel()
	raw, err := buildDigestPayload(3, 7)
	if err != nil {
		t.Fatalf("buildDigestPayload: %v", err)
	}

	var parsed struct {
		Aps struct {
			Alert struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"alert"`
			Sound string `json:"sound"`
			Badge int    `json:"badge"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if parsed.Aps.Alert.Title != "Town Crier" {
		t.Errorf("title: got %q, want Town Crier", parsed.Aps.Alert.Title)
	}
	if parsed.Aps.Alert.Body != "3 new applications this week" {
		t.Errorf("body: got %q", parsed.Aps.Alert.Body)
	}
	if parsed.Aps.Sound != "default" {
		t.Errorf("sound: got %q, want default", parsed.Aps.Sound)
	}
	// The badge is the total unread count, distinct from the digest application
	// count.
	if parsed.Aps.Badge != 7 {
		t.Errorf("badge: got %d, want 7", parsed.Aps.Badge)
	}
}

func TestBuildDigestPayload_SingularBody(t *testing.T) {
	t.Parallel()
	raw, err := buildDigestPayload(1, 1)
	if err != nil {
		t.Fatalf("buildDigestPayload: %v", err)
	}
	var parsed struct {
		Aps struct {
			Alert struct {
				Body string `json:"body"`
			} `json:"alert"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if parsed.Aps.Alert.Body != "1 new application this week" {
		t.Errorf("singular body: got %q", parsed.Aps.Alert.Body)
	}
}
