export function ProgressBar({ ratio, label }: { ratio: number; label?: string }) {
  const pct = Math.max(0, Math.min(100, ratio * 100));
  return (
    <div
      className="relative h-4 w-full border border-border-dark rounded-[2px] overflow-hidden"
      style={{ background: 'rgba(255, 252, 240, 0.55)', boxShadow: 'inset 0 1px 3px rgba(96, 48, 0, 0.3)' }}
    >
      <div
        className="h-full"
        style={{
          width: `${pct}%`,
          background: 'linear-gradient(to bottom, #a2d86e, #5aa337 55%, #3f7d26)',
          boxShadow: 'inset 0 1px 0 rgba(255, 255, 255, 0.5), inset 0 -2px 3px rgba(30, 60, 15, 0.35)',
        }}
      />
      {label && (
        <span className="absolute inset-0 flex items-center justify-center text-[10px] font-semibold tabular-nums"
          style={{ textShadow: '0 1px 0 rgba(255, 250, 230, 0.6)' }}>
          {label}
        </span>
      )}
    </div>
  );
}
