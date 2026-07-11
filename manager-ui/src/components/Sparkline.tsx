interface SparklineProps {
  values: number[];
  color?: string;
  width?: number;
  height?: number;
}

// Sparkline renders a compact inline SVG line chart — no charting dependency.
export default function Sparkline({
  values,
  color = "#1677ff",
  width = 180,
  height = 32,
}: SparklineProps) {
  if (values.length < 2) {
    return <span style={{ color: "#aaa", fontSize: 12 }}>—</span>;
  }
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;
  const points = values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * width;
      const y = height - ((v - min) / range) * height;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
  return (
    <svg width={width} height={height} style={{ display: "block" }}>
      <polyline fill="none" stroke={color} strokeWidth={1.5} points={points} />
    </svg>
  );
}
