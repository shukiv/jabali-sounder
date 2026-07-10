package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

const monitorTimeout = 10 * time.Second

// statusCacheTTL bounds how long a managed panel's automation-status probe is
// reused. The Monitor page loads live + summary at once and both need status;
// two identical same-second HMAC requests trip the panel's replay protection,
// so a single probe is shared across near-simultaneous callers (SND-7).
const statusCacheTTL = 5 * time.Second

// MonitorHandlerConfig wires the monitor endpoints.
type MonitorHandlerConfig struct {
	Repo      repository.ServerRepository
	SecretKey *secrets.Key
	Log       *slog.Logger
	// AllowPlaintext permits the dev hex-plaintext token fallback (SND-6).
	AllowPlaintext bool
}

// RegisterMonitorRoutes mounts the monitor endpoints.
func RegisterMonitorRoutes(g *gin.RouterGroup, cfg MonitorHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &monitorHandler{cfg: cfg, statusCache: map[string]cachedStatus{}, now: time.Now}
	g.GET("/admin/monitor/live", h.live)
	g.GET("/admin/monitor/summary", h.summary)
}

type monitorHandler struct {
	cfg         MonitorHandlerConfig
	sf          singleflight.Group
	statusMu    sync.Mutex
	statusCache map[string]cachedStatus
	now         func() time.Time
	// probe overrides the real status fetch in tests; nil uses the panel client.
	probe func(ctx context.Context, s models.Server) (*remote.ServerStatusResp, int, error)
}

type cachedStatus struct {
	resp *remote.ServerStatusResp
	code int
	at   time.Time
}

type statusResult struct {
	resp *remote.ServerStatusResp
	code int
}

type monitorServerRef struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	BaseURL          string `json:"base_url"`
	Status           string `json:"status"`
	CredentialStatus string `json:"credential_status"`
	Version          string `json:"version"`
}

type monitorLiveEntry struct {
	Server        monitorServerRef `json:"server"`
	Available     bool             `json:"available"`
	AsOf          string           `json:"as_of,omitempty"`
	CPUPercent    *float64         `json:"cpu_percent,omitempty"`
	RAMUsedBytes  *int64           `json:"ram_used_bytes,omitempty"`
	RAMTotalBytes *int64           `json:"ram_total_bytes,omitempty"`
	RAMPercent    *float64         `json:"ram_percent,omitempty"`
	IOWaitPercent *float64         `json:"io_wait_percent,omitempty"`
	IOReadBPS     *float64         `json:"io_read_bps,omitempty"`
	IOWriteBPS    *float64         `json:"io_write_bps,omitempty"`
	Load1         *float64         `json:"load1,omitempty"`
	Load5         *float64         `json:"load5,omitempty"`
	Load15        *float64         `json:"load15,omitempty"`
	WarmingUp     bool             `json:"warming_up"`
	Error         string           `json:"error,omitempty"`
}

type monitorSummaryEntry struct {
	Server            monitorServerRef `json:"server"`
	Available         bool             `json:"available"`
	AsOf              string           `json:"as_of,omitempty"`
	DiskUsedBytes     *int64           `json:"disk_used_bytes,omitempty"`
	DiskTotalBytes    *int64           `json:"disk_total_bytes,omitempty"`
	DiskPercent       *float64         `json:"disk_percent,omitempty"`
	AccountsTotal     *int             `json:"accounts_total,omitempty"`
	DomainsTotal      *int             `json:"domains_total,omitempty"`
	ApplicationsTotal *int             `json:"applications_total,omitempty"`
	Error             string           `json:"error,omitempty"`
}

func (h *monitorHandler) live(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	results := make([]monitorLiveEntry, len(servers))
	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(8)

	for i := range servers {
		i := i
		s := servers[i]
		results[i] = monitorLiveEntry{Server: serverRef(s)}
		if s.Status != models.ServerStatusActive {
			results[i].Error = "server is not active"
			continue
		}
		g.Go(func() error {
			entry := h.fetchLive(gctx, s)
			results[i] = entry
			return nil
		})
	}
	_ = g.Wait()

	c.JSON(http.StatusOK, gin.H{
		"data":      results,
		"total":     len(results),
		"page":      1,
		"page_size": len(results),
	})
}

func (h *monitorHandler) summary(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	results := make([]monitorSummaryEntry, len(servers))
	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(8)

	for i := range servers {
		i := i
		s := servers[i]
		results[i] = monitorSummaryEntry{Server: serverRef(s)}
		if s.Status != models.ServerStatusActive {
			results[i].Error = "server is not active"
			continue
		}
		g.Go(func() error {
			entry := h.fetchSummary(gctx, s)
			results[i] = entry
			return nil
		})
	}
	_ = g.Wait()

	c.JSON(http.StatusOK, gin.H{
		"data":      results,
		"total":     len(results),
		"page":      1,
		"page_size": len(results),
	})
}

func (h *monitorHandler) fetchLive(ctx context.Context, s models.Server) monitorLiveEntry {
	entry := monitorLiveEntry{Server: serverRef(s)}
	status, code, err := h.serverStatus(ctx, s)
	if err != nil {
		entry.Error = safeRemoteError(h.cfg.Log, s.Name, "metrics", code, err)
		return entry
	}

	entry.Available = true
	entry.AsOf = status.AsOf
	if entry.AsOf == "" {
		entry.AsOf = status.Time
	}
	if status.CPU != nil {
		entry.CPUPercent = ptrFloat(status.CPU.UsagePercent)
		entry.IOWaitPercent = ptrFloat(status.CPU.IOWaitPercent)
		entry.WarmingUp = status.CPU.WarmingUp
		if entry.AsOf == "" {
			entry.AsOf = status.CPU.AsOf
		}
	}
	if status.IO != nil {
		entry.IOReadBPS = ptrFloat(status.IO.ReadBPS)
		entry.IOWriteBPS = ptrFloat(status.IO.WriteBPS)
	}
	host := status.Host
	if host == nil {
		host = status.System
	}
	if host != nil {
		used := host.MemUsedKB * 1024
		total := host.MemTotalKB * 1024
		entry.RAMUsedBytes = ptrInt64(used)
		entry.RAMTotalBytes = ptrInt64(total)
		entry.RAMPercent = percentPtr(used, total)
		if len(host.LoadAvg) > 0 {
			entry.Load1 = ptrFloat(host.LoadAvg[0])
		}
		if len(host.LoadAvg) > 1 {
			entry.Load5 = ptrFloat(host.LoadAvg[1])
		}
		if len(host.LoadAvg) > 2 {
			entry.Load15 = ptrFloat(host.LoadAvg[2])
		}
	}
	return entry
}

func (h *monitorHandler) fetchSummary(ctx context.Context, s models.Server) monitorSummaryEntry {
	entry := monitorSummaryEntry{Server: serverRef(s)}
	client, err := h.clientForServer(&s)
	if err != nil {
		h.cfg.Log.Warn("decrypt secret failed", "server", s.Name, "error", err)
		entry.Error = "server credential unavailable"
		return entry
	}

	var (
		mu       sync.Mutex
		errParts []string
	)
	addErr := func(part string) {
		mu.Lock()
		errParts = append(errParts, part)
		mu.Unlock()
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	g.Go(func() error {
		status, code, err := h.serverStatus(gctx, s)
		if err != nil {
			addErr(safeRemoteError(h.cfg.Log, s.Name, "metrics", code, err))
			return nil
		}
		mu.Lock()
		entry.Available = true
		entry.AsOf = status.AsOf
		if entry.AsOf == "" {
			entry.AsOf = status.Time
		}
		host := status.Host
		if host == nil {
			host = status.System
		}
		if host != nil {
			if used, total, ok := primaryDisk(host.Partitions); ok {
				entry.DiskUsedBytes = ptrInt64(used)
				entry.DiskTotalBytes = ptrInt64(total)
				entry.DiskPercent = percentPtr(used, total)
			}
		}
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, monitorTimeout)
		defer cancel()
		resp, code, err := client.Users(subCtx)
		if err != nil {
			addErr(safeRemoteError(h.cfg.Log, s.Name, "users", code, err))
			return nil
		}
		mu.Lock()
		entry.AccountsTotal = ptrInt(resp.Total)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, monitorTimeout)
		defer cancel()
		resp, code, err := client.Domains(subCtx)
		if err != nil {
			addErr(safeRemoteError(h.cfg.Log, s.Name, "domains", code, err))
			return nil
		}
		mu.Lock()
		entry.DomainsTotal = ptrInt(resp.Total)
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, monitorTimeout)
		defer cancel()
		resp, code, err := client.Applications(subCtx)
		if err != nil {
			addErr(safeRemoteError(h.cfg.Log, s.Name, "applications", code, err))
			return nil
		}
		mu.Lock()
		entry.ApplicationsTotal = ptrInt(resp.Total)
		mu.Unlock()
		return nil
	})

	_ = g.Wait()
	if len(errParts) > 0 {
		entry.Error = fmt.Sprintf("%v", errParts)
	}
	return entry
}

// serverStatus fetches the managed panel's automation status, sharing one probe
// across concurrent/near-simultaneous callers via singleflight + a short TTL
// cache keyed by server ID (SND-7). Live and summary both need status, and two
// identical same-second HMAC requests would trip the panel's replay protection.
func (h *monitorHandler) serverStatus(ctx context.Context, s models.Server) (*remote.ServerStatusResp, int, error) {
	h.statusMu.Lock()
	if e, ok := h.statusCache[s.ID]; ok && h.now().Sub(e.at) < statusCacheTTL {
		resp, code := e.resp, e.code
		h.statusMu.Unlock()
		return resp, code, nil
	}
	h.statusMu.Unlock()

	v, err, _ := h.sf.Do(s.ID, func() (any, error) {
		resp, code, ferr := h.fetchStatus(ctx, s)
		if ferr != nil {
			return statusResult{code: code}, ferr
		}
		h.statusMu.Lock()
		h.statusCache[s.ID] = cachedStatus{resp: resp, code: code, at: h.now()}
		h.statusMu.Unlock()
		return statusResult{resp: resp, code: code}, nil
	})
	res, _ := v.(statusResult)
	return res.resp, res.code, err
}

// fetchStatus performs the actual /automation/status probe (or the test override).
func (h *monitorHandler) fetchStatus(ctx context.Context, s models.Server) (*remote.ServerStatusResp, int, error) {
	if h.probe != nil {
		return h.probe(ctx, s)
	}
	client, err := h.clientForServer(&s)
	if err != nil {
		return nil, 0, err
	}
	subCtx, cancel := context.WithTimeout(ctx, monitorTimeout)
	defer cancel()
	return client.ServerStatus(subCtx)
}

func (h *monitorHandler) clientForServer(s *models.Server) (*remote.Client, error) {
	secret, err := h.decryptSecret(s)
	if err != nil {
		return nil, err
	}
	return remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify), nil
}

func (h *monitorHandler) decryptSecret(s *models.Server) (string, error) {
	return secrets.OpenSecret(h.cfg.SecretKey, s.TokenSecretEnc, h.cfg.AllowPlaintext)
}

func serverRef(s models.Server) monitorServerRef {
	return monitorServerRef{
		ID:               s.ID,
		Name:             s.Name,
		BaseURL:          s.BaseURL,
		Status:           string(s.Status),
		CredentialStatus: string(s.CredentialStatus),
		Version:          s.Version,
	}
}

func primaryDisk(partitions []remote.Partition) (int64, int64, bool) {
	if len(partitions) == 0 {
		return 0, 0, false
	}
	var used int64
	var total int64
	for _, p := range partitions {
		if p.MountPoint == "/" {
			return p.UsedBytes, p.TotalBytes, p.TotalBytes > 0
		}
		used += p.UsedBytes
		total += p.TotalBytes
	}
	return used, total, total > 0
}

func ptrFloat(v float64) *float64 { return &v }

func ptrInt(v int) *int { return &v }

func ptrInt64(v int64) *int64 { return &v }

func percentPtr(used, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	pct := float64(used) / float64(total) * 100
	return &pct
}
