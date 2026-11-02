package tc

import "io"

// Env carries the process streams the CLI reads from and writes to.
type Env struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}
