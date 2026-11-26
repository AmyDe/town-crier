package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func slackText(t *testing.T, payloads ...Payload) string {
	t.Helper()
	raw, err := SlackBody(payloads)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 {
		t.Errorf("body keys = %v, want only text", body)
	}
	return body["text"]
}

func slackAlert(typ EventType, zone, ref, address, description string, appState, shareURL *string) Payload {
	var z *PayloadZone
	if zone != "" {
		z = &PayloadZone{ID: "z", Name: zone}
	}
	return Payload{
		Type:      typ,
		WatchZone: z,
		Application: PayloadApplication{
			Reference:   ref,
			Address:     address,
			Description: description,
			AppState:    appState,
			ShareURL:    shareURL,
		},
	}
}

func TestSlackBody_Message(t *testing.T) {
	t.Parallel()
	got := slackText(t,
		slackAlert(EventApplicationNew, "Home", "26/01234/FUL", "1 High Street, Leeds", "Single-storey rear extension", new("Undecided"), new("https://share.towncrierapp.uk/a/leeds/26/01234/FUL")),
		slackAlert(EventApplicationDecision, "Home", "26/01001/HOU", "4 Mill Lane, Leeds", "Loft conversion", new("Approved"), new("https://share.towncrierapp.uk/a/leeds/26/01001/HOU")),
		slackAlert(EventApplicationNew, "Office", "26/09999/FUL", "9 Park Row, Leeds", "Change of use to café", nil, nil),
	)
	const want = "*Town Crier: 3 planning updates*\n" +
		"• New near Home: <https://share.towncrierapp.uk/a/leeds/26/01234/FUL|26/01234/FUL>, 1 High Street, Leeds. Single-storey rear extension\n" +
		"• Decision (Approved): <https://share.towncrierapp.uk/a/leeds/26/01001/HOU|26/01001/HOU>, 4 Mill Lane, Leeds. Loft conversion\n" +
		"• New near Office: 26/09999/FUL, 9 Park Row, Leeds. Change of use to café"
	if got != want {
		t.Errorf("text\n got %q\nwant %q", got, want)
	}
}

func TestSlackBody_Lines(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload Payload
		want    string
	}{
		{
			"saved-only decision", slackAlert(EventApplicationDecision, "", "26/1", "1 Road", "Shed", new("Rejected"), nil),
			"• Decision (Rejected): 26/1, 1 Road. Shed",
		},
		{
			"decision with no state", slackAlert(EventApplicationDecision, "Home", "26/1", "1 Road", "Shed", nil, nil),
			"• Decision: 26/1, 1 Road. Shed",
		},
		{
			"no address or description", slackAlert(EventApplicationNew, "Home", "26/1", "", "", nil, nil),
			"• New near Home: 26/1",
		},
		{
			"new with no zone", slackAlert(EventApplicationNew, "", "26/1", "1 Road", "Shed", nil, nil),
			"• New: 26/1, 1 Road. Shed",
		},
		{
			"line breaks collapse", slackAlert(EventApplicationNew, "My\nzone", "26/1", "1 Road\r\nTown", "A\n\n• B", nil, nil),
			"• New near My zone: 26/1, 1 Road Town. A • B",
		},
		{
			"escapes interpolated values",
			slackAlert(EventApplicationNew, "A & <!channel>", "<26>&", "<b> & c", "x > y", nil, new("https://example.com/?a=1&b=<2>")),
			"• New near A &amp; &lt;!channel&gt;: <https://example.com/?a=1&amp;b=&lt;2&gt;|&lt;26&gt;&amp;>, &lt;b&gt; &amp; c. x &gt; y",
		},
		{
			"escapes the decision state", slackAlert(EventApplicationDecision, "", "26/1", "", "", new("<x>&"), nil),
			"• Decision (&lt;x&gt;&amp;): 26/1",
		},
		{
			"150 runes are kept", slackAlert(EventApplicationNew, "Z", "r", "", strings.Repeat("é", 150), nil, nil),
			"• New near Z: r. " + strings.Repeat("é", 150),
		},
		{
			"151 runes are truncated", slackAlert(EventApplicationNew, "Z", "r", "", strings.Repeat("é", 151), nil, nil),
			"• New near Z: r. " + strings.Repeat("é", 150) + "…",
		},
		{
			"truncates before escaping", slackAlert(EventApplicationNew, "Z", "r", "", strings.Repeat("&", 151), nil, nil),
			"• New near Z: r. " + strings.Repeat("&amp;", 150) + "…",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := "*Town Crier: 1 planning update*\n" + tc.want
			if got := slackText(t, tc.payload); got != want {
				t.Errorf("text\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestSlackBody_CapsAtTenLines(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		count    int
		wantTail string
	}{
		{10, "• New near Home: ref-10"},
		{11, "…and 1 more in the Town Crier app"},
		{50, "…and 40 more in the Town Crier app"},
	} {
		t.Run(fmt.Sprint(tc.count), func(t *testing.T) {
			t.Parallel()
			payloads := make([]Payload, tc.count)
			for i := range payloads {
				payloads[i] = slackAlert(EventApplicationNew, "Home", fmt.Sprintf("ref-%d", i+1), "", "", nil, nil)
			}
			lines := strings.Split(slackText(t, payloads...), "\n")
			if lines[0] != fmt.Sprintf("*Town Crier: %d planning updates*", tc.count) {
				t.Errorf("header = %q", lines[0])
			}
			if got := strings.Count(strings.Join(lines, "\n"), "• "); got != 10 {
				t.Errorf("bullet lines = %d, want 10", got)
			}
			if lines[1] != "• New near Home: ref-1" {
				t.Errorf("first line = %q", lines[1])
			}
			if got := lines[len(lines)-1]; got != tc.wantTail {
				t.Errorf("last line = %q, want %q", got, tc.wantTail)
			}
		})
	}
}

func TestSlackBody_TestMessage(t *testing.T) {
	t.Parallel()
	got := slackText(t, BuildTestPayload("e", time.Unix(0, 0)))
	if want := "*Town Crier: test message*\nYour webhook works."; got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
}

func TestSlackBody_NoPayloads(t *testing.T) {
	t.Parallel()
	if _, err := SlackBody(nil); err == nil {
		t.Error("want an error for no payloads")
	}
}
