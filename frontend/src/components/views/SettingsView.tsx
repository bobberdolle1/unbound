import React from 'react';
import { UICheckbox } from '../UICheckbox';
import { UISelect } from '../UISelect';
import { UIGear, UIShield, UITerminal, UIX } from '../icons';
import { QuitApp } from '../../../wailsjs/go/main/App';

export interface AppSettings {
  autoStart: boolean;
  startMinimized: boolean;
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
}

interface SettingsViewProps {
  settings: AppSettings;
  setSettings: React.Dispatch<React.SetStateAction<AppSettings>>;
  theme: string;
  setThemeState: (theme: string) => void;
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
          id="showLogs"
          label="Показать журнал"
          desc="Показать панель логов внизу"
          checked={settings.showLogs}
          onChange={() => setSettings({ ...settings, showLogs: !settings.showLogs })}
        />

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
