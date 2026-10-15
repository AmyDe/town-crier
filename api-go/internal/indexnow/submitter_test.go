package indexnow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSource struct {
	pairs []Pair
	err   error
}

func (f fakeSource) Pairs(context.Context) ([]Pair, error) { return f.pairs, f.err }

type fakeState struct {
	prev    map[string]string
	has     bool
	saved   map[string]string
	saves   int
	loadErr error
	saveErr error
}

func (f *fakeState) Load(context.Context) (map[string]string, bool, error) {
	return f.prev, f.has, f.loadErr
}

func (f *fakeState) Save(_ context.Context, m map[string]string) error {
	f.saves++
	f.saved = m
	return f.saveErr
}

type indexNowServer struct {
	mu        sync.Mutex
	statuses  []int
	bodies    []submission
	responses int
}

type submission struct {
	Host        string   `json:"host"`
	Key         string   `json:"key"`
	KeyLocation string   `json:"keyLocation"`
	URLList     []string `json:"urlList"`
}

func (s *indexNowServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := io.ReadAll(r.Body)
	var sub submission
	_ = json.Unmarshal(raw, &sub)
	s.bodies = append(s.bodies, sub)
	status := http.StatusOK
	if s.responses < len(s.statuses) {
		status = s.statuses[s.responses]
	} else if len(s.statuses) > 0 {
		status = s.statuses[len(s.statuses)-1]
	}
	s.responses++
	w.WriteHeader(status)
	_, _ = w.Write([]byte("body-" + http.StatusText(status)))
}

func newSubmitter(t *testing.T, srv *indexNowServer, src fakeSource, st *fakeState) (*Submitter, *[]time.Duration) {
	t.Helper()
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	var sleeps []time.Duration
	s := New(src, st, Config{
		Host:     "towncrierapp.uk",
		Key:      "k123",
		Endpoint: ts.URL,
		Client:   ts.Client(),
		Sleep:    func(_ context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil },
	}, slog.New(slog.DiscardHandler))
	return s, &sleeps
}

var threePairs = []Pair{
	{"https://towncrierapp.uk/planning", "2026-06-28"},
	{"https://towncrierapp.uk/planning/a", "2026-06-27"},
	{"https://towncrierapp.uk/planning/b", ""},
}

func TestSubmit_NoPreviousSetSubmitsAll(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{}
	st := &fakeState{}
	s, _ := newSubmitter(t, srv, fakeSource{pairs: threePairs}, st)

	res, err := s.Submit(context.Background())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Submitted != 3 || res.Skipped != 0 {
		t.Errorf("result = %+v, want 3 submitted 0 skipped", res)
	}
	if len(srv.bodies) != 1 {
		t.Fatalf("posts = %d, want 1", len(srv.bodies))
	}
	b := srv.bodies[0]
	if b.Host != "towncrierapp.uk" || b.Key != "k123" || b.KeyLocation != "https://towncrierapp.uk/k123.txt" || len(b.URLList) != 3 {
		t.Errorf("payload = %+v", b)
	}
	if st.saves != 1 || len(st.saved) != 3 || st.saved["https://towncrierapp.uk/planning/a"] != "2026-06-27" {
		t.Errorf("saved = %v (saves %d)", st.saved, st.saves)
	}
}

func TestSubmit_OnlyNewOrChangedAndDeletedDropped(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{statuses: []int{http.StatusAccepted}}
	st := &fakeState{has: true, prev: map[string]string{
		"https://towncrierapp.uk/planning":   "2026-06-27",
		"https://towncrierapp.uk/planning/a": "2026-06-27",
		"https://towncrierapp.uk/planning/z": "2026-01-01",
	}}
	s, _ := newSubmitter(t, srv, fakeSource{pairs: threePairs}, st)

	res, err := s.Submit(context.Background())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	want := []string{"https://towncrierapp.uk/planning", "https://towncrierapp.uk/planning/b"}
	if !slices.Equal(srv.bodies[0].URLList, want) {
		t.Errorf("urlList = %v, want %v", srv.bodies[0].URLList, want)
	}
	if res.Submitted != 2 || res.Skipped != 1 {
		t.Errorf("result = %+v", res)
	}
	if _, ok := st.saved["https://towncrierapp.uk/planning/z"]; ok {
		t.Error("deleted URL kept in saved set")
	}
}

func TestSubmit_NothingChangedSkipsPostAndSave(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{}
	prev := map[string]string{}
	for _, p := range threePairs {
		prev[p.Loc] = p.Lastmod
	}
	st := &fakeState{has: true, prev: prev}
	s, _ := newSubmitter(t, srv, fakeSource{pairs: threePairs}, st)

	res, err := s.Submit(context.Background())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if res.Submitted != 0 || res.Skipped != 3 || len(srv.bodies) != 0 {
		t.Errorf("result = %+v, posts = %d", res, len(srv.bodies))
	}
}

func TestSubmit_RetriesOn403ThenSucceeds(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{statuses: []int{403, 403, 200}}
	st := &fakeState{}
	s, sleeps := newSubmitter(t, srv, fakeSource{pairs: threePairs}, st)

	if _, err := s.Submit(context.Background()); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(srv.bodies) != 3 {
		t.Errorf("posts = %d, want 3", len(srv.bodies))
	}
	if len(*sleeps) != 2 || (*sleeps)[0] != 30*time.Second {
		t.Errorf("sleeps = %v, want two of 30s", *sleeps)
	}
}

func TestSubmit_Persistent403FailsAfterSixAttemptsWithoutSaving(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{statuses: []int{403}}
	st := &fakeState{}
	s, _ := newSubmitter(t, srv, fakeSource{pairs: threePairs}, st)

	_, err := s.Submit(context.Background())
	if !errors.Is(err, ErrNotAccepted) {
		t.Fatalf("err = %v, want ErrNotAccepted", err)
	}
	if len(srv.bodies) != 6 {
		t.Errorf("posts = %d, want 6", len(srv.bodies))
	}
	if st.saves != 0 {
		t.Error("state saved despite rejection")
	}
}

func TestSubmit_HardErrorIncludesStatusAndBody(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{statuses: []int{422}}
	st := &fakeState{}
	s, sleeps := newSubmitter(t, srv, fakeSource{pairs: threePairs}, st)

	_, err := s.Submit(context.Background())
	if !errors.Is(err, ErrNotAccepted) {
		t.Fatalf("err = %v, want ErrNotAccepted", err)
	}
	for _, want := range []string{"422", "body-Unprocessable Entity"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if len(srv.bodies) != 1 || len(*sleeps) != 0 || st.saves != 0 {
		t.Errorf("posts %d sleeps %d saves %d", len(srv.bodies), len(*sleeps), st.saves)
	}
}

func TestSubmit_BatchesOver10000(t *testing.T) {
	t.Parallel()
	pairs := make([]Pair, 10001)
	for i := range pairs {
		pairs[i] = Pair{Loc: "https://towncrierapp.uk/planning/" + time.Duration(i).String(), Lastmod: "d"}
	}
	srv := &indexNowServer{}
	s, _ := newSubmitter(t, srv, fakeSource{pairs: pairs}, &fakeState{})

	res, err := s.Submit(context.Background())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(srv.bodies) != 2 || len(srv.bodies[0].URLList) != 10000 || len(srv.bodies[1].URLList) != 1 || res.Submitted != 10001 {
		t.Errorf("batches = %d, result = %+v", len(srv.bodies), res)
	}
}

func TestSubmit_EmptySitemapIsAnError(t *testing.T) {
	t.Parallel()
	s, _ := newSubmitter(t, &indexNowServer{}, fakeSource{}, &fakeState{})
	if _, err := s.Submit(context.Background()); err == nil {
		t.Fatal("want error for an empty sitemap")
	}
}

func TestSubmit_SourceAndStateErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	tests := []struct {
		name string
		src  fakeSource
		st   *fakeState
	}{
		{"source", fakeSource{err: boom}, &fakeState{}},
		{"load", fakeSource{pairs: threePairs}, &fakeState{loadErr: boom}},
		{"save", fakeSource{pairs: threePairs}, &fakeState{saveErr: boom}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, _ := newSubmitter(t, &indexNowServer{}, tt.src, tt.st)
			if _, err := s.Submit(context.Background()); !errors.Is(err, boom) {
				t.Fatalf("err = %v, want boom", err)
			}
		})
	}
}

func TestSubmit_CancelledDuringBackoffStops(t *testing.T) {
	t.Parallel()
	srv := &indexNowServer{statuses: []int{403}}
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	s := New(fakeSource{pairs: threePairs}, &fakeState{}, Config{
		Host: "h", Key: "k", Endpoint: ts.URL, Client: ts.Client(),
		Sleep: func(context.Context, time.Duration) error { return context.Canceled },
	}, slog.New(slog.DiscardHandler))

	if _, err := s.Submit(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(srv.bodies) != 1 {
		t.Errorf("posts = %d, want 1", len(srv.bodies))
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
