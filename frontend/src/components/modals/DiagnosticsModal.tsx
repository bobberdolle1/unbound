import React from 'react';
import { cn } from '../../lib/cn';
import { UITerminal, UISpinner, UIX } from '../icons';

interface DiagnosticResult {
  Component: string;
  Status: string;
  Details: string;
  IsError?: boolean;
}

interface DiagnosticsModalProps {
  isOpen: boolean;
  isRunning: boolean;
  results: DiagnosticResult[];
  onClose: () => void;
}

export const DiagnosticsModal: React.FC<DiagnosticsModalProps> = ({
  isOpen,
  isRunning,
  results,
  onClose,
}) => {
  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 app-no-drag"
      onClick={onClose}
    >
      <div
        className="w-full max-w-sm bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl flex flex-col max-h-[80vh] p-4 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[var(--ui-border)] pb-2 mb-3">
          <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ui-text)]">
            <UITerminal className="w-4 h-4" />
            <span>Проверка системы</span>
          </div>
          <button
            onClick={onClose}
            className="text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] text-xs"
          >
            <UIX className="w-4 h-4" />
          </button>
        </div>

        <div className="overflow-y-auto space-y-2 flex-1 pr-1">
          {isRunning ? (
            <div className="flex flex-col items-center justify-center py-8 gap-3">
              <UISpinner className="w-8 h-8 animate-spin" />
              <span className="text-xs text-[var(--ui-text-muted)]">Выполняется диагностика...</span>
            </div>
          ) : (
            results.map((res, idx) => (
              <div
                key={idx}
                className="p-3 border rounded-xl bg-[var(--ui-surface-elevated)] border-[var(--ui-border)] text-xs"
              >
                <div className="flex justify-between items-start mb-1 font-semibold text-[var(--ui-text)]">
                  <span>{res.Component}</span>
                  <span
                    className={cn(
                      'text-[10px] px-1.5 py-0.5 rounded uppercase font-mono',
                      res.IsError ? 'bg-red-500/20 text-red-400' : 'bg-emerald-500/20 text-emerald-400'
                    )}
                  >
                    {res.Status}
                  </span>
                </div>
                <p className="text-[11px] text-[var(--ui-text-muted)]">{res.Details}</p>
              </div>
            ))
          )}
        </div>

        <div className="pt-3 border-t border-[var(--ui-border)] mt-2">
          <button onClick={onClose} className="btn-ui-primary w-full">
            Закрыть
          </button>
        </div>
      </div>
    </div>
  );
};
