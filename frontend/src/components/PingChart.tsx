export const PingChart = ({ history }: { history: number[] }) => {
  if (history.length === 0) return null;
  const maxVal = Math.max(...history, 100);
  const minVal = 0;
  const range = maxVal - minVal;
  const width = 300;
  const height = 50;
  const padding = 5;

  const points = history.map((val, idx) => {
    const x = padding + (idx / (history.length - 1 || 1)) * (width - padding * 2);
    const y = height - padding - ((val - minVal) / range) * (height - padding * 2);
    return `${x},${y}`;
  });

  const pathD = points.length > 0 ? `M ${points.join(' L ')}` : '';
  const areaD = points.length > 0 ? `${pathD} L ${padding + (history.length - 1) * ((width - padding * 2) / (history.length - 1 || 1))},${height - padding} L ${padding},${height - padding} Z` : '';

  return (
    <div className="flex flex-col items-center mt-3 p-3 bg-white border-2 border-gray-800 rounded-xl relative z-10 shadow-[2px_2px_0_#222] animate-fade-in">
      <div className="flex justify-between w-full text-xs font-bold text-gray-700 px-1 mb-2">
        <span className="flex items-center gap-1.5">
          <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
          Задержка DPI обхода
        </span>
        <span>{history[history.length - 1]} мс</span>
      </div>
      <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} className="overflow-visible">
        <defs>
          <linearGradient id="pingGradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--ui-accent, #3b82f6)" stopOpacity="0.4" />
            <stop offset="100%" stopColor="var(--ui-accent, #3b82f6)" stopOpacity="0.0" />
          </linearGradient>
        </defs>
        <path d={areaD} fill="url(#pingGradient)" />
        <path d={pathD} fill="none" stroke="var(--ui-accent, #3b82f6)" strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />
        {history.map((_, idx) => {
          const [x, y] = points[idx].split(',').map(Number);
          return (
            <circle
              key={idx}
              cx={x}
              cy={y}
              r={idx === history.length - 1 ? 5 : 2.5}
              fill="var(--ui-accent, #3b82f6)"
              stroke="white"
              strokeWidth={idx === history.length - 1 ? 2.5 : 1}
            />
          );
        })}
      </svg>
    </div>
  );
};
