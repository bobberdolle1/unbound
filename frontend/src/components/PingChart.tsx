export const PingChart = ({ history, livePingData }: { history: number[]; livePingData?: { active: boolean; latency: number; status: string } }) => {
  if (history.length === 0) return null;
  const maxVal = Math.max(...history, 100);
  const minVal = 0;
  const range = maxVal - minVal;
  const width = 300;
  const height = 44;
  const padding = 4;

  const points = history.map((val, idx) => {
    const x = padding + (idx / (history.length - 1 || 1)) * (width - padding * 2);
    const y = height - padding - ((val - minVal) / range) * (height - padding * 2);
    return `${x},${y}`;
  });

  const pathD = points.length > 0 ? `M ${points.join(' L ')}` : '';
  const areaD = points.length > 0 ? `${pathD} L ${padding + (history.length - 1) * ((width - padding * 2) / (history.length - 1 || 1))},${height - padding} L ${padding},${height - padding} Z` : '';

  const current = livePingData?.latency ?? (history[history.length - 1] || 0);
  const avg = Math.round(history.reduce((a, b) => a + b, 0) / (history.length || 1));
  const min = Math.min(...history);
  const max = Math.max(...history);

  return (
    <div
      className="flex flex-col items-center p-2 rounded-lg relative z-10 transition-all duration-200 app-no-drag w-full"
      style={{
        background: 'var(--ui-panel-transparent)',
        borderColor: 'var(--ui-border)',
      }}
    >
      <div className="flex justify-between items-center w-full text-xs font-medium px-1 mb-2" style={{ color: 'var(--ui-text)' }}>
        <span className="flex items-center gap-2 font-semibold">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          </span>
          Задержка обхода
        </span>
        <div className="flex items-center gap-2">
          <span className="text-[11px] font-mono" style={{ color: 'var(--ui-text-dim)' }}>
            ср: {avg} мс (мин: {min} / макс: {max})
          </span>
          <span className="text-xs font-mono font-bold px-2 py-0.5 rounded-md bg-[var(--ui-surface-elevated)] text-[var(--ui-text)] border border-[var(--ui-border)]">
            {current} мс
          </span>
        </div>
      </div>
      <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible">
        <defs>
          <linearGradient id="pingGradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--ui-text-muted)" stopOpacity="0.25" />
            <stop offset="100%" stopColor="var(--ui-text-muted)" stopOpacity="0.0" />
          </linearGradient>
        </defs>
        <path d={areaD} fill="url(#pingGradient)" />
        <path d={pathD} fill="none" stroke="var(--ui-text-muted)" strokeWidth="2.0" strokeLinecap="round" strokeLinejoin="round" />
        {history.map((_, idx) => {
          const [x, y] = points[idx].split(',').map(Number);
          return (
            <circle
              key={idx}
              cx={x}
              cy={y}
              r={idx === history.length - 1 ? 3.5 : 1.5}
              fill="var(--ui-text)"
              stroke="var(--ui-panel)"
              strokeWidth={idx === history.length - 1 ? 2 : 1}
            />
          );
        })}
      </svg>
    </div>
  );
};
