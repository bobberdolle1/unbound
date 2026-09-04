import React from 'react';
import { UICheckbox } from '../UICheckbox';
import { UISelect } from '../UISelect';
import { UIGear, UIShield, UITerminal, UIX } from '../icons';
import { QuitApp } from '../../../wailsjs/go/main/App';

export interface AppSettings {
  autoStart: boolean;
  startMinimized: boolean;
  autoStartProfile: boolean;
  defaultProfile: string;
  startupProfileMode: string;
  gameFilter: boolean;
  autoUpdateEnabled: boolean;
  showLogs: boolean;
  enableTCPTimestamps: boolean;
  discordCacheAutoClean: boolean;
  secureDns: boolean;
  autoReconnect: boolean;
  favoriteProfiles: string[];
  autoTuneTargets?: string[];
  diagnosticsMode?: string;
  autoUpdatePolicy?: string;
}

interface SettingsViewProps {
  settings: AppSettings;
  setSettings: React.Dispatch<React.SetStateAction<AppSettings>>;
  theme: string;
  setThemeState: (theme: string) => void;
  startupProfiles: string[];
  handleRunDiagnostics: () => void;
  handleVerifyAssets: () => void;
  isVerifyingAssets: boolean;
  handleDiagnosticReport: () => void;
  isGeneratingReport: boolean;
  handleUpdateHostlists: () => void;
  isUpdatingHostlists: boolean;
  handleClearCache: () => void;
  handleKillWinws2: () => void;
}

export const SettingsView: React.FC<SettingsViewProps> = ({
  settings,
  setSettings,
  theme,
  setThemeState,
  startupProfiles,
  handleRunDiagnostics,
  handleVerifyAssets,
  isVerifyingAssets,
  handleDiagnosticReport,
  isGeneratingReport,
  handleUpdateHostlists,
  isUpdatingHostlists,
  handleClearCache,
  handleKillWinws2,
}) => {
  return (
    <div className="flex-1 flex flex-col gap-4">
      <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 space-y-4">
        <h3 className="text-sm font-semibold text-[var(--ui-text)]">Параметры системы</h3>

        <UICheckbox
          id="autoStart"
          label="Автозапуск"
          desc="Запускать Unbound при старте системы"
          checked={settings.autoStart}
          onChange={() => setSettings({ ...settings, autoStart: !settings.autoStart })}
        />

        <UICheckbox
          id="startMinimized"
          label="Тихий старт"
          desc="Запускать свёрнутым в системный трей"
          checked={settings.startMinimized}
          onChange={() => setSettings({ ...settings, startMinimized: !settings.startMinimized })}
        />

        <UICheckbox
          id="startMinimized"
          label="Тихий старт"
          desc="Запускать скрыто в системном трее, без окна"
          checked={settings.startMinimized}
          onChange={() => setSettings({ ...settings, startMinimized: !settings.startMinimized })}
        />

        <UICheckbox
          id="autoStartProfile"
          label="Автовключение профиля"
          desc="При запуске Unbound автоматически включать выбранный профиль"
          checked={settings.autoStartProfile}
          onChange={() => setSettings({ ...settings, autoStartProfile: !settings.autoStartProfile })}
        />

        <div className={`flex flex-col gap-1.5 ${settings.autoStartProfile ? '' : 'opacity-50 pointer-events-none'}`}>
          <span className="text-xs font-semibold text-[var(--ui-text-muted)]">Профиль для автовключения</span>
          <UISelect
            value={settings.startupProfileMode}
            options={['Последний использованный', 'Автоподбор', ...startupProfiles]}
            onChange={(val: string) => setSettings({ ...settings, startupProfileMode: val })}
            up={false}
          />
        </div>

        {/* Theme Selector */}
        <div className="flex flex-col gap-1.5 pt-2 border-t border-[var(--ui-border)]">
          <span className="text-xs font-semibold text-[var(--ui-text)]">Тема оформления</span>
          <UISelect
            value={
              theme === 'monolith'
                ? 'Monolith (Dark)'
                : theme === 'paper'
                ? 'Paper (Warm Light)'
                : theme === 'graphite'
                ? 'Graphite (Steel Dark)'
                : 'Monolith (Dark)'
            }
            options={['Monolith (Dark)', 'Paper (Warm Light)', 'Graphite (Steel Dark)']}
            onChange={(val: string) => {
              const themeMap: Record<string, string> = {
                'Monolith (Dark)': 'monolith',
                'Paper (Warm Light)': 'paper',
                'Graphite (Steel Dark)': 'graphite',
              };
              setThemeState(themeMap[val] || 'monolith');
            }}
            up={false}
          />
        </div>

        {/* Component Versions & Updates */}
        <div className="space-y-2 pt-3 border-t border-[var(--ui-border)]">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-[var(--ui-text-muted)] uppercase">Обновление компонентов</span>
            <button
              onClick={async () => {
                try {
                  const w = window as unknown as { go?: { main?: { App?: { CheckAllUpdates?: () => Promise<{ components?: Array<{ name: string; currentVersion: string; latestVersion: string; status: string }> }> } } } };
                  const res = await w.go?.main?.App?.CheckAllUpdates?.();
                  if (res && res.components) {
                    const msg = res.components.map((c) => `${c.name}: ${c.currentVersion} (${c.status === 'update_available' ? 'Доступно обновление ' + c.latestVersion : 'Актуально'})`).join('\n');
                    alert('Состояние компонентов:\n\n' + msg);
                  }
                } catch (e) {
                  console.error(e);
                }
              }}
              className="text-[11px] text-emerald-400 hover:underline"
            >
              Проверить обновления
            </button>
          </div>

          <div className="grid grid-cols-2 gap-2 text-xs">
            <div className="p-2.5 rounded-xl bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)]">
              <div className="text-[10px] text-[var(--ui-text-muted)] uppercase font-mono">Приложение</div>
              <div className="font-semibold text-[var(--ui-text)] mt-0.5">UNBOUND v0.5.0</div>
              <div className="text-[10px] text-emerald-400 font-mono mt-0.5">● Актуально</div>
            </div>
            <div className="p-2.5 rounded-xl bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)]">
              <div className="text-[10px] text-[var(--ui-text-muted)] uppercase font-mono">Движок</div>
              <div className="font-semibold text-[var(--ui-text)] mt-0.5">Zapret 2 v1.0.5</div>
              <div className="text-[10px] text-emerald-400 font-mono mt-0.5">● Актуально</div>
            </div>
            <div className="p-2.5 rounded-xl bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)]">
              <div className="text-[10px] text-[var(--ui-text-muted)] uppercase font-mono">Стратегии</div>
              <div className="font-semibold text-[var(--ui-text)] mt-0.5">v0.5.0 (2026.09.04)</div>
              <div className="text-[10px] text-emerald-400 font-mono mt-0.5">● Актуально</div>
            </div>
            <div className="p-2.5 rounded-xl bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)]">
              <div className="text-[10px] text-[var(--ui-text-muted)] uppercase font-mono">Списки обхода</div>
              <div className="font-semibold text-[var(--ui-text)] mt-0.5">Steam/Discord/YT</div>
              <div className="text-[10px] text-emerald-400 font-mono mt-0.5">● Синхронизировано</div>
            </div>
          </div>
        </div>

        {/* System Actions */}
        <div className="space-y-2 pt-3 border-t border-[var(--ui-border)]">
          <span className="text-xs font-semibold text-[var(--ui-text-muted)] uppercase">Системные действия</span>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            <button onClick={handleRunDiagnostics} className="btn-ui-secondary text-xs">
              <UITerminal className="w-3.5 h-3.5" />
              <span>Диагностика</span>
            </button>

            <button onClick={handleVerifyAssets} disabled={isVerifyingAssets} className="btn-ui-secondary text-xs">
              <UIShield className="w-3.5 h-3.5" />
              <span>Проверка SHA256</span>
            </button>

            <button onClick={handleDiagnosticReport} disabled={isGeneratingReport} className="btn-ui-secondary text-xs">
              <UITerminal className="w-3.5 h-3.5" />
              <span>Отчёт</span>
            </button>

            <button onClick={handleUpdateHostlists} disabled={isUpdatingHostlists} className="btn-ui-secondary text-xs">
              <UIGear className="w-3.5 h-3.5" />
              <span>Обновить списки</span>
            </button>

            <button onClick={handleClearCache} className="btn-ui-secondary text-xs">
              <UIX className="w-3.5 h-3.5" />
              <span>Кэш Discord</span>
            </button>

            <button onClick={handleKillWinws2} className="btn-ui-secondary text-xs">
              <UIX className="w-3.5 h-3.5" />
              <span>Остановить движок</span>
            </button>
          </div>

          <button
            onClick={async () => {
              try {
                await QuitApp();
              } catch (err) {}
            }}
            className="w-full mt-3 py-2 text-xs font-semibold rounded-xl bg-red-600/20 text-red-400 border border-red-500/30 hover:bg-red-600/30 transition-colors"
          >
            Завершить приложение
          </button>
        </div>
      </div>
    </div>
  );
};
