package main

import (
	"context"
	"errors"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

type fakeApplicationLocationsJobs struct {
	res applications.ReconcileResult
	err error
	all bool
}

func (f *fakeApplicationLocationsJobs) Load(context.Context) (int64, error) { return 3, f.err }

func (f *fakeApplicationLocationsJobs) Reconcile(_ context.Context, all bool) (applications.ReconcileResult, error) {
	f.all = all
	return f.res, f.err
}

func TestApplicationLocationsRunner_ReconcileReportsRepairsAndAbandonedAuthorities(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		name        string
		jobs        *fakeApplicationLocationsJobs
		wantRepairs int
		wantErr     error
	}{
		{"sums upserts and deletes", &fakeApplicationLocationsJobs{res: applications.ReconcileResult{Upserted: 4, Deleted: 2}}, 6, nil},
		{"abandoned authority fails the run", &fakeApplicationLocationsJobs{res: applications.ReconcileResult{Upserted: 1, Failed: 1}}, 1, errAuthoritiesAbandoned},
		{"passes errors through", &fakeApplicationLocationsJobs{err: boom}, 0, boom},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := applicationLocationsRunner{jobs: tc.jobs}.Reconcile(context.Background(), true)
			if got != tc.wantRepairs || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && err != nil) {
				t.Errorf("Reconcile() = %d, %v; want %d, %v", got, err, tc.wantRepairs, tc.wantErr)
			}
			if !tc.jobs.all {
				t.Errorf("all flag was not passed through")
			}
		})
	}
}
