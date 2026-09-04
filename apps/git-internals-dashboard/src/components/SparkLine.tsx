interface SparkLineProps {
  points: number[];
  color: string;
  width?: number;
  height?: number;
}

// Inline SVG polyline matching the design comp (92×28 viewport, non-scaling stroke).
export function SparkLine({ points, color, width = 92, height = 28 }: SparkLineProps) {
  if (points.length < 2) return <svg width={width} height={height} />;
  const max = Math.max(...points, 1);
  const step = width / (points.length - 1);
  const coords = points
    .map((v, i) => `${(i * step).toFixed(1)},${(height - 3 - (v / max) * (height - 6)).toFixed(1)}`)
    .join(" ");
  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      style={{ flexShrink: 0, overflow: "visible" }}
    >
      <polyline
        points={coords}
        fill="none"
        stroke={color}
        strokeWidth="1.8"
        vectorEffect="non-scaling-stroke"
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  );
}
