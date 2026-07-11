package alert

import (
	"fmt"
	"sync/atomic"
)

// IDGen hands out monotonic alert IDs ("a-000123"). Shared by every alert
// source (inline checks and the dark sweeper) so IDs never collide.
type IDGen struct {
	n atomic.Uint64
}

// NewIDGen returns a generator starting at a-000001.
func NewIDGen() *IDGen { return &IDGen{} }

// Next returns the next unique alert ID.
func (g *IDGen) Next() string { return fmt.Sprintf("a-%06d", g.n.Add(1)) }
