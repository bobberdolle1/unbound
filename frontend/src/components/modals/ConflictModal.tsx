import React from 'react';

interface ConflictModalProps {
  conflicts: string[];
  onIgnore: () => void;
  onKillConflicts: () => void;
}

export const ConflictModal: React.FC<ConflictModalProps> = ({
  conflicts,
  onIgnore,
  onKillConflicts,
}) => {
  if (!conflicts || conflicts.length === 0) return null;

  return (
    <div className="fixed inset-0 z-[9998] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 app-no-drag">
      <div className="w-full max-w-sm bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl p-5 shadow-2xl space-y-4">
        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-full bg-amber-500/20 text-amber-400 flex items-center justify-center font-bold text-lg flex-shrink-0 border border-amber-500/30">
            !
          </div>
          <div>
            <h3 className="text-base font-semibold text-[var(--ui-text)]">Обнаружены конфликты</h3>
            <div className="text-xs text-[var(--ui-text-muted)] mt-1 space-y-1">
              {conflicts.map((c, i) => (
                <div key={i}>{c}</div>
              ))}
            </div>
          </div>
        </div>
        <div className="flex gap-2 pt-2">
          <button
            onClick={onIgnore}
            className="flex-1 py-2 text-xs font-semibold rounded-xl border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]"
          >
            Игнорировать
          </button>
          <button
            onClick={onKillConflicts}
            className="flex-1 py-2 text-xs font-semibold rounded-xl bg-[var(--ui-btn-primary-bg)] text-[var(--ui-btn-primary-fg)]"
          >
            Завершить все
          </button>
        </div>
      </div>
    </div>
  );
};
