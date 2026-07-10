package api

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
)

// TestServerStatusSharesProbe covers SND-7: concurrent live+summary status
// requests for one server collapse into a single automation-status probe (so
// two identical same-second HMAC requests don't trip the panel's replay
// protection), and a subsequent call within the TTL reuses the cached result.
func TestServerStatusSharesProbe(t *testing.T) {
	var calls int32
	fixed := time.Unix(1_700_000_000, 0)

	h := &monitorHandler{
		cfg:         MonitorHandlerConfig{Log: slog.New(slog.NewTextHandler(nil, nil))},
		statusCache: map[string]cachedStatus{},
		now:         func() time.Time { return fixed },
		probe: func(_ context.Context, _ models.Server) (*remote.ServerStatusResp, int, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(25 * time.Millisecond) // let concurrent callers pile into singleflight
			return &remote.ServerStatusResp{}, 200, nil
		},
	}
	s := models.Server{ID: "srv1", Name: "srv1"}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, code, err := h.serverStatus(context.Background(), s); err != nil || code != 200 {
				t.Errorf("serverStatus: code=%d err=%v", code, err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("probe called %d times for concurrent callers, want 1", got)
	}

	// Within TTL -> cache hit, no new probe.
	if _, _, err := h.serverStatus(context.Background(), s); err != nil {
		t.Fatalf("cached call: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("cached call re-probed: calls=%d", got)
	}
}

// TestServerStatusReprobesAfterTTL asserts the cache expires so 60s refreshes
// still get fresh data.
func TestServerStatusReprobesAfterTTL(t *testing.T) {
	var calls int32
	now := time.Unix(1_700_000_000, 0)
	h := &monitorHandler{
		cfg:         MonitorHandlerConfig{Log: slog.New(slog.NewTextHandler(nil, nil))},
		statusCache: map[string]cachedStatus{},
		now:         func() time.Time { return now },
		probe: func(_ context.Context, _ models.Server) (*remote.ServerStatusResp, int, error) {
			atomic.AddInt32(&calls, 1)
			return &remote.ServerStatusResp{}, 200, nil
		},
	}
	s := models.Server{ID: "srv1"}

	h.serverStatus(context.Background(), s)
	now = now.Add(statusCacheTTL + time.Second) // advance past TTL
	h.serverStatus(context.Background(), s)

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected re-probe after TTL, calls=%d want 2", got)
	}
}
