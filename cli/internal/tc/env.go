package tc

import "io"

// Env carries the process streams the CLI reads from and writes to. Injecting
// them keeps commands testable without touching the real stdio.
type Env struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}
