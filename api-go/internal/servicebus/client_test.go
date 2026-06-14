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
		// A bare namespace name gets the suffix appended (mirrors .NET's
		// double-suffix guard that avoids NXDOMAIN).
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
