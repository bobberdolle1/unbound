export const PingChart = ({ history }: { history: number[] }) => {
  if (history.length === 0) return null;
  const maxVal = Math.max(...history, 100);
  const minVal = 0;
  const range = maxVal - minVal;
  const width = 300;
  const height = 48;
  const padding = 4;

  const points = history.map((val, idx) => {
    const x = padding + (idx / (history.length - 1 || 1)) * (width - padding * 2);
    const y = height - padding - ((val - minVal) / range) * (height - padding * 2);
    return `${x},${y}`;
  });

  const pathD = points.length > 0 ? `M ${points.join(' L ')}` : '';
  const areaD = points.length > 0 ? `${pathD} L ${padding + (history.length - 1) * ((width - padding * 2) / (history.length - 1 || 1))},${height - padding} L ${padding},${height - padding} Z` : '';

  const current = history[history.length - 1] || 0;
  const avg = Math.round(history.reduce((a, b) => a + b, 0) / (history.length || 1));
  const min = Math.min(...history);
  const max = Math.max(...history);

  return (
    <div
      className="flex flex-col items-center mt-3 p-3 border rounded-xl relative z-10 transition-all duration-200 app-no-drag"
      style={{
        background: 'var(--ui-panel-transparent)',
        borderColor: 'var(--ui-border)',
        backdropFilter: 'blur(var(--ui-blur))',
        boxShadow: '0 4px 20px rgba(0, 0, 0, 0.15)'
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
          <span className="text-xs font-mono font-bold px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            {current} мс
          </span>
        </div>
      </div>
      <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible">
        <defs>
          <linearGradient id="pingGradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--ui-accent, #38bdf8)" stopOpacity="0.3" />
            <stop offset="100%" stopColor="var(--ui-accent, #38bdf8)" stopOpacity="0.0" />
          </linearGradient>
        </defs>
        <path d={areaD} fill="url(#pingGradient)" />
        <path d={pathD} fill="none" stroke="var(--ui-accent, #38bdf8)" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" />
        {history.map((_, idx) => {
          const [x, y] = points[idx].split(',').map(Number);
          return (
            <circle
              key={idx}
              cx={x}
              cy={y}
              r={idx === history.length - 1 ? 4 : 1.5}
              fill="var(--ui-accent, #38bdf8)"
              stroke="var(--ui-panel, #090c10)"
              strokeWidth={idx === history.length - 1 ? 2 : 1}
            />
          );
        })}
      </svg>
    </div>
  );
};
