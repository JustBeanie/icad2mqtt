// Package normalize applies conservative v1 heuristics. Agency category comes
// from one ordered rule table with fire precedence (so "Fire & EMS" and
// "Fire Police" are fire); type and address are kept whole because no reliable
// delimiter exists; municipality names stay null without an authority mapping.
// Invalid/nonexistent local times are dropped, while bad page timestamps warn.
package normalize
