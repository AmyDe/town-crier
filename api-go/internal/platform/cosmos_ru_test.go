package platform

import (
	"context"
	"errors"
	"testing"
)

// fakeRUMetrics records the request-charge calls a Cosmos op makes. It satisfies
// the platform consumer-side cosmosMetricsRecorder interface.
type fakeRUMetrics struct {
	charges    []float64
	operations []string
	containers []string
}

func (f *fakeRUMetrics) CosmosRequestCharge(_ context.Context, ru float64, operation, container string) {
	f.charges = append(f.charges, ru)
	f.operations = append(f.operations, operation)
	f.containers = append(f.containers, container)
}

func TestTraceCosmosOp_RecordsRequestCharge(t *testing.T) {
	t.Parallel()
	rec := &fakeRUMetrics{}
	c := &CosmosContainer{name: "Users", accountHost: "acct.documents.azure.com", metrics: rec}

	err := traceCosmosOp(context.Background(), c, "ReadItem", func(context.Context) (float64, error) {
		return 2.75, nil
	})
	if err != nil {
		t.Fatalf("traceCosmosOp: %v", err)
	}

	if len(rec.charges) != 1 || rec.charges[0] != 2.75 {
		t.Errorf("CosmosRequestCharge charges = %v, want [2.75]", rec.charges)
	}
	if len(rec.operations) != 1 || rec.operations[0] != "ReadItem" {
		t.Errorf("operations = %v, want [ReadItem]", rec.operations)
	}
	if len(rec.containers) != 1 || rec.containers[0] != "Users" {
		t.Errorf("containers = %v, want [Users]", rec.containers)
	}
}

func TestTraceCosmosOp_DoesNotRecordChargeOnError(t *testing.T) {
	t.Parallel()
	rec := &fakeRUMetrics{}
	c := &CosmosContainer{name: "Users", accountHost: "acct.documents.azure.com", metrics: rec}
	sentinel := errors.New("cosmos boom")

	err := traceCosmosOp(context.Background(), c, "ReadItem", func(context.Context) (float64, error) {
		return 0, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	if len(rec.charges) != 0 {
		t.Errorf("must not record a charge on error: %v", rec.charges)
	}
}

func TestTraceCosmosOp_NilMetricsRecorderIsNoOp(t *testing.T) {
	t.Parallel()
	c := &CosmosContainer{name: "Users", accountHost: "acct.documents.azure.com"}
	err := traceCosmosOp(context.Background(), c, "ReadItem", func(context.Context) (float64, error) {
		return 1.0, nil
	})
	if err != nil {
		t.Fatalf("traceCosmosOp without recorder: %v", err)
	}
}
