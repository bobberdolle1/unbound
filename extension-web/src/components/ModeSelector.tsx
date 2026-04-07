import { type ExtensionMode } from '../types';

interface ModeSelectorProps {
  mode: ExtensionMode;
  onModeChange: (mode: ExtensionMode) => void;
  disabled?: boolean;
}

export function ModeSelector({ mode, onModeChange, disabled = false }: ModeSelectorProps) {
  return (
    <div className="space-y-2">
      <label className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wide">
        Mode
      </label>
      <div className="grid grid-cols-2 gap-2 p-1 rounded-lg bg-[var(--bg-surface)] border border-[var(--border-color)]">
        <button
          onClick={() => onModeChange('companion')}
          disabled={disabled}
          className={`
            px-3 py-2 rounded-md text-sm font-medium transition-all duration-200
            ${mode === 'companion'
              ? 'bg-[var(--color-primary)] text-white shadow-sm'
              : 'text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--border-color)]'
            }
            ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
          `}
        >
          <div className="flex flex-col items-center gap-1">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <span>Companion</span>
          </div>
        </button>
        <button
          onClick={() => onModeChange('standalone')}
          disabled={disabled}
          className={`
            px-3 py-2 rounded-md text-sm font-medium transition-all duration-200
            ${mode === 'standalone'
              ? 'bg-[var(--color-primary)] text-white shadow-sm'
              : 'text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--border-color)]'
            }
            ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
          `}
        >
          <div className="flex flex-col items-center gap-1">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span>Standalone</span>
          </div>
        </button>
      </div>
      <p className="text-xs text-[var(--text-muted)]">
        {mode === 'companion' 
          ? 'Uses local Unbound Desktop daemon' 
          : 'Routes through external proxy server'
        }
      </p>
    </div>
  );
}
