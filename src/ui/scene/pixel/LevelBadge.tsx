export function LevelBadge({ x, y, level }: { x: number; y: number; level: number }) {
  return (
    <g transform={`translate(${x}, ${y})`} className="badge">
      <rect x="-9" y="-9" width="18" height="18" rx="3" fill="#2f7d32" stroke="#f4efe0" strokeWidth="1.4" />
      <rect x="-7.5" y="-7.5" width="15" height="7" rx="2" fill="#fff" opacity="0.15" />
      <text y="4" textAnchor="middle" fontSize="11" fontWeight="700" fill="#fff" fontFamily="ui-sans-serif, system-ui, sans-serif">{level}</text>
    </g>
  );
}
