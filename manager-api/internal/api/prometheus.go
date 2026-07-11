package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// PrometheusHandlerConfig wires the Prometheus exporter (SND-23).
type PrometheusHandlerConfig struct {
	Servers       repository.ServerRepository
	MetricSamples repository.MetricSampleRepository
	Log           *slog.Logger
}

// RegisterPrometheusRoutes mounts /api/v1/metrics/prometheus. It sits under the
// authed admin group, so an API token (Authorization: Bearer snd_…) or a session
// scrapes it. Point Prometheus at metrics_path=/api/v1/metrics/prometheus.
func RegisterPrometheusRoutes(g *gin.RouterGroup, cfg PrometheusHandlerConfig) {
	if cfg.Servers == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &prometheusHandler{cfg: cfg}
	g.GET("/metrics/prometheus", h.scrape)
}

type prometheusHandler struct{ cfg PrometheusHandlerConfig }

func (h *prometheusHandler) scrape(c *gin.Context) {
	ctx := c.Request.Context()
	servers, err := h.cfg.Servers.List(ctx)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	now := time.Now()
	var b strings.Builder

	help(&b, "jabali_server_up", "gauge", "1 if the server is active with valid credentials, else 0.")
	healthy := 0
	for _, s := range servers {
		up := 0.0
		if s.Status == models.ServerStatusActive && s.CredentialStatus == models.CredentialValid {
			up = 1
			healthy++
		}
		metric(&b, "jabali_server_up", s, up)
	}

	// Per-server resource gauges from the latest sample.
	cpu := map[string]*float64{}
	ram := map[string]*float64{}
	disk := map[string]*float64{}
	load := map[string]*float64{}
	for _, s := range servers {
		if h.cfg.MetricSamples == nil {
			break
		}
		rows, err := h.cfg.MetricSamples.Recent(ctx, s.ID, 1)
		if err != nil || len(rows) == 0 {
			continue
		}
		cpu[s.ID] = rows[0].CPUPercent
		ram[s.ID] = rows[0].RAMPercent
		disk[s.ID] = rows[0].DiskPercent
		load[s.ID] = rows[0].Load1
	}
	emit := func(name, typ, helpText string, vals map[string]*float64) {
		help(&b, name, typ, helpText)
		for _, s := range servers {
			if v := vals[s.ID]; v != nil {
				metric(&b, name, s, *v)
			}
		}
	}
	emit("jabali_server_cpu_percent", "gauge", "Latest CPU utilisation percent.", cpu)
	emit("jabali_server_ram_percent", "gauge", "Latest RAM utilisation percent.", ram)
	emit("jabali_server_disk_percent", "gauge", "Latest primary-disk utilisation percent.", disk)
	emit("jabali_server_load1", "gauge", "Latest 1-minute load average.", load)

	help(&b, "jabali_server_cert_expiry_seconds", "gauge", "Seconds until the TLS certificate expires (negative if expired).")
	for _, s := range servers {
		if s.CertExpiresAt != nil {
			metric(&b, "jabali_server_cert_expiry_seconds", s, s.CertExpiresAt.Sub(now).Seconds())
		}
	}

	help(&b, "jabali_fleet_servers_total", "gauge", "Total enrolled servers.")
	b.WriteString("jabali_fleet_servers_total " + strconv.Itoa(len(servers)) + "\n")
	help(&b, "jabali_fleet_servers_healthy", "gauge", "Servers that are active with valid credentials.")
	b.WriteString("jabali_fleet_servers_healthy " + strconv.Itoa(healthy) + "\n")

	c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	c.String(http.StatusOK, b.String())
}

// help writes a HELP+TYPE preamble for a metric family.
func help(b *strings.Builder, name, typ, text string) {
	b.WriteString("# HELP " + name + " " + text + "\n")
	b.WriteString("# TYPE " + name + " " + typ + "\n")
}

// metric writes one sample with server/id/env labels.
func metric(b *strings.Builder, name string, s models.Server, value float64) {
	b.WriteString(name)
	b.WriteString(`{server="`)
	b.WriteString(escapeLabel(s.Name))
	b.WriteString(`",id="`)
	b.WriteString(escapeLabel(s.ID))
	b.WriteString(`",environment="`)
	b.WriteString(escapeLabel(s.Environment))
	b.WriteString(`"} `)
	b.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	b.WriteString("\n")
}

// escapeLabel escapes a Prometheus label value (backslash, quote, newline).
func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}
