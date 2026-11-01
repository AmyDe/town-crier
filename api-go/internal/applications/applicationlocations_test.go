package applications

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestBucketForWeekday_EachWeekdaySelectsItsOwnBucket(t *testing.T) {
	t.Parallel()
	seen := map[int]time.Weekday{}
	for d := time.Sunday; d <= time.Saturday; d++ {
		b := bucketForWeekday(d)
		if b < 0 || b > 6 {
			t.Fatalf("bucketForWeekday(%v) = %d, want 0..6", d, b)
		}
		if prev, dup := seen[b]; dup {
			t.Fatalf("%v and %v share bucket %d", prev, d, b)
		}
		seen[b] = d
	}
}

func TestSelectBuckets_EveryAuthorityFallsInExactlyOneWeekday(t *testing.T) {
	t.Parallel()
	all := make([]authorityBucket, 0, 100)
	for i := range 100 {
		all = append(all, authorityBucket{code: fmt.Sprint(i), bucket: i % 7})
	}
	counts := map[string]int{}
	for d := time.Sunday; d <= time.Saturday; d++ {
		for _, code := range selectBuckets(all, []int{bucketForWeekday(d)}) {
			counts[code]++
		}
	}
	for _, a := range all {
		if counts[a.code] != 1 {
			t.Errorf("authority %s selected %d times across the week, want 1", a.code, counts[a.code])
		}
	}
}

func TestSelectBuckets_AllBucketsSelectsEveryAuthority(t *testing.T) {
	t.Parallel()
	all := []authorityBucket{{"a", 0}, {"b", 3}, {"c", 6}}

	got := selectBuckets(all, allBuckets)

	if len(got) != 3 {
		t.Errorf("selected %v, want all three", got)
	}
}

func TestRetryDeadlock(t *testing.T) {
	t.Parallel()
	deadlock := &pgconn.PgError{Code: "40P01"}
	other := errors.New("other")
	cases := []struct {
		name      string
		errs      []error
		wantErr   error
		wantCalls int
	}{
		{"succeeds first time", []error{nil}, nil, 1},
		{"retries a deadlock then succeeds", []error{deadlock, nil}, nil, 2},
		{"gives up after three retries", []error{deadlock, deadlock, deadlock, deadlock, nil}, deadlock, 4},
		{"does not retry other errors", []error{other, nil}, other, 1},
		{"retries a wrapped deadlock", []error{fmt.Errorf("x: %w", deadlock), nil}, nil, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			err := retryDeadlock(context.Background(), func(context.Context, time.Duration) error { return nil }, func() error {
				e := tc.errs[calls]
				calls++
				return e
			})
			if calls != tc.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tc.wantCalls)
			}
			if tc.wantErr == nil && err != nil {
				t.Errorf("err = %v, want nil", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) && !isDeadlock(err) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
