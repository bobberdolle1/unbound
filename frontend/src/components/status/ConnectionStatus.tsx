import React from 'react';
import { cn } from '../../lib/cn';
import { UISpinner } from '../icons';

interface ConnectionStatusProps {
  status: string;
  onToggleEngine: () => void;
}

export const ConnectionStatus: React.FC<ConnectionStatusProps> = ({ status, onToggleEngine }) => {
  const isRunning = status === 'Running';
  const isConnecting = status === 'Connecting';

  let statusDotClass = 'bg-gray-500';
  let statusText = 'Служба остановлена';

  if (isRunning) {
    statusDotClass = 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)] animate-pulse';
    statusText = 'Обход активен';
  } else if (isConnecting) {
    statusDotClass = 'bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.5)] animate-ping';
    statusText = 'Инициализация драйвера...';
  } else if (status === 'Error') {
    statusDotClass = 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]';
    statusText = 'Ошибка драйвера';
  }

  return (
    <div className="flex flex-col items-center gap-3">
      {/* Central Connect Button */}
      <button
        onClick={onToggleEngine}
        disabled={isConnecting}
        className={cn(
          'w-28 h-28 rounded-full flex flex-col items-center justify-center transition-all duration-200 shadow-lg relative group cursor-pointer border-2',
          isRunning
            ? 'bg-emerald-500 text-white border-emerald-400 hover:bg-emerald-600 active:scale-95'
            : isConnecting
            ? 'bg-[var(--ui-surface)] text-[var(--ui-text-muted)] border-[var(--ui-border)] cursor-not-allowed'
            : 'bg-[var(--ui-text)] text-[var(--ui-bg)] border-[var(--ui-text)] hover:opacity-90 active:scale-95'
        )}
      >
        {isConnecting ? (
          <UISpinner className="w-8 h-8 animate-spin text-[var(--ui-text-muted)]" />
        ) : (
          <div className="flex flex-col items-center gap-1">
            <span className="text-sm font-bold tracking-wider">
              {isRunning ? 'ОТКЛЮЧИТЬ' : 'ВКЛЮЧИТЬ'}
            </span>
            <span className="text-[10px] opacity-75 font-mono">
              {isRunning ? 'L3/L4 Active' : 'Start Engine'}
            </span>
          </div>
        )}
      </button>

      {/* Status LED Indicator */}
      <div className="flex items-center gap-2 px-3 py-1 rounded-full bg-[var(--ui-surface)] border border-[var(--ui-border)]">
        <span className={cn('w-2 h-2 rounded-full', statusDotClass)} />
        <span className="text-xs font-medium text-[var(--ui-text)]">{statusText}</span>
      </div>
    </div>
  );
};
