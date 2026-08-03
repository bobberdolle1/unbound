import React from 'react';

interface PrivilegesModalProps {
  privilegeError: string;
  platform: string;
  onClose: () => void;
}

export const PrivilegesModal: React.FC<PrivilegesModalProps> = ({
  privilegeError,
  platform,
  onClose,
}) => {
  if (!privilegeError) return null;

  return (
    <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 app-no-drag">
      <div className="w-full max-w-sm bg-[var(--ui-panel)] border border-red-500/40 rounded-2xl p-5 shadow-2xl space-y-4">
        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-full bg-red-500/20 text-red-400 flex items-center justify-center font-bold text-lg flex-shrink-0 border border-red-500/30">
            !
          </div>
          <div>
            <h3 className="text-base font-semibold text-[var(--ui-text)]">
              {platform === 'darwin' ? 'Требуются права root (sudo)' : 'Требуются права администратора'}
            </h3>
            <p className="text-xs text-[var(--ui-text-muted)] mt-1">{privilegeError}</p>
          </div>
        </div>
        <button
          onClick={onClose}
          className="w-full py-2.5 text-xs font-semibold rounded-xl bg-red-600 text-white hover:bg-red-700 transition-colors"
        >
          Понятно
        </button>
      </div>
    </div>
  );
};
