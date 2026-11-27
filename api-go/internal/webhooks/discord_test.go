package webhooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

type discordMessageBody struct {
	Content         string `json:"content"`
	AllowedMentions struct {
		Parse []string `json:"parse"`
	} `json:"allowed_mentions"`
	Flags int `json:"flags"`
}

func discordContent(t *testing.T, payloads ...Payload) string {
	t.Helper()
	raw, err := DiscordBody(payloads)
	if err != nil {
		t.Fatal(err)
	}
	return decodeDiscord(t, raw).Content
}

func decodeDiscord(t *testing.T, raw []byte) discordMessageBody {
	t.Helper()
	var body discordMessageBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.AllowedMentions.Parse == nil || len(body.AllowedMentions.Parse) != 0 {
		t.Errorf("allowed_mentions.parse = %#v, want an empty array", body.AllowedMentions.Parse)
	}
	if body.Flags != 4 {
		t.Errorf("flags = %d, want 4", body.Flags)
	}
	return body
}

func TestDiscordBody_TestMessage(t *testing.T) {
	t.Parallel()
	raw, err := DiscordBody([]Payload{BuildTestPayload("e", time.Unix(0, 0))})
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"content":"**Town Crier: test message**\nYour webhook works.","allowed_mentions":{"parse":[]},"flags":4}`
	if string(raw) != want {
		t.Errorf("body\n got %s\nwant %s", raw, want)
	}
}

func TestDiscordBody_Message(t *testing.T) {
	t.Parallel()
	got := discordContent(t,
		slackAlert(EventApplicationNew, "Home", "26/01234/FUL", "1 High Street, Leeds", "Single-storey rear extension", new("Undecided"), new("https://share.towncrierapp.uk/a/leeds/26/01234/FUL")),
		slackAlert(EventApplicationDecision, "Home", "26/01001/HOU", "4 Mill Lane, Leeds", "Loft conversion", new("Approved"), new("https://share.towncrierapp.uk/a/leeds/26/01001/HOU")),
		slackAlert(EventApplicationNew, "Office", "26/09999/FUL", "9 Park Row, Leeds", "Change of use to café", nil, nil),
	)
	const want = "**Town Crier: 3 planning updates**\n" +
		"• New near Home: [26/01234/FUL](https://share.towncrierapp.uk/a/leeds/26/01234/FUL), 1 High Street, Leeds. Single-storey rear extension\n" +
		"• Decision (Approved): [26/01001/HOU](https://share.towncrierapp.uk/a/leeds/26/01001/HOU), 4 Mill Lane, Leeds. Loft conversion\n" +
		"• New near Office: 26/09999/FUL, 9 Park Row, Leeds. Change of use to café"
	if got != want {
		t.Errorf("content\n got %q\nwant %q", got, want)
	}
}

func TestDiscordBody_Lines(t *testing.T) {
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
			"decision with no state", slackAlert(EventApplicationDecision, "Home", "26/1", "1 Road", "Shed", new(""), nil),
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
			"line breaks collapse", slackAlert(EventApplicationNew, "My\nzone", "26/1", "1 Road\r\nTown", "A\n\n# B", nil, nil),
			"• New near My zone: 26/1, 1 Road Town. A # B",
		},
		{
			"every markdown character is escaped",
			slackAlert(EventApplicationNew, "", "26/1", "", "\\ * _ ~ ` | [ ] ( ) < >", nil, nil),
			"• New: 26/1. \\\\ \\* \\_ \\~ \\` \\| \\[ \\] \\( \\) \\< \\>",
		},
		{
			"mentions and links in council text",
			slackAlert(EventApplicationNew, "", "26/1", "", "@everyone **x** [a](b) <@123>", nil, nil),
			`• New: 26/1. @everyone \*\*x\*\* \[a\]\(b\) \<@123\>`,
		},
		{
			"escapes every interpolated value",
			slackAlert(EventApplicationNew, "*Z*", "[26]", "_A_", "", nil, new("https://example.com/a_b?x=*")),
			`• New near \*Z\*: [\[26\]](https://example.com/a_b?x=*), \_A\_`,
		},
		{
			"escapes the decision state", slackAlert(EventApplicationDecision, "", "26/1", "", "", new("~x~"), nil),
			`• Decision (\~x\~): 26/1`,
		},
		{
			"share URL cannot break out of the link",
			slackAlert(EventApplicationNew, "", "26/1", "", "", nil, new("https://example.com/a) @here <b> (c\n\td ")),
			"• New: [26/1](https://example.com/a%29%20@here%20%3Cb%3E%20%28c%0A%09d%C2%A0)",
		},
		{
			"150 runes are kept", slackAlert(EventApplicationNew, "Z", "r", "", strings.Repeat("é", 150), nil, nil),
			"• New near Z: r. " + strings.Repeat("é", 150),
		},
		{
			"truncates before escaping", slackAlert(EventApplicationNew, "Z", "r", "", strings.Repeat("*", 151), nil, nil),
			"• New near Z: r. " + strings.Repeat(`\*`, 150) + "…",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := "**Town Crier: 1 planning update**\n" + tc.want
			if got := discordContent(t, tc.payload); got != want {
				t.Errorf("content\n got %q\nwant %q", got, want)
			}
		})
	}
}

func TestDiscordBody_CapsAtTenLines(t *testing.T) {
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
			lines := strings.Split(discordContent(t, payloads...), "\n")
			if want := fmt.Sprintf("**Town Crier: %d planning updates**", tc.count); lines[0] != want {
				t.Errorf("header = %q, want %q", lines[0], want)
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

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func TestDiscordBody_FitsTheLimitInUTF16Units(t *testing.T) {
	t.Parallel()
	long := func(unit string, runes int) string { return strings.Repeat(unit, runes) }
	for _, tc := range []struct {
		name  string
		unit  string
		count int
	}{
		{"ascii", "a", 10},
		{"astral", "🏠", 10},
		{"astral with more than ten", "🏠", 25},
		{"escaped", "*", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payloads := make([]Payload, tc.count)
			for i := range payloads {
				payloads[i] = slackAlert(EventApplicationNew, long(tc.unit, 100), fmt.Sprintf("ref-%02d", i),
					long(tc.unit, 200), long(tc.unit, 1000), nil, new("https://share.towncrierapp.uk/a/leeds/26/01234/FUL"))
			}

			content := discordContent(t, payloads...)

			if n := utf16Len(content); n > 2000 {
				t.Errorf("content = %d UTF-16 units, want at most 2000", n)
			}
			listed := strings.Count(content, "• ")
			if listed == 0 || listed >= tc.count {
				t.Fatalf("listed = %d of %d", listed, tc.count)
			}
			for i := range tc.count {
				if got := strings.Contains(content, fmt.Sprintf("[ref-%02d]", i)); got != (i < listed) {
					t.Errorf("payload %d listed = %v, want the first %d in order", i, got, listed)
				}
			}
			if want := fmt.Sprintf("\n…and %d more in the Town Crier app", tc.count-listed); !strings.HasSuffix(content, want) {
				t.Errorf("content does not end %q", want)
			}
		})
	}
}

// The one-update content is the header, "\n• New: " and the reference.
func TestDiscordBody_LimitBoundary(t *testing.T) {
	t.Parallel()
	const header = "**Town Crier: 1 planning update**"
	room := 2000 - utf16Len(header+"\n• New: ")
	fits := strings.Repeat("🏠", room/2) + strings.Repeat("a", room%2)

	got := discordContent(t, slackAlert(EventApplicationNew, "", fits, "", "", nil, nil))
	if n := utf16Len(got); n != 2000 || got != header+"\n• New: "+fits {
		t.Errorf("a line that exactly fits: %d units", n)
	}

	got = discordContent(t, slackAlert(EventApplicationNew, "", fits+"a", "", "", nil, nil))
	if want := header + "\n…and 1 more in the Town Crier app"; got != want {
		t.Errorf("a line one unit too long: content = %q", got)
	}
}

func TestDiscordBody_StopsAtTheFirstLineThatDoesNotFit(t *testing.T) {
	t.Parallel()
	got := discordContent(t,
		slackAlert(EventApplicationNew, "", "first", "", "", nil, nil),
		slackAlert(EventApplicationNew, "", strings.Repeat("a", 2000), "", "", nil, nil),
		slackAlert(EventApplicationNew, "", "third", "", "", nil, nil),
	)
	const want = "**Town Crier: 3 planning updates**\n• New: first\n…and 2 more in the Town Crier app"
	if got != want {
		t.Errorf("content\n got %q\nwant %q", got, want)
	}
}

func TestDiscordBody_ReservesRoomForTheMoreSuffix(t *testing.T) {
	t.Parallel()
	const header = "**Town Crier: 2 planning updates**"
	const suffix = "\n…and 1 more in the Town Crier app"
	room := 2000 - utf16Len(header+"\n• New: "+suffix)
	for _, tc := range []struct {
		name   string
		length int
		listed int
	}{
		{"first line leaves room", room, 1},
		{"first line leaves no room", room + 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := discordContent(t,
				slackAlert(EventApplicationNew, "", strings.Repeat("a", tc.length), "", "", nil, nil),
				slackAlert(EventApplicationNew, "", strings.Repeat("b", 100), "", "", nil, nil),
			)
			if listed := strings.Count(got, "• "); listed != tc.listed {
				t.Errorf("listed = %d, want %d", listed, tc.listed)
			}
			if utf16Len(got) > 2000 || !strings.HasSuffix(got, fmt.Sprintf("\n…and %d more in the Town Crier app", 2-tc.listed)) {
				t.Errorf("content = %d units, ends %q", utf16Len(got), got[max(0, len(got)-60):])
			}
		})
	}
}

func TestDiscordBody_NoPayloads(t *testing.T) {
	t.Parallel()
	if _, err := DiscordBody(nil); !errors.Is(err, errNoPayloads) {
		t.Errorf("err = %v, want errNoPayloads", err)
	}
}
