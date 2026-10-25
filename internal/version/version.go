// Package version holds build metadata. Release builds override these values
// with -ldflags.
package version

import "strings"

var (
	// Version is the semantic version without a leading "v" (for example 0.1.0).
	Version = "0.1.1"
	// Commit is the short source revision.
	Commit = "dev"
	// BuildDate is the UTC build timestamp.
	BuildDate = "unknown"
)

// String returns a human-readable version string.
func String() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	var details []string
	if c := strings.TrimSpace(Commit); c != "" && c != "dev" && c != "unknown" {
		details = append(details, c)
	}
	if d := strings.TrimSpace(BuildDate); d != "" && d != "unknown" {
		details = append(details, d)
	}
	if len(details) == 0 {
		return v
	}
	return v + " (" + strings.Join(details, ", ") + ")"
}
