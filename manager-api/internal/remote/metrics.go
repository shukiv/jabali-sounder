package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ServerStatusResp is the thinned automation status envelope exposed by a
// managed server. Newer jabali2 builds enrich /automation/status with
// system/cpu slices; older builds return only the bare healthy stub and the
// manager degrades per field.
type ServerStatusResp struct {
	AsOf    string              `json:"as_of"`
	Time    string              `json:"time"`
	Version string              `json:"version"`
	Healthy bool                `json:"healthy"`
	Host    *HostStatusSlice    `json:"host"`
	System  *HostStatusSlice    `json:"system"`
	CPU     *CPUStatusSlice     `json:"cpu"`
	IO      *IOStatusSlice      `json:"io"`
	Errors  map[string]string   `json:"errors,omitempty"`
	Alerts  []ServerStatusAlert `json:"alerts,omitempty"`
	// Services + Net are additive fields Sounder consumes when the managed Panel
	// exposes them (JAB-150 / SND-80/81); absent today, rendered as unsupported.
	Services []ServiceHealth `json:"services,omitempty"`
	Net      *NetTelemetry   `json:"net,omitempty"`
}

// ServiceHealth is one workload's health as reported by the managed Panel.
type ServiceHealth struct {
	Name        string `json:"name"`
	Status      string `json:"status"` // healthy | degraded | failed
	LastChecked string `json:"last_checked,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

// NetTelemetry is the managed server's network throughput + loss over a window.
type NetTelemetry struct {
	DownloadBPS   float64 `json:"download_bps"`
	UploadBPS     float64 `json:"upload_bps"`
	PacketLossPct float64 `json:"packet_loss_pct"`
	WindowSeconds int     `json:"window_seconds"`
}

// HostStatusSlice mirrors the host slice used by jabali2's server-status page.
type HostStatusSlice struct {
	Hostname       string      `json:"hostname"`
	OS             string      `json:"os"`
	Kernel         string      `json:"kernel"`
	CPUModel       string      `json:"cpu_model"`
	Timezone       string      `json:"timezone"`
	UptimeSeconds  float64     `json:"uptime_seconds"`
	LoadAvg        []float64   `json:"load_avg"`
	CPUCount       int         `json:"cpu_count"`
	MemTotalKB     int64       `json:"mem_total_kb"`
	MemAvailableKB int64       `json:"mem_available_kb"`
	MemUsedKB      int64       `json:"mem_used_kb"`
	SwapTotalKB    int64       `json:"swap_total_kb"`
	SwapUsedKB     int64       `json:"swap_used_kb"`
	Partitions     []Partition `json:"partitions"`
	NTPSynced      bool        `json:"ntp_synced"`
}

// Partition is one filesystem usage row from the managed server.
type Partition struct {
	MountPoint string `json:"mount_point"`
	TotalBytes int64  `json:"total_bytes"`
	UsedBytes  int64  `json:"used_bytes"`
	FreeBytes  int64  `json:"free_bytes"`
}

// CPUStatusSlice is the live CPU snapshot from the managed server.
type CPUStatusSlice struct {
	UsagePercent  float64   `json:"usage_percent"`
	IOWaitPercent float64   `json:"iowait_percent"`
	PerCore       []float64 `json:"per_core"`
	WarmingUp     bool      `json:"warming_up"`
	AsOf          string    `json:"as_of"`
}

// IOStatusSlice is intentionally permissive so future jabali2 automation
// payloads can add richer disk IO without breaking older managers.
type IOStatusSlice struct {
	ReadBPS   float64 `json:"read_bps"`
	WriteBPS  float64 `json:"write_bps"`
	ReadIOPS  float64 `json:"read_iops"`
	WriteIOPS float64 `json:"write_iops"`
	AsOf      string  `json:"as_of"`
}

// ServerStatusAlert is one warning/critical row from the managed server.
type ServerStatusAlert struct {
	Level  string `json:"level"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// ServerStatus calls GET /api/v1/automation/status on the managed server.
func (c *Client) ServerStatus(ctx context.Context) (*ServerStatusResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/status")
	if err != nil {
		return nil, 0, fmt.Errorf("server status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("server status: HTTP %d", resp.StatusCode)
	}
	var result ServerStatusResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("server status decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

// MetricSnapshot is a compact time-series sample derived from a server status.
// Fields are pointers so "not reported" is distinct from zero.
type MetricSnapshot struct {
	CPUPercent  *float64
	RAMPercent  *float64
	DiskPercent *float64
	Load1       *float64
}

// Snapshot extracts a compact metrics sample from a server status response.
func (s *ServerStatusResp) Snapshot() MetricSnapshot {
	var m MetricSnapshot
	if s == nil {
		return m
	}
	if s.CPU != nil {
		v := s.CPU.UsagePercent
		m.CPUPercent = &v
	}
	host := s.Host
	if host == nil {
		host = s.System
	}
	if host != nil {
		if host.MemTotalKB > 0 {
			p := float64(host.MemUsedKB) / float64(host.MemTotalKB) * 100
			m.RAMPercent = &p
		}
		if used, total, ok := snapshotPrimaryDisk(host.Partitions); ok {
			p := float64(used) / float64(total) * 100
			m.DiskPercent = &p
		}
		if len(host.LoadAvg) > 0 {
			v := host.LoadAvg[0]
			m.Load1 = &v
		}
	}
	return m
}

func snapshotPrimaryDisk(partitions []Partition) (int64, int64, bool) {
	for _, p := range partitions {
		if p.MountPoint == "/" {
			return p.UsedBytes, p.TotalBytes, p.TotalBytes > 0
		}
	}
	var used, total int64
	for _, p := range partitions {
		used += p.UsedBytes
		total += p.TotalBytes
	}
	return used, total, total > 0
}

// CheckWithMetrics performs /health + /automation/status in a single pass,
// returning both the health result and the raw status (for metrics). It replaces
// CheckHealth in the poller so status isn't fetched twice — two same-second
// requests to /automation/status would trip the panel's replay protection.
func (c *Client) CheckWithMetrics(ctx context.Context) (*CheckResult, *ServerStatusResp, error) {
	result := &CheckResult{}

	h, hcode, err := c.Health(ctx)
	if err != nil {
		result.Reachable = false
		result.HealthError = err.Error()
		result.HealthCode = hcode
		return result, nil, nil //nolint:nilerr // reachable=false is a result
	}
	result.Reachable = true
	result.HealthCode = hcode
	result.Version = h.Version

	st, scode, err := c.ServerStatus(ctx)
	if err != nil {
		result.CredentialValid = false
		result.StatusError = err.Error()
		result.StatusCode = scode
		return result, nil, nil //nolint:nilerr
	}
	result.CredentialValid = true
	result.StatusCode = scode
	result.Healthy = st.Healthy
	return result, st, nil
}
