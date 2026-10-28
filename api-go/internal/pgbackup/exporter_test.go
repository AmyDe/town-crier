package pgbackup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type fakeSnapshot struct {
	version   int64
	rows      map[string]int64
	copyErr   map[string]error
	closed    bool
	beginErr  error
	versionEr error
}

func (s *fakeSnapshot) SchemaVersion(context.Context) (int64, error) { return s.version, s.versionEr }

func (s *fakeSnapshot) CopyTable(_ context.Context, w io.Writer, t Table) (int64, error) {
	if err := s.copyErr[t.Name]; err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintf(w, "csv-for-%s\n", t.Name); err != nil {
		return 0, err
	}
	return s.rows[t.Name], nil
}

func (s *fakeSnapshot) Close(context.Context) error { s.closed = true; return nil }

type fakeSource struct{ snap *fakeSnapshot }

func (f fakeSource) Begin(context.Context) (Snapshot, error) { return f.snap, f.snap.beginErr }

type fakeSink struct {
	order  []string
	blobs  map[string][]byte
	failOn string
	err    error
}

func (s *fakeSink) Put(_ context.Context, name string, body io.Reader) error {
	if s.failOn != "" && strings.HasSuffix(name, s.failOn) {
		return s.err
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if s.blobs == nil {
		s.blobs = map[string][]byte{}
	}
	s.blobs[name] = data
	s.order = append(s.order, name)
	return nil
}

func newTestExporter(snap *fakeSnapshot, sink *fakeSink) *Exporter {
	now := func() time.Time { return time.Date(2026, 10, 28, 5, 15, 0, 0, time.UTC) }
	return New(fakeSource{snap}, sink, now, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestExporter_UploadsEveryTableThenManifestLast(t *testing.T) {
	t.Parallel()
	snap := &fakeSnapshot{version: 43, rows: map[string]int64{"users": 3}}
	sink := &fakeSink{}

	res, err := newTestExporter(snap, sink).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(sink.order) != len(Tables)+1 {
		t.Fatalf("uploads = %d, want %d", len(sink.order), len(Tables)+1)
	}
	if last := sink.order[len(sink.order)-1]; last != "2026/10/28/manifest.json" {
		t.Errorf("last upload = %q, want the manifest", last)
	}
	if !snap.closed {
		t.Error("snapshot was not closed")
	}
	if res.Rows != 3 || len(res.Tables) != len(Tables) {
		t.Errorf("result rows=%d tables=%d, want 3 and %d", res.Rows, len(res.Tables), len(Tables))
	}
}

func TestExporter_ObjectsAreGzipAndManifestMatches(t *testing.T) {
	t.Parallel()
	snap := &fakeSnapshot{version: 43, rows: map[string]int64{"users": 3}}
	sink := &fakeSink{}

	res, err := newTestExporter(snap, sink).Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var m Manifest
	if err := json.Unmarshal(sink.blobs["2026/10/28/manifest.json"], &m); err != nil {
		t.Fatalf("manifest json: %v", err)
	}
	if m.SchemaVersion != 43 || !m.SnapshotAt.Equal(time.Date(2026, 10, 28, 5, 15, 0, 0, time.UTC)) {
		t.Errorf("manifest header = %+v", m)
	}
	var users ManifestTable
	for _, mt := range m.Tables {
		if mt.Name == "users" {
			users = mt
		}
	}
	blob := sink.blobs["2026/10/28/users.csv.gz"]
	sum := sha256.Sum256(blob)
	if users.SHA256 != hex.EncodeToString(sum[:]) || users.Rows != 3 || users.Bytes != int64(len(blob)) {
		t.Errorf("users manifest entry = %+v, blob len %d", users, len(blob))
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	plain, _ := io.ReadAll(zr)
	if string(plain) != "csv-for-users\n" {
		t.Errorf("decompressed = %q", plain)
	}
	if res.Bytes == 0 {
		t.Error("result bytes = 0")
	}
}

func TestExporter_SinkFailureStopsRunWithoutManifest(t *testing.T) {
	t.Parallel()
	boom := errors.New("sink down")
	snap := &fakeSnapshot{}
	sink := &fakeSink{failOn: "watch_zones.csv.gz", err: boom}

	_, err := newTestExporter(snap, sink).Run(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want wrapping %v", err, boom)
	}
	if _, ok := sink.blobs["2026/10/28/manifest.json"]; ok {
		t.Error("manifest uploaded after a sink failure")
	}
	if len(sink.order) != 1 {
		t.Errorf("uploads after failure = %v, want only users", sink.order)
	}
}

func TestExporter_CopyFailureStopsRunWithoutManifest(t *testing.T) {
	t.Parallel()
	boom := errors.New("copy broke")
	snap := &fakeSnapshot{copyErr: map[string]error{"watch_zones": boom}}
	sink := &fakeSink{}

	_, err := newTestExporter(snap, sink).Run(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want wrapping %v", err, boom)
	}
	if _, ok := sink.blobs["2026/10/28/manifest.json"]; ok {
		t.Error("manifest uploaded after a copy failure")
	}
}

func TestExporter_BeginFailureIsReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("no tx")
	snap := &fakeSnapshot{beginErr: boom}

	_, err := newTestExporter(snap, &fakeSink{}).Run(context.Background())

	if !errors.Is(err, boom) {
		t.Fatalf("error = %v, want wrapping %v", err, boom)
	}
}
