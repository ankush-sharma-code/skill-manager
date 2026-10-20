package ports_test

import (
	"testing"
	"time"

	"github.com/ankush-sharma-code/skill-manager/internal/ports"
)

func TestSystemClock(t *testing.T) {
	now := time.Now()
	got := ports.SystemClock{}.Now()
	if got.Before(now.Add(-time.Minute)) || got.After(now.Add(time.Minute)) {
		t.Fatalf("Now = %s", got)
	}
}
