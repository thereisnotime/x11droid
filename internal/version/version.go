// Package version exposes build metadata stamped in at link time via
// -ldflags -X (see the justfile). A plain `go build` leaves the dev defaults.
package version

import "fmt"

var (
	// Version is `git describe` output (tag or short sha, with -dirty suffix).
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = "none"
	// Date is the UTC build timestamp (RFC3339).
	Date = "unknown"
)

// String returns a compact one-line version summary.
func String() string {
	return fmt.Sprintf("%s (%s, built %s)", Version, Commit, Date)
}
