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
  const latestLine = logs.length > 0 ? logs[logs.length - 1] : '';

  if (!isExpanded) {
    return (
      <div className="border-t border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] text-[var(--ui-text)] shrink-0 font-mono text-xs app-no-drag">
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={false}
          aria-label="Развернуть журнал"
          title="Развернуть системный журнал (Ctrl+J)"
          className="w-full flex items-center justify-between gap-3 px-3 py-1.5 hover:bg-[var(--ui-surface-hover)] transition-colors text-left cursor-pointer"
        >
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <UITerminal className="w-3.5 h-3.5 text-[var(--ui-text-muted)] shrink-0" />
            <span className="font-semibold text-[11px] text-[var(--ui-text-muted)] shrink-0">
              Журнал ({logs.length})
            </span>
            <span className="text-[11px] text-[var(--ui-text-dim)] truncate">
              {latestLine ? formatLog(latestLine) : 'Ожидание событий службы...'}
            </span>
          </div>
          <div className="flex items-center gap-2 shrink-0 text-[10px] text-[var(--ui-text-muted)]">
            <span className="px-1.5 py-0.5 rounded border border-[var(--ui-border)] bg-[var(--ui-panel)]">
              Ctrl+J
            </span>
            <span>▲ Развернуть</span>
          </div>
        </button>
      </div>
    );
  }

  return (
    <div className="h-44 border-t border-[var(--ui-border)] bg-[var(--ui-bg)] text-[var(--ui-text)] flex flex-col shrink-0 font-mono text-xs app-no-drag">
      <div className="flex items-center justify-between px-3 py-1.5 border-b border-[var(--ui-border)] bg-[var(--ui-surface-elevated)]">
        <div className="flex items-center gap-2">
          <UITerminal className="w-3.5 h-3.5 text-[var(--ui-text-muted)]" />
          <span className="font-semibold text-xs text-[var(--ui-text-muted)]">
            Системный журнал L3/L4 ({logs.length})
          </span>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onExportLogs}
            className="px-2 py-0.5 rounded text-[10px] bg-[var(--ui-btn-sec-bg)] hover:bg-[var(--ui-surface-hover)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] border border-[var(--ui-border)] transition-colors"
          >
            Сохранить
          </button>
          <button
            type="button"
            onClick={onToggle}
            aria-expanded={true}
            aria-label="Свернуть журнал"
            title="Свернуть системный журнал (Ctrl+J)"
            className="p-1 rounded hover:bg-[var(--ui-surface-hover)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] transition-colors"
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
            <div key={i} className="leading-relaxed hover:bg-[var(--ui-surface-hover)] px-1 rounded">
              {formatLog(log)}
            </div>
          ))
        )}
      </div>
    </div>
  );
};
