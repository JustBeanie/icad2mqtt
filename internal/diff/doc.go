// Package diff compares valid snapshots. A failed page must be rejected by the
// caller before Diff, so the previous snapshot remains authoritative. The
// one-shot closed event deletes its remembered incident to bound memory.
package diff
