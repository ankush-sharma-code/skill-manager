package fake

import (
	"sync"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

// Clock returns one fixed time.
type Clock struct {
	mu sync.Mutex
	t  time.Time
}

func NewClock(t time.Time) *Clock {
	return &Clock{t: t}
}

func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

var _ ports.Clock = (*Clock)(nil)
