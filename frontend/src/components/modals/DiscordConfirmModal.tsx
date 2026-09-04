import React from 'react';

interface DiscordConfirmModalProps {
  isOpen: boolean;
  runningProcesses: string[];
  onCancel: () => void;
  onConfirm: () => void;
}

export const DiscordConfirmModal: React.FC<DiscordConfirmModalProps> = ({
  isOpen,
  runningProcesses,
  onCancel,
  onConfirm,
}) => {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-[9998] flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 app-no-drag">
      <div className="w-full max-w-sm bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl p-5 shadow-2xl space-y-4">
        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-full bg-amber-500/20 text-amber-400 flex items-center justify-center font-bold text-lg flex-shrink-0 border border-amber-500/30">
            !
          </div>
          <div>
            <h3 className="text-base font-semibold text-[var(--ui-text)]">Очистить кэш Discord?</h3>
            <p className="text-xs text-[var(--ui-text-muted)] mt-1.5 leading-relaxed">
              Для полной очистки UNBOUND закроет запущенные процессы Discord ({runningProcesses.length > 0 ? runningProcesses.join(', ') : 'Discord.exe'}).
            </p>
            <p className="text-xs text-[var(--ui-text-muted)] mt-1.5 leading-relaxed">
              Текущий голосовой звонок будет прерван, а несохранённый ввод сообщения может быть потерян.
            </p>
            <p className="text-[11px] text-emerald-400/90 mt-2 font-mono">
              ✓ Аккаунт, настройки, сессия и закреплённые ярлыки не затрагиваются.
            </p>
          </div>
        </div>

        <div className="flex gap-2 pt-2 border-t border-[var(--ui-border)]">
          <button
            onClick={onCancel}
            className="flex-1 py-2 text-xs font-semibold rounded-xl border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] transition-colors"
          >
            Отмена
          </button>
          <button
            onClick={onConfirm}
            className="flex-1 py-2 text-xs font-semibold rounded-xl bg-red-600/20 text-red-400 border border-red-500/30 hover:bg-red-600/30 transition-colors"
          >
            Закрыть и очистить
          </button>
        </div>
      </div>
    </div>
  );
};
