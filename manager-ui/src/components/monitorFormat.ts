// Shared formatters for the Monitor overview (SND-81/82): uptime durations,
// heartbeat ages, and absolute local timestamps. Kept dependency-free so the
// table and the expandable detail rows format values identically.

export function fmtUptime(seconds?: number): string {
  if (typeof seconds !== "number" || Number.isNaN(seconds) || seconds <= 0) return "n/a";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

// fmtAge renders a compact relative age ("3m ago", "2h ago", "4d ago").
export function fmtAge(iso?: string): string {
  if (!iso) return "never";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "unknown";
  const secs = Math.max(0, Math.floor((Date.now() - t) / 1000));
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

// fmtAbs renders an absolute local timestamp.
export function fmtAbs(iso?: string): string {
  if (!iso) return "n/a";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "unknown" : d.toLocaleString();
}
