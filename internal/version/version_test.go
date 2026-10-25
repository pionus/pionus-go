package version

import "testing"

func TestString(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = oldVersion, oldCommit, oldDate
	})

	Version, Commit, BuildDate = "0.1.0", "dev", "unknown"
	if got := String(); got != "0.1.0" {
		t.Fatalf("String() = %q", got)
	}

	Version, Commit, BuildDate = "0.1.0", "abc123", "2026-10-25T12:00:00Z"
	if got := String(); got != "0.1.0 (abc123, 2026-10-25T12:00:00Z)" {
		t.Fatalf("String() = %q", got)
	}

	Version = ""
	if got := String(); got != "dev (abc123, 2026-10-25T12:00:00Z)" {
		t.Fatalf("String() = %q", got)
	}
}
