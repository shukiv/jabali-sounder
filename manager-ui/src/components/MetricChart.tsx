interface Series {
  label: string;
  color: string;
  values: number[];
}

interface Props {
  series: Series[];
  timestamps: string[];
  yMax?: number;
  height?: number;
}

// MetricChart is a dependency-free multi-series line chart with a shared Y axis
// (default 0..100 for percentages), gridlines, and a legend (M6 / SND-25).
export default function MetricChart({ series, timestamps, yMax = 100, height = 160 }: Props) {
  const width = 640;
  const padL = 32;
  const padB = 18;
  const padT = 8;
  const plotW = width - padL - 4;
  const plotH = height - padB - padT;
  const n = timestamps.length;
  if (n < 2) return <div style={{ color: "#aaa", fontSize: 12 }}>Not enough data in range.</div>;

  const max = Math.max(yMax, ...series.flatMap((s) => s.values), 1);
  const x = (i: number) => padL + (i / (n - 1)) * plotW;
  const y = (v: number) => padT + plotH - (v / max) * plotH;

  const gridY = [0, max / 2, max];
  const firstT = new Date(timestamps[0]);
  const lastT = new Date(timestamps[n - 1]);
  const fmt = (d: Date) =>
    lastT.getTime() - firstT.getTime() > 2 * 86400000
      ? d.toLocaleDateString([], { month: "short", day: "numeric" })
      : d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

  return (
    <div style={{ overflowX: "auto" }}>
      <svg width={width} height={height} style={{ display: "block", maxWidth: "100%" }} role="img" aria-label="metric chart">
        {gridY.map((gv, i) => (
          <g key={i}>
            <line x1={padL} y1={y(gv)} x2={width - 4} y2={y(gv)} stroke="rgba(128,128,128,0.2)" strokeWidth={1} />
            <text x={0} y={y(gv) + 3} fontSize={10} fill="#999">
              {Math.round(gv)}
            </text>
          </g>
        ))}
        <text x={padL} y={height - 4} fontSize={10} fill="#999">{fmt(firstT)}</text>
        <text x={width - 4} y={height - 4} fontSize={10} fill="#999" textAnchor="end">{fmt(lastT)}</text>
        {series.map((s) =>
          s.values.length === n ? (
            <polyline
              key={s.label}
              fill="none"
              stroke={s.color}
              strokeWidth={1.5}
              points={s.values.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ")}
            />
          ) : null,
        )}
      </svg>
      <div style={{ display: "flex", gap: 16, flexWrap: "wrap", marginTop: 4 }}>
        {series.map((s) => (
          <span key={s.label} style={{ fontSize: 12, color: "#888" }}>
            <span style={{ display: "inline-block", width: 10, height: 10, background: s.color, borderRadius: 2, marginRight: 4 }} />
            {s.label}
            {s.values.length ? <strong> {s.values[s.values.length - 1].toFixed(1)}</strong> : null}
          </span>
        ))}
      </div>
    </div>
  );
}
