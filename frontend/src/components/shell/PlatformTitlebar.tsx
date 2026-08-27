import React from 'react';
import { UILogo, UIMinimize, UIMaximize, UIX, UICheck } from '../icons';
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
  const versionDisplay = appVersion ? `v${appVersion}` : 'v0.3.2';

  return (
    <div
      onDoubleClick={() => windowService.toggleMaximise()}
      style={{ '--wails-draggable': 'drag' }}
      className="flex-none h-11 flex items-center justify-between px-4 z-10 border-b border-[var(--ui-border)] bg-[var(--ui-panel)] app-drag select-none"
    >
      {isMac ? (
        <>
          <div className="flex items-center gap-2 app-no-drag" style={{ '--wails-draggable': 'no-drag' }}>
            <span
              onClick={(e) => {
                e.stopPropagation();
                windowService.hideToTray();
              }}
              className="w-3 h-3 rounded-full bg-red-500 hover:opacity-80 cursor-pointer inline-block"
              title="Закрыть в трей"
            />
            <span
              onClick={(e) => {
                e.stopPropagation();
                windowService.minimise();
              }}
              className="w-3 h-3 rounded-full bg-amber-500 hover:opacity-80 cursor-pointer inline-block"
              title="Свернуть"
            />
            <span
              onClick={(e) => {
                e.stopPropagation();
                windowService.toggleMaximise();
              }}
              className="w-3 h-3 rounded-full bg-emerald-500 hover:opacity-80 cursor-pointer inline-block"
              title="Развернуть / Восстановить"
            />
          </div>
          <div className="flex items-center gap-2 font-semibold text-xs text-[var(--ui-text)]" style={{ '--wails-draggable': 'drag' }}>
            <UILogo className="w-4 h-4 text-[var(--ui-text)]" />
            <span>UNBOUND</span>
            <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">{versionDisplay}</span>
          </div>
          <div className="flex items-center gap-1 text-[10px] font-mono text-[var(--ui-text-dim)]" style={{ '--wails-draggable': 'drag' }}>
            <span>ROOT</span>
            <UICheck className="w-3 h-3 text-emerald-500" />
          </div>
        </>
      ) : isWin ? (
        <>
          <div className="flex items-center gap-2.5 font-semibold text-xs text-[var(--ui-text)]" style={{ '--wails-draggable': 'drag' }}>
            <UILogo className="w-4 h-4 text-[var(--ui-text)]" />
            <span>UNBOUND Refresh</span>
            <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">{versionDisplay}</span>
            <div className="flex items-center gap-1 text-[10px] font-mono text-[var(--ui-text-dim)] ml-2" style={{ '--wails-draggable': 'drag' }}>
              <span>ADMIN</span>
              <UICheck className="w-3 h-3 text-emerald-500" />
            </div>
          </div>
          <div className="flex items-center gap-1 text-xs text-[var(--ui-text-muted)] app-no-drag" style={{ '--wails-draggable': 'no-drag' }}>
            <button
              onClick={() => windowService.minimise()}
              style={{ '--wails-draggable': 'no-drag' }}
              className="p-1.5 rounded hover:bg-[var(--ui-surface-hover)] hover:text-[var(--ui-text)] transition-colors"
              title="Свернуть"
            >
              <UIMinimize className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => windowService.toggleMaximise()}
              style={{ '--wails-draggable': 'no-drag' }}
              className="p-1.5 rounded hover:bg-[var(--ui-surface-hover)] hover:text-[var(--ui-text)] transition-colors"
              title="Развернуть"
            >
              <UIMaximize className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => windowService.hideToTray()}
              style={{ '--wails-draggable': 'no-drag' }}
              className="p-1.5 rounded hover:bg-red-500/20 hover:text-red-400 transition-colors"
              title="Закрыть"
            >
              <UIX className="w-3.5 h-3.5" />
            </button>
          </div>
        </>
      ) : (
        /* Neutral Linux Titlebar */
        <>
          <div className="flex items-center gap-2 font-semibold text-xs text-[var(--ui-text)]" style={{ '--wails-draggable': 'drag' }}>
            <UILogo className="w-4 h-4 text-[var(--ui-text)]" />
            <span>UNBOUND</span>
            <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">{versionDisplay}</span>
          </div>
          <div className="flex items-center gap-2 app-no-drag" style={{ '--wails-draggable': 'no-drag' }}>
            <button
              onClick={() => windowService.minimise()}
              style={{ '--wails-draggable': 'no-drag' }}
              className="w-5 h-5 rounded bg-[var(--ui-border)] hover:bg-[var(--ui-border-strong)] text-xs flex items-center justify-center"
              title="Свернуть"
            >
              <UIMinimize className="w-3 h-3" />
            </button>
            <button
              onClick={() => windowService.toggleMaximise()}
              style={{ '--wails-draggable': 'no-drag' }}
              className="w-5 h-5 rounded bg-[var(--ui-border)] hover:bg-[var(--ui-border-strong)] text-xs flex items-center justify-center"
              title="Развернуть"
            >
              <UIMaximize className="w-3 h-3" />
            </button>
            <button
              onClick={() => windowService.hideToTray()}
              style={{ '--wails-draggable': 'no-drag' }}
              className="w-5 h-5 rounded bg-[var(--ui-border)] hover:bg-[var(--ui-border-strong)] text-xs flex items-center justify-center"
              title="Закрыть"
            >
              <UIX className="w-3 h-3" />
            </button>
          </div>
        </>
      )}
    </div>
  );
};
