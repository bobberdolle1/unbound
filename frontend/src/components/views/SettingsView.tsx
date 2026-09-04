import React from 'react';
import { UICheckbox } from '../UICheckbox';
import { UISelect } from '../UISelect';
import { UIGear, UIShield, UITerminal, UIX } from '../icons';
import { QuitApp } from '../../../wailsjs/go/main/App';
import { backendService } from '../../services/backend';
import { engine } from '../../../wailsjs/go/models';
import { cn } from '../../lib/cn';
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
  const [componentStates, setComponentStates] = React.useState<engine.ComponentLocalState[]>([
    new engine.ComponentLocalState({
      component: 'app',
      name: 'Приложение UNBOUND',
      currentVersion: '...',
      status: 'installed',
      statusLabel: 'Установлено',
    }),
    new engine.ComponentLocalState({
      component: 'engine',
      name: 'Движок Zapret 2',
      currentVersion: '...',
      status: 'installed',
      statusLabel: 'Установлено',
    }),
    new engine.ComponentLocalState({
      component: 'strategies',
      name: 'Каталог стратегий',
      currentVersion: '...',
      status: 'installed',
      statusLabel: 'Установлено',
    }),
    new engine.ComponentLocalState({
      component: 'hostlists',
      name: 'Списки обхода',
      currentVersion: '...',
      status: 'synced',
      statusLabel: 'Синхронизировано',
    }),
  ]);
  const [onlineUpdates, setOnlineUpdates] = React.useState<Record<string, engine.ComponentUpdateStatus> | null>(null);
  const [isCheckingUpdates, setIsCheckingUpdates] = React.useState(false);
  const [checkError, setCheckError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (backendService?.getSystemComponentState) {
      backendService.getSystemComponentState()
        .then((state) => {
          if (state && Array.isArray(state.components) && state.components.length > 0) {
            setComponentStates(state.components);
          }
        })
        .catch((err) => {
          console.error('Failed to get local component versions:', err);
        });
    }
  }, []);

  const handleCheckUpdates = async () => {
    setIsCheckingUpdates(true);
    setCheckError(null);
    try {
      const res = await backendService.checkAllUpdates();
      if (res && Array.isArray(res.components)) {
        const updateMap: Record<string, engine.ComponentUpdateStatus> = {};
        res.components.forEach((c) => {
          updateMap[c.component] = c;
        });
        setOnlineUpdates(updateMap);
      }
    } catch (err) {
      console.error('Failed to check updates:', err);
      setCheckError('Ошибка сети или доступа к GitHub API');
    } finally {
      setIsCheckingUpdates(false);
    }
  };

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

        <div className={`pl-6 -mt-1 transition-opacity ${settings.autoStart ? '' : 'opacity-40 pointer-events-none'}`}>
          <UICheckbox
            id="startMinimized"
            label="Тихий старт"
            desc="При автозапуске Windows запускать UNBOUND скрыто в системном трее"
            checked={settings.startMinimized}
            onChange={() => setSettings({ ...settings, startMinimized: !settings.startMinimized })}
            disabled={!settings.autoStart}
          />
        </div>

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
              onClick={handleCheckUpdates}
              disabled={isCheckingUpdates}
              className="text-[11px] text-emerald-400 hover:underline disabled:opacity-50"
            >
              {isCheckingUpdates ? 'Проверка...' : 'Проверить обновления'}
            </button>
          </div>

          <div className="grid grid-cols-2 gap-2 text-xs">
            {componentStates.map((comp) => {
              const online = onlineUpdates ? onlineUpdates[comp.component] : null;
              let statusText = `● ${comp.statusLabel}`;
              let statusClass = 'text-[var(--ui-text-muted)]';

              if (isCheckingUpdates) {
                statusText = 'Проверка...';
                statusClass = 'text-blue-400 animate-pulse';
              } else if (checkError) {
                statusText = '! Не проверено';
                statusClass = 'text-amber-400';
              } else if (online) {
                if (online.updateAvailable) {
                  statusText = `● Доступно ${online.latestVersion}`;
                  statusClass = 'text-emerald-400 font-semibold';
                } else if (online.status === 'check_failed') {
                  statusText = '! Ошибка проверки';
                  statusClass = 'text-amber-400';
                } else {
                  statusText = '● Актуально';
                  statusClass = 'text-emerald-400';
                }
              }

              return (
                <div key={comp.component} className="p-2.5 rounded-xl bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)]">
                  <div className="text-[10px] text-[var(--ui-text-muted)] uppercase font-mono">{comp.name}</div>
                  <div className="font-semibold text-[var(--ui-text)] mt-0.5">{comp.currentVersion}</div>
                  <div className={cn('text-[10px] font-mono mt-0.5', statusClass)}>
                    {statusText}
                  </div>
                </div>
              );
            })}
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

            <button
              onClick={async () => {
                try {
                  await backendService.openLogsFolder();
                } catch (err) {
                  console.error('Failed to open logs folder:', err);
                }
              }}
              className="btn-ui-secondary text-xs"
            >
              <UITerminal className="w-3.5 h-3.5" />
              <span>Папка логов</span>
            </button>

            <button
              onClick={async () => {
                try {
                  await backendService.openCurrentLogFile();
                } catch (err) {
                  console.error('Failed to open current log:', err);
                }
              }}
              className="btn-ui-secondary text-xs"
            >
              <UITerminal className="w-3.5 h-3.5" />
              <span>Текущий лог</span>
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
