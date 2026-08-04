import React from 'react';
import { UITerminal, UIX } from '../icons';
import { formatLog } from '../../lib/format';

interface LogJournalDrawerProps {
  logs: string[];
  isExpanded: boolean;
  onToggle: () => void;
  onExportLogs: () => void;
}

export const LogJournalDrawer: React.FC<LogJournalDrawerProps> = ({
  logs,
  isExpanded,
  onToggle,
  onExportLogs,
}) => {
  if (!isExpanded) return null;

  return (
    <div className="h-44 border-t border-[var(--ui-border)] bg-[#090b0e] text-[var(--ui-text)] flex flex-col shrink-0 font-mono text-xs">
      <div className="flex items-center justify-between px-3 py-1.5 border-b border-[var(--ui-border)] bg-[#101318]">
        <div className="flex items-center gap-2">
          <UITerminal className="w-3.5 h-3.5 text-[var(--ui-text-muted)]" />
          <span className="font-semibold text-xs text-[var(--ui-text-muted)]">Системный журнал L3/L4 ({logs.length})</span>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onExportLogs}
            className="px-2 py-0.5 rounded text-[10px] bg-[var(--ui-surface)] hover:bg-[var(--ui-surface-hover)] text-[var(--ui-text-muted)] border border-[var(--ui-border)] transition-colors"
          >
            Сохранить
          </button>
          <button
            onClick={onToggle}
            className="p-1 rounded hover:bg-[var(--ui-surface-hover)] text-[var(--ui-text-muted)] transition-colors"
          >
            <UIX className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div className="flex-1 p-3 overflow-y-auto space-y-1 font-mono text-[11px] select-text">
        {logs.length === 0 ? (
          <div className="text-[var(--ui-text-dim)] italic">Журнал пуст. Ожидание сообщений службы...</div>
        ) : (
          logs.map((log, i) => (
            <div key={i} className="leading-relaxed hover:bg-white/5 px-1 rounded">
              {formatLog(log)}
            </div>
          ))
        )}
      </div>
    </div>
  );
};
