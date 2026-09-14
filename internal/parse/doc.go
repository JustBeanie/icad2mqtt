// Package parse discovers the incident table by labels, independent of nesting.
// It skips malformed rows and counts them; absent/conflicting headers and an
// absent usable table are page failures. A header-only table is a valid empty
// page; the header itself is the usable structure. Text is expected to be UTF-8.
package parse
