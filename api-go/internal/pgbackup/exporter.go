// Package pgbackup writes a nightly logical backup of the user-owned Postgres
// tables to blob storage. Each table streams through COPY ... TO STDOUT and
// gzip straight into the sink, so nothing touches disk, and every table comes
// from one REPEATABLE READ snapshot.
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
	"hash"
	"io"
	"log/slog"
	"time"
)

// Sink stores one named object. Put must read body to EOF and return nil only
// when the object is durably stored.
type Sink interface {
	Put(ctx context.Context, name string, body io.Reader) error
}

// Snapshot is one consistent, read-only view of the database.
type Snapshot interface {
	SchemaVersion(ctx context.Context) (int64, error)
	// CopyTable writes the table as CSV with a header row and returns the row count.
	CopyTable(ctx context.Context, w io.Writer, t Table) (int64, error)
	Close(ctx context.Context) error
}

// Source opens snapshots.
type Source interface {
	Begin(ctx context.Context) (Snapshot, error)
}

// Manifest describes a complete dump. Its presence marks the dump as complete
// because it is uploaded last.
type Manifest struct {
	SchemaVersion int64           `json:"schemaVersion"`
	SnapshotAt    time.Time       `json:"snapshotAt"`
	Tables        []ManifestTable `json:"tables"`
}

// ManifestTable records one uploaded file. SHA256 and Bytes describe the
// gzip file as stored.
type ManifestTable struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	Rows   int64  `json:"rows"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// TableResult is the per-table outcome of a run.
type TableResult struct {
	Name  string
	Rows  int64
	Bytes int64
}

// Result is the outcome of a complete run.
type Result struct {
	Tables []TableResult
	Rows   int64
	Bytes  int64
}

// Exporter runs one backup.
type Exporter struct {
	source Source
	sink   Sink
	now    func() time.Time
	logger *slog.Logger
}

// New builds an Exporter. The object path prefix is the UTC date of now().
func New(source Source, sink Sink, now func() time.Time, logger *slog.Logger) *Exporter {
	return &Exporter{source: source, sink: sink, now: now, logger: logger}
}

// Run uploads every table in Tables, then the manifest. Any failure stops the
// run before the manifest is written.
func (e *Exporter) Run(ctx context.Context) (Result, error) {
	snap, err := e.source.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin snapshot: %w", err)
	}
	defer func() {
		if cerr := snap.Close(context.WithoutCancel(ctx)); cerr != nil {
			e.logger.WarnContext(ctx, "pg-backup: close snapshot", "error", cerr)
		}
	}()

	started := e.now().UTC()
	prefix := started.Format("2006/01/02") + "/"

	version, err := snap.SchemaVersion(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read schema version: %w", err)
	}

	manifest := Manifest{SchemaVersion: version, SnapshotAt: started}
	var res Result
	for _, t := range Tables {
		file := prefix + t.Name + ".csv.gz"
		mt, err := e.uploadTable(ctx, snap, t, file)
		if err != nil {
			return Result{}, fmt.Errorf("back up %s: %w", t.Name, err)
		}
		manifest.Tables = append(manifest.Tables, mt)
		res.Tables = append(res.Tables, TableResult{Name: t.Name, Rows: mt.Rows, Bytes: mt.Bytes})
		res.Rows += mt.Rows
		res.Bytes += mt.Bytes
	}

	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode manifest: %w", err)
	}
	if err := e.sink.Put(ctx, prefix+"manifest.json", bytes.NewReader(body)); err != nil {
		return Result{}, fmt.Errorf("upload manifest: %w", err)
	}
	return res, nil
}

type countingHasher struct {
	w io.Writer
	h hash.Hash
	n int64
}

func (c *countingHasher) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.h.Write(p[:n])
	c.n += int64(n)
	return n, err
}

func (e *Exporter) uploadTable(ctx context.Context, snap Snapshot, t Table, file string) (ManifestTable, error) {
	pr, pw := io.Pipe()
	ch := &countingHasher{w: pw, h: sha256.New()}

	type copyOutcome struct {
		rows int64
		err  error
	}
	done := make(chan copyOutcome, 1)
	go func() {
		gz := gzip.NewWriter(ch)
		rows, err := snap.CopyTable(ctx, gz, t)
		if cerr := gz.Close(); err == nil {
			err = cerr
		}
		_ = pw.CloseWithError(err)
		done <- copyOutcome{rows: rows, err: err}
	}()

	putErr := e.sink.Put(ctx, file, pr)
	_ = pr.CloseWithError(errors.New("sink stopped reading"))
	out := <-done

	if out.err != nil {
		return ManifestTable{}, errors.Join(out.err, putErr)
	}
	if putErr != nil {
		return ManifestTable{}, fmt.Errorf("upload %s: %w", file, putErr)
	}
	return ManifestTable{
		Name:   t.Name,
		File:   file,
		Rows:   out.rows,
		Bytes:  ch.n,
		SHA256: hex.EncodeToString(ch.h.Sum(nil)),
	}, nil
}
