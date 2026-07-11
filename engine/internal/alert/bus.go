package alert

// Sink receives a finished alert. The engine's only outbound path is the
// dashboard websocket (no SMS, email, or webhooks; see CLAUDE.md out-of-scope).
// A Sink is the seam between the checks that raise alerts and whatever consumes
// them (the websocket hub in the binary, a counter in the benchmark).
type Sink func(Alert)

// Discard is a no-op sink for benchmarks and tests.
func Discard(Alert) {}
