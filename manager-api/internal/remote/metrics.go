package remote

import (
	"context"
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
	if err := decodeJSONBody(resp, &result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("server status decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}
