package platform

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDateOnly_MarshalsAsYearMonthDay(t *testing.T) {
	t.Parallel()
	d := DateOnly(time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC))
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `"2026-03-07"` {
		t.Errorf("got %s, want \"2026-03-07\"", raw)
	}
}

func TestDateOnly_RoundTrip(t *testing.T) {
	t.Parallel()
	var d DateOnly
	if err := json.Unmarshal([]byte(`"2026-12-31"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := time.Time(d)
	if got.Year() != 2026 || got.Month() != time.December || got.Day() != 31 {
		t.Errorf("parsed wrong date: %v", got)
	}
}

func TestDateOnlyPtr_NilPreserved(t *testing.T) {
	t.Parallel()
	if DateOnlyPtr(nil) != nil {
		t.Error("nil time must map to nil DateOnly")
	}
	tm := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	d := DateOnlyPtr(&tm)
	if d == nil {
		t.Fatal("non-nil time must map to non-nil DateOnly")
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `"2026-01-02"` {
		t.Errorf("got %s", raw)
	}
}

func TestDateOnly_NilPointerMarshalsNull(t *testing.T) {
	t.Parallel()
	type holder struct {
		D *DateOnly `json:"d"`
	}
	raw, err := json.Marshal(holder{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != `{"d":null}` {
		t.Errorf("got %s, want {\"d\":null}", raw)
	}
}

func TestDateOnly_TimePtrRoundTrip(t *testing.T) {
	t.Parallel()
	var d DateOnly
	if err := json.Unmarshal([]byte(`"2026-06-13"`), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tp := d.TimePtr()
	if tp == nil || !tp.Equal(time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("TimePtr: got %v", tp)
	}
}
