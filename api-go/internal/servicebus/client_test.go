package servicebus

import (
	"errors"
	"testing"
)

func TestNewClient_RequiresNamespaceAndQueue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		namespace string
		queue     string
		wantErr   error
	}{
		{"valid", "sb-town-crier-prod.servicebus.windows.net", "poll-triggers", nil},
		{"missing namespace", "", "poll-triggers", ErrMissingNamespace},
		{"missing queue", "sb-town-crier-prod.servicebus.windows.net", "", ErrMissingQueue},
		{"both missing", "", "", ErrMissingNamespace},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, err := NewClient(tc.namespace, tc.queue, "")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewClient: got err=%v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewClient: unexpected error %v", err)
			}
			if client == nil {
				t.Fatal("NewClient: got nil client for valid config")
			}
			t.Cleanup(func() { _ = client.Close(t.Context()) })
			if client.QueueName() != tc.queue {
				t.Errorf("QueueName: got %q, want %q", client.QueueName(), tc.queue)
			}
		})
	}
}

// TestQueueDepth_IsEmpty proves the fork-guard's emptiness/threshold semantics
// (GH#938 PR2): a queue is "empty" only when it has no live (active+scheduled)
// trigger. Dead-lettered messages are corpses, not live triggers, so a queue
// holding only dead letters still counts as empty of live triggers.
func TestQueueDepth_IsEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		depth QueueDepth
		want  bool
	}{
		{"all zero", QueueDepth{}, true},
		{"active present", QueueDepth{ActiveMessageCount: 1}, false},
		{"scheduled present", QueueDepth{ScheduledMessageCount: 1}, false},
		{"dead letters only", QueueDepth{DeadLetterMessageCount: 4}, true},
		{"active and dead letters", QueueDepth{ActiveMessageCount: 1, DeadLetterMessageCount: 4}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.depth.IsEmpty(); got != tc.want {
				t.Errorf("IsEmpty(): got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestQueueDepth_TriggerCount proves TriggerCount sums only the live
// (active+scheduled) counts, excluding dead letters — the threshold the
// bootstrap reconciler uses to detect a fork (>1).
func TestQueueDepth_TriggerCount(t *testing.T) {
	t.Parallel()
	depth := QueueDepth{ActiveMessageCount: 2, ScheduledMessageCount: 3, DeadLetterMessageCount: 10}
	if got, want := depth.TriggerCount(), int64(5); got != want {
		t.Errorf("TriggerCount(): got %d, want %d (dead letters must not count)", got, want)
	}
}

func TestNormalizeFQDN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		// Pulumi sets the env var to the full FQDN; accept it unchanged.
		{
			"already fqdn",
			"sb-town-crier-prod.servicebus.windows.net",
			"sb-town-crier-prod.servicebus.windows.net",
		},
		// A bare namespace name gets the suffix appended to build the full FQDN.
		{"bare name", "sb-town-crier-prod", "sb-town-crier-prod.servicebus.windows.net"},
		// Mixed-case suffix is still recognised (case-insensitive).
		{
			"uppercase suffix",
			"sb-town-crier-prod.SERVICEBUS.WINDOWS.NET",
			"sb-town-crier-prod.SERVICEBUS.WINDOWS.NET",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeFQDN(tc.in); got != tc.want {
				t.Errorf("normalizeFQDN(%q): got %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
