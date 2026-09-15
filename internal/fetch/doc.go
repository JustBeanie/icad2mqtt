// Package fetch retrieves and bounds the CAD source response.
//
// Consecutive failures use full jitter while preserving the 60-second floor:
// delay = poll interval + random[0, min(10 minutes, poll interval *
// 2^failures) - poll interval). A successful fetch resets the failure count.
package fetch
