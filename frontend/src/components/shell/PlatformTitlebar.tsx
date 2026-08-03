import React from 'react';
import { UIShield } from '../icons';
import { windowService } from '../../services/window';

interface PlatformTitlebarProps {
  platform: string;
  appVersion: string;
}

export const PlatformTitlebar: React.FC<PlatformTitlebarProps> = ({
  platform,
  appVersion,
}) => {
  const isMac = platform === 'darwin';
  const isWin = platform === 'windows' || platform === 'win32';
  const versionDisplay = appVersion ? `v${appVersion}` : 'v0.3.0';

  return (
    <div className="flex-none h-11 flex items-center justify-between px-4 z-10 border-b border-[var(--ui-border)] bg-[var(--ui-panel)] app-drag select-none">
      {isMac ? (
        <>
          <div className="flex items-center gap-2 app-no-drag">
            <span
              onClick={() => windowService.hideToTray()}
              className="w-3 h-3 rounded-full bg-red-500 hover:opacity-80 cursor-pointer inline-block"
              title="Закрыть в трей"
            />
            <span
              onClick={() => windowService.minimise()}
              className="w-3 h-3 rounded-full bg-amber-500 hover:opacity-80 cursor-pointer inline-block"
              title="Свернуть"
            />
            <span className="w-3 h-3 rounded-full bg-emerald-500 opacity-40 inline-block" />
          </div>
          <div className="flex items-center gap-2 font-semibold text-xs text-[var(--ui-text)]">
            <UIShield className="w-4 h-4" />
            <span>UNBOUND</span>
            <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">{versionDisplay}</span>
          </div>
          <div className="text-[10px] font-mono text-[var(--ui-text-dim)]">ROOT ✓</div>
        </>
      ) : isWin ? (
        <>
          <div className="flex items-center gap-2 font-semibold text-xs text-[var(--ui-text)]">
            <UIShield className="w-4 h-4" />
            <span>UNBOUND Refresh</span>
            <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">{versionDisplay}</span>
          </div>
          <div className="flex items-center gap-3 text-xs text-[var(--ui-text-muted)] app-no-drag">
            <button onClick={() => windowService.minimise()} className="hover:text-[var(--ui-text)] px-1" title="Свернуть">
              ─
            </button>
            <button onClick={() => windowService.hideToTray()} className="hover:text-red-400 px-1" title="Закрыть">
              ✕
            </button>
          </div>
        </>
      ) : (
        /* Neutral Linux Titlebar */
        <>
          <div className="flex items-center gap-2 font-semibold text-xs text-[var(--ui-text)]">
            <UIShield className="w-4 h-4" />
            <span>UNBOUND</span>
            <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">{versionDisplay}</span>
          </div>
          <div className="flex items-center gap-2 app-no-drag">
            <button
              onClick={() => windowService.minimise()}
              className="w-5 h-5 rounded bg-[var(--ui-border)] hover:bg-[var(--ui-border-strong)] text-xs flex items-center justify-center"
              title="Свернуть"
            >
              ─
            </button>
            <button
              onClick={() => windowService.hideToTray()}
              className="w-5 h-5 rounded bg-[var(--ui-border)] hover:bg-[var(--ui-border-strong)] text-xs flex items-center justify-center"
              title="Закрыть"
            >
              ✕
            </button>
          </div>
        </>
      )}
    </div>
  );
};

