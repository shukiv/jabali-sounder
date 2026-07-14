package remote

import (
	"encoding/json"
	"testing"
)

// TestServerStatusResp_DecodesPanelShape locks the wire contract: the panel
// emits BOTH a raw "services" object AND the normalized "service_health" array
// (JAB-150). Binding to "services" broke decode for every server; this guards
// against that regression and confirms service_health + net populate.
func TestServerStatusResp_DecodesPanelShape(t *testing.T) {
	raw := `{
	  "healthy": true,
	  "time": "2026-07-14T19:00:00Z",
	  "services": { "services": [ {"unit":"nginx.service","active":"active","sub":"running"} ] },
	  "service_health": [
	    {"name":"web","unit":"nginx.service","status":"healthy","reason":"running","last_checked":"2026-07-14T19:00:00Z"},
	    {"name":"mail","unit":"jabali-stalwart.service","status":"stopped","last_checked":"2026-07-14T19:00:00Z"}
	  ],
	  "net": {"download_bps":10000,"upload_bps":5000,"packet_loss_pct":5.26,"window_seconds":3},
	  "host": {"os":"Debian GNU/Linux 13"}
	}`
	var s ServerStatusResp
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("decode panel status: %v", err)
	}
	if len(s.Services) != 2 || s.Services[0].Name != "web" || s.Services[0].Status != "healthy" {
		t.Fatalf("service_health not parsed: %+v", s.Services)
	}
	if s.Services[1].Status != "stopped" {
		t.Fatalf("stopped status: %+v", s.Services[1])
	}
	if s.Net == nil || s.Net.DownloadBPS != 10000 || s.Net.WindowSeconds != 3 || s.Net.PacketLossPct < 5.2 {
		t.Fatalf("net not parsed: %+v", s.Net)
	}
	if s.Host == nil || s.Host.OS == "" {
		t.Fatalf("host lost: %+v", s.Host)
	}
}
