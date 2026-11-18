import React from 'react';

interface ConflictModalProps {
  conflicts: string[];
  onIgnore: () => void;
  onKillConflicts: () => void;
}

const EXTERNAL_VPN_OR_MANUAL_PATTERNS = [
  /openvpn/i,
  /warp/i,
  /expressvpn/i,
  /nordvpn/i,
  /xray/i,
  /happ/i,
  /sing-box/i,
  /v2ray/i,
  /clash/i,
  /hiddify/i,
  /nekoray/i,
  /shadowsocks/i,
  /wireguard/i,
  /wg-quick/i,
  /tailscale/i,
  /\bvpn\b/i,
  /\bproxy\b/i,
  /прокси/i,
  /mangle/i,
  /nfqueue/i,
];

export function isExternalManualConflict(item: string): boolean {
  return EXTERNAL_VPN_OR_MANUAL_PATTERNS.some((pattern) => pattern.test(item));
}

export const ConflictModal: React.FC<ConflictModalProps> = ({
  conflicts,
  onIgnore,
  onKillConflicts,
}) => {
  if (!conflicts || conflicts.length === 0) return null;

  const killableConflicts = conflicts.filter((c) => !isExternalManualConflict(c));
  const manualConflicts = conflicts.filter((c) => isExternalManualConflict(c));

  return (
    <div className="fixed inset-0 z-[9998] flex items-center justify-center bg-black/70 backdrop-blur-sm p-4 app-no-drag">
      <div className="w-full max-w-md bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl p-5 shadow-2xl space-y-4">
        <div className="flex items-start gap-3">
          <div className="w-9 h-9 rounded-full bg-amber-500/20 text-amber-400 flex items-center justify-center font-bold text-lg flex-shrink-0 border border-amber-500/30">
            !
          </div>
          <div className="flex-1 min-w-0 space-y-3">
            <h3 className="text-base font-semibold text-[var(--ui-text)]">Обнаружены конфликты</h3>

            {killableConflicts.length > 0 && (
              <div className="rounded-xl border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 space-y-1.5">
                <div className="text-[11px] font-semibold text-[var(--ui-text)]">
                  DPI-процессы (могут быть завершены автоматически):
                </div>
                <div className="text-xs text-[var(--ui-text-muted)] space-y-1">
                  {killableConflicts.map((c, i) => (
                    <div key={i}>{c}</div>
                  ))}
                </div>
              </div>
            )}

            {manualConflicts.length > 0 && (
              <div className="rounded-xl border border-amber-500/35 bg-amber-500/10 p-3 space-y-1.5">
                <div className="text-[11px] font-semibold text-amber-400">
                  Внешние VPN / прокси (требуют ручного закрытия):
                </div>
                <div className="text-xs text-[var(--ui-text)] space-y-1">
                  {manualConflicts.map((c, i) => (
                    <div key={i}>{c}</div>
                  ))}
                </div>
                <p className="text-[11px] text-[var(--ui-text-muted)] leading-snug pt-1">
                  UNBOUND не останавливает сторонние VPN и прокси принудительно, чтобы не оборвать активные туннели. При сбоях закройте их вручную.
                </p>
              </div>
            )}
          </div>
        </div>

        <div className="flex gap-2 pt-2">
          <button
            onClick={onIgnore}
            className="flex-1 py-2 px-3 text-xs font-semibold rounded-xl border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]"
          >
            Игнорировать
          </button>
          {killableConflicts.length > 0 ? (
            <button
              onClick={onKillConflicts}
              className="flex-1 py-2 px-3 text-xs font-semibold rounded-xl bg-[var(--ui-btn-primary-bg)] text-[var(--ui-btn-primary-fg)]"
            >
              {manualConflicts.length > 0 ? 'Завершить DPI-процессы' : 'Завершить все'}
            </button>
          ) : (
            <button
              onClick={onIgnore}
              className="flex-1 py-2 px-3 text-xs font-semibold rounded-xl bg-[var(--ui-btn-primary-bg)] text-[var(--ui-btn-primary-fg)]"
            >
              Понятно, закрыть окно
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
