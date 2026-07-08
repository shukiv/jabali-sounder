// Package ids centralises ULID generation + validation.
package ids

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// NewULID returns a new ULID in its canonical 26-char textual form.
func NewULID() string {
	mu.Lock()
	defer mu.Unlock()
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}
