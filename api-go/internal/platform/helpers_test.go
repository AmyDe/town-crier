package platform

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSleep_ReturnsNilAfterDuration(t *testing.T) {
	t.Parallel()
	if err := Sleep(context.Background(), time.Millisecond); err != nil {
		t.Fatalf("Sleep: %v", err)
	}
}

func TestSleep_ReturnsContextErrorOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Sleep(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
}

func TestPtr_ReturnsPointerToValue(t *testing.T) {
	t.Parallel()
	p := Ptr(42)
	if p == nil {
		t.Fatal("Ptr returned nil")
	}
	if *p != 42 {
		t.Errorf("got %d, want 42", *p)
	}

	s := Ptr("application/json")
	if s == nil || *s != "application/json" {
		t.Errorf("got %v, want pointer to \"application/json\"", s)
	}
}

func TestDateOnlyPtrToTime_NilPointerMapsToNil(t *testing.T) {
	t.Parallel()
	if DateOnlyPtrToTime(nil) != nil {
		t.Error("nil *DateOnly must map to nil *time.Time")
	}
}

func TestDateOnlyPtrToTime_ValueMapsToTime(t *testing.T) {
	t.Parallel()
	d := DateOnly(time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC))
	tp := DateOnlyPtrToTime(&d)
	if tp == nil {
		t.Fatal("non-nil *DateOnly must map to non-nil *time.Time")
	}
	if !tp.Equal(time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("got %v, want 2026-06-13 UTC", tp)
	}
}
