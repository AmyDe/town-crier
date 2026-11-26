package main

import (
	"reflect"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/webhooks"
)

func TestDormantDeleters_EveryErasureStepIsWired(t *testing.T) {
	t.Parallel()
	st := newStores(lazyPool(t), discardLogger())

	deleters := dormantDeleters(platform.Config{}, st, discardLogger())

	fields := reflect.ValueOf(deleters)
	for i := range fields.NumField() {
		if fields.Field(i).IsZero() {
			t.Errorf("erasure step %s is not wired", fields.Type().Field(i).Name)
		}
	}
	if store, ok := deleters.Webhooks.(*webhooks.PostgresStore); !ok || store != st.webhook {
		t.Errorf("webhook deleter = %T, want the worker's webhook store", deleters.Webhooks)
	}
}
