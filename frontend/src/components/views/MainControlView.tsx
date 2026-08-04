import React from 'react';
import { cn } from '../../lib/cn';
import { UINetwork, UIStar, UISpinner, UIZap } from '../icons';
import { UISelect } from '../UISelect';
import { PingChart } from '../PingChart';

interface MainControlViewProps {
  statusLedState: 'connected' | 'connecting' | 'error' | 'disconnected';
  isConnected: boolean;
  isConnecting: boolean;
  disableMain: boolean;
  toggleConnection: () => void;
  selectedProfile: string;
  setSelectedProfile: (profile: string) => void;
  sortedProfiles: string[];
  selectedEngine: string;
  handleToggleFavorite: () => void;
  favoriteProfiles: string[];
  handleAutoTune: () => void;
  isScanning: boolean;
  scanProgress: string;
  autotuneProgress: { percent?: number; msg?: string } | null;
  handleCancelAutoTune: () => void;
  livePingData: { active: boolean; latency: number; status: string };
  pingHistory: number[];
}

export const MainControlView: React.FC<MainControlViewProps> = ({
  statusLedState,
  isConnected,
  isConnecting,
  disableMain,
  toggleConnection,
  selectedProfile,
  setSelectedProfile,
  sortedProfiles,
  selectedEngine,
  handleToggleFavorite,
  favoriteProfiles,
  handleAutoTune,
  isScanning,
  scanProgress,
  autotuneProgress,
  handleCancelAutoTune,
  livePingData,
  pingHistory,
}) => {
  const isFavorite = favoriteProfiles.includes(selectedProfile);

  return (
    <div className="flex-1 flex flex-col gap-4">
      {/* CONNECTION CARD */}
      <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-5 flex flex-col items-center gap-4 text-center">
        <div className="flex items-center gap-2 text-xs font-semibold text-[var(--ui-text-muted)]">
          <span className={cn('status-dot-led', statusLedState)} />
          <span>
            {statusLedState === 'connected'
              ? 'Обход активен'
              : statusLedState === 'connecting'
              ? 'Инициализация драйвера...'
              : statusLedState === 'error'
              ? 'Ошибка драйвера'
              : 'Служба остановлена'}
          </span>
        </div>

        <button
          onClick={toggleConnection}
          disabled={disableMain}
          className="btn-ui-primary max-w-[280px]"
        >
          <UINetwork className="w-4 h-4" />
          <span>{isConnected ? 'Отключить' : isConnecting ? 'Подключение...' : 'Подключить'}</span>
        </button>

        <div className="text-xs text-[var(--ui-text-muted)] truncate max-w-full">
          Стратегия:{' '}
          <strong className="text-[var(--ui-text)] font-semibold">
            {selectedProfile || 'Автоматическая'}
          </strong>
        </div>
      </div>

      {/* PROFILE SELECTOR & AUTOTUNE (RESPONSIVE GRID) */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="sm:col-span-2 flex gap-2">
          <div className="flex-1 min-w-0">
            <UISelect
              value={selectedProfile}
              options={sortedProfiles}
              onChange={(val: string) => setSelectedProfile(val)}
              disabled={isConnected || disableMain || !selectedEngine}
            />
          </div>
          <button
            onClick={handleToggleFavorite}
            disabled={!selectedProfile}
            title={isFavorite ? 'Удалить из избранного' : 'Добавить в избранное'}
            className={cn(
              'px-3 rounded-lg border transition-all flex items-center justify-center shrink-0',
              isFavorite
                ? 'bg-amber-500/10 text-amber-400 border-amber-500/30'
                : 'btn-ui-secondary'
            )}
          >
            <UIStar className="w-4 h-4" />
          </button>
        </div>

        <button
          onClick={handleAutoTune}
          disabled={disableMain || isScanning}
          className="btn-ui-secondary w-full justify-center"
        >
          <UIZap className="w-4 h-4" />
          <span>{isScanning ? 'Сканирование...' : 'Автоподбор стратегии'}</span>
        </button>
      </div>

      {/* AUTOTUNE PROGRESS CARD */}
      {isScanning && (
        <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 space-y-3">
          <div className="flex items-center justify-between text-xs">
            <div className="flex items-center gap-2 font-semibold text-[var(--ui-text)]">
              <UISpinner className="w-4 h-4 animate-spin" />
              <span>Автоподбор стратегии...</span>
            </div>
            {autotuneProgress && (
              <span className="font-mono text-[var(--ui-text-muted)]">{autotuneProgress.percent}%</span>
            )}
          </div>

          <div className="w-full h-2 rounded-full bg-[var(--ui-bg)] overflow-hidden border border-[var(--ui-border)]">
            <div
              className="h-full bg-[var(--ui-text)] transition-all duration-300"
              style={{ width: `${autotuneProgress?.percent || 0}%` }}
            />
          </div>

          <div className="flex items-center justify-between text-xs text-[var(--ui-text-muted)]">
            <span className="truncate">{scanProgress || 'Тестирование профилей...'}</span>
            <button
              onClick={handleCancelAutoTune}
              className="text-red-400 hover:underline shrink-0 text-xs"
            >
              Отмена
            </button>
          </div>
        </div>
      )}

      {/* PING & NETWORK TELEMETRY */}
      <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <span className="text-xs font-semibold text-[var(--ui-text-muted)]">Задержка соединения (Ping)</span>
          <span className="text-xs font-mono font-semibold text-[var(--ui-text)]">
            {livePingData?.latency ? `${livePingData.latency} ms` : '-- ms'}
          </span>
        </div>
        <PingChart history={pingHistory} livePingData={livePingData} />
      </div>
    </div>
  );
};
