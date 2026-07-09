package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These golden tests lock Sounder's parsing against the ACTUAL jabali2
// automation response shapes (Plane JAB-75 metrics, JAB-77 mail). If jabali2
// changes its wire format, these break before the UI silently shows blanks.

// serveJSON stands up an HTTP test server returning a fixed body on any path.
func serveJSON(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestServerStatusParsesJabali2Shape (JAB-75): /api/v1/automation/status returns
// {healthy, time, version, system, cpu, services}. system == agent system.info,
// cpu == agent system.cpu_usage. Sounder must read system/cpu into host/CPU.
func TestServerStatusParsesJabali2Shape(t *testing.T) {
	body := `{
		"healthy": true,
		"time": "2026-07-09T00:00:00Z",
		"version": "v1.2.3",
		"system": {
			"hostname": "panel-01",
			"os": "Debian 13",
			"load_avg": [0.5, 0.4, 0.3],
			"cpu_count": 4,
			"mem_total_kb": 8000000,
			"mem_available_kb": 5000000,
			"mem_used_kb": 3000000,
			"swap_total_kb": 2000000,
			"swap_used_kb": 100000,
			"partitions": [{"mount": "/", "total_kb": 100, "used_kb": 40}],
			"ntp_synced": true
		},
		"cpu": {
			"usage_percent": 12.5,
			"iowait_percent": 0.2,
			"per_core": [10.0, 15.0],
			"warming_up": false,
			"as_of": "2026-07-09T00:00:00Z"
		}
	}`
	srv := serveJSON(t, body)
	c := NewClient(srv.URL, "kid", "secret", false)

	status, code, err := c.ServerStatus(context.Background())
	if err != nil || code != http.StatusOK {
		t.Fatalf("ServerStatus err=%v code=%d", err, code)
	}
	if status.System == nil {
		t.Fatal("system slice not parsed")
	}
	if status.System.MemUsedKB != 3000000 || status.System.MemTotalKB != 8000000 {
		t.Errorf("mem parsed wrong: used=%d total=%d", status.System.MemUsedKB, status.System.MemTotalKB)
	}
	if len(status.System.LoadAvg) != 3 || status.System.LoadAvg[0] != 0.5 {
		t.Errorf("load_avg parsed wrong: %v", status.System.LoadAvg)
	}
	if status.CPU == nil || status.CPU.UsagePercent != 12.5 {
		t.Errorf("cpu usage parsed wrong: %+v", status.CPU)
	}
}

// TestMailboxesParsesJabali2Shape (JAB-77): /api/v1/automation/mail/mailboxes
// returns {email, domain, owner, quota_bytes, last_usage_bytes, disabled}.
// Sounder must map domain->DomainName, owner->UserUsername, disabled->IsDisabled.
func TestMailboxesParsesJabali2Shape(t *testing.T) {
	body := `{"data":[
		{"email":"a@example.com","domain":"example.com","owner":"alice","quota_bytes":1048576,"last_usage_bytes":512,"disabled":false},
		{"email":"b@example.com","domain":"example.com","owner":"bob","quota_bytes":0,"last_usage_bytes":0,"disabled":true}
	],"total":2}`
	srv := serveJSON(t, body)
	c := NewClient(srv.URL, "kid", "secret", false)

	resp, code, err := c.Mailboxes(context.Background())
	if err != nil || code != http.StatusOK {
		t.Fatalf("Mailboxes err=%v code=%d", err, code)
	}
	if resp.Total != 2 || len(resp.Data) != 2 {
		t.Fatalf("expected 2 mailboxes, got total=%d len=%d", resp.Total, len(resp.Data))
	}
	m0 := resp.Data[0]
	if m0.Email != "a@example.com" || m0.DomainName != "example.com" || m0.UserUsername != "alice" {
		t.Errorf("mailbox[0] mapped wrong: %+v", m0)
	}
	if m0.QuotaBytes != 1048576 || m0.LastUsageBytes != 512 {
		t.Errorf("mailbox[0] usage mapped wrong: %+v", m0)
	}
	if !resp.Data[1].IsDisabled {
		t.Errorf("mailbox[1] disabled flag not mapped")
	}
}
