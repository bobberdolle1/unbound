import React, { useEffect, useState } from 'react';
import { backendService } from '../../services/backend';
import { main } from '../../../wailsjs/go/models';

interface AutoTuneVNextModalProps {
  isOpen: boolean;
  onClose: () => void;
  onRunningChange: (running: boolean) => void;
}

const strategyLabels: Record<string, string> = {
  'prod-tls-multisplit-1-v1': 'TLS Split',
  'prod-tls-multisplit-overlap-v1': 'TLS Split + overlap',
  'prod-tls-hostfakesplit-v1': 'HostFakeSplit',
};

const resultMessages: Record<string, string> = {
  COMPLETED_NO_ACTION_NEEDED: 'Прямое соединение работает. Изменения не требуются.',
  COMPLETED_NO_ELIGIBLE_CANDIDATES: 'Для обнаруженного типа сбоя нет подходящей стратегии.',
  COMPLETED_NO_VERIFIED_CANDIDATE: 'Проверенные стратегии не восстановили соединение.',
  COMPLETED_SELECTED: 'Проверенная стратегия найдена.',
  PREFLIGHT_FAILED: 'Среда не готова к безопасной проверке.',
  CANCELLED: 'Проверка отменена.',
  INCONCLUSIVE: 'Недостаточно данных для вывода.',
  LIFECYCLE_FAILED: 'Ошибка во время проверки стратегии.',
  STATE_RESTORE_FAILED: 'Не удалось подтвердить восстановление исходного состояния.',
};

function redactTarget(raw?: string): string {
  if (!raw) return '—';
  try {
    const parsed = new URL(raw);
    return `${parsed.protocol}//${parsed.host}${parsed.pathname || '/'}`;
  } catch {
    return '—';
  }
}

function strategyLabel(strategyID?: string): string {
  return strategyID ? strategyLabels[strategyID] || 'Проверенная стратегия' : '—';
}

export const AutoTuneVNextModal: React.FC<AutoTuneVNextModalProps> = ({ isOpen, onClose, onRunningChange }) => {
  const [config, setConfig] = useState<main.AutoTuneVNextExperimentConfig | null>(null);
  const [presetID, setPresetID] = useState('');
  const [customTarget, setCustomTarget] = useState('');
  const [result, setResult] = useState<main.AutoTuneVNextResult | null>(null);
  const [managed, setManaged] = useState<main.AutoTuneVNextManagedStatus | null>(null);
  const [running, setRunning] = useState(false);
  const [loadError, setLoadError] = useState('');

  const refreshStatus = async () => {
    try {
      setManaged(await backendService.getAutoTuneVNextManagedStatus());
    } catch {
      setManaged(null);
    }
  };

  useEffect(() => {
    if (!isOpen) return;
    let active = true;
    setResult(null);
    setLoadError('');
    setCustomTarget('');
    Promise.all([backendService.getAutoTuneVNextExperimentConfig(), backendService.getAutoTuneVNextManagedStatus()])
      .then(([nextConfig, nextStatus]) => {
        if (!active) return;
        setConfig(nextConfig);
        setPresetID(nextConfig.targets?.[0]?.id || '');
        setManaged(nextStatus);
      })
      .catch(() => active && setLoadError('Не удалось загрузить настройки автоподбора.'));
    return () => { active = false; };
  }, [isOpen]);

  if (!isOpen) return null;
  const selectedCustom = presetID === 'custom';
  const canStart = !running && Boolean(config) && Boolean(presetID) && (!selectedCustom || customTarget.trim() !== '') && !managed?.active;
  const restoreFailure = result?.status === 'STATE_RESTORE_FAILED' || managed?.state === 'STATE_RESTORE_FAILED';
  const canRevert = Boolean(managed?.active || restoreFailure);

  const start = async () => {
    if (!canStart) return;
    setResult(null);
    setRunning(true);
    onRunningChange(true);
    try {
      setResult(await backendService.runExperimentalAutoTuneVNext(presetID, selectedCustom ? customTarget : ''));
    } catch {
      setResult(new main.AutoTuneVNextResult({ status: 'LIFECYCLE_FAILED', state_restored: false, catalog_status: 'UNAVAILABLE' }));
    } finally {
      setRunning(false);
      onRunningChange(false);
    }
  };

  const apply = async () => {
    if (!result?.apply_available || !result.apply_token || running) return;
    setRunning(true);
    onRunningChange(true);
    try {
      setManaged(await backendService.applyAutoTuneVNextSelection(result.apply_token));
    } finally {
      setRunning(false);
      onRunningChange(false);
      await refreshStatus();
    }
  };

  const revert = async () => {
    if (!canRevert || running) return;
    onRunningChange(true);
    try {
      setManaged(await backendService.revertAutoTuneVNext());
    } finally {
      setRunning(false);
      onRunningChange(false);
      await refreshStatus();
    }
  };

  return (
    <div className="fixed inset-0 z-[10020] bg-black/60 flex items-center justify-center p-4 app-no-drag" role="dialog" aria-modal="true" aria-label="Автоподбор стратегии">
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-[var(--ui-border-strong)] bg-[var(--ui-panel)] p-5 space-y-4 shadow-xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-base font-semibold">Автоподбор стратегии</h2>
            <p className="mt-1 text-xs text-[var(--ui-text-muted)]">Проверка сначала восстанавливает исходное состояние. Применение повторно проверяет стратегию на текущем соединении.</p>
          </div>
          <button onClick={onClose} disabled={running} className="btn-ui-secondary text-xs">Закрыть</button>
        </div>

        {loadError && <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">{loadError}</div>}
        {restoreFailure && <div className="rounded-lg border-2 border-red-500 bg-red-500/15 p-4 text-red-100"><div className="font-semibold">Критично: Не удалось подтвердить восстановление исходного состояния.</div></div>}

        {managed?.active && (
          <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 p-4 space-y-2 text-sm text-emerald-100">
            <div className="font-semibold">Стратегия применена</div>
            <div>Стратегия: {strategyLabel(managed.strategy_id)}</div>
            <div>Цель: {redactTarget(managed.target)}</div>
            <button onClick={revert} disabled={running} className="btn-ui-secondary w-full justify-center">Отключить / Вернуть прямое подключение</button>
          </div>
        )}
        {restoreFailure && !managed?.active && (
          <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-4 space-y-2 text-sm text-red-100">
            <div className="font-semibold">Восстановление состояния ожидает повторной проверки</div>
            <button onClick={revert} disabled={running} className="btn-ui-secondary w-full justify-center">Повторить восстановление</button>
          </div>
        )}

        {config && !running && !result && !managed?.active && !restoreFailure && (
          <div className="space-y-3">
            <div><div className="text-xs font-semibold text-[var(--ui-text-muted)] mb-2">Целевой сервис</div><div className="flex flex-wrap gap-2">
              {config.targets.map((preset) => <button key={preset.id} onClick={() => setPresetID(preset.id)} className={presetID === preset.id ? 'btn-ui-primary text-xs' : 'btn-ui-secondary text-xs'}>{preset.label}</button>)}
              <button onClick={() => setPresetID('custom')} className={selectedCustom ? 'btn-ui-primary text-xs' : 'btn-ui-secondary text-xs'}>Другой HTTPS URL</button>
            </div></div>
            {selectedCustom && <label className="block text-xs text-[var(--ui-text-muted)]">HTTPS URL<input value={customTarget} onChange={(event) => setCustomTarget(event.target.value)} placeholder="https://example.com/path" className="mt-1 w-full rounded-lg border border-[var(--ui-border)] bg-[var(--ui-bg)] px-3 py-2 text-sm text-[var(--ui-text)]" /></label>}
            <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 text-xs text-[var(--ui-text-muted)]">Защищённый контроль: {redactTarget(config.default_control?.target)}</div>
            <button onClick={start} disabled={!canStart} className="btn-ui-primary w-full justify-center">Проверить стратегии</button>
          </div>
        )}

        {running && <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4"><div className="font-medium">Проверяем и безопасно восстанавливаем состояние...</div><button onClick={() => backendService.cancelAutoTune()} className="mt-3 text-sm text-red-300 hover:underline">Отменить</button></div>}

        {result && !running && !managed?.active && (
          <div className="space-y-3">
            <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4 font-medium">{resultMessages[result.status] || 'Получен неизвестный результат проверки.'}</div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-xs"><dt className="text-[var(--ui-text-muted)]">Цель</dt><dd>{redactTarget(result.target)}</dd><dt className="text-[var(--ui-text-muted)]">Стратегия</dt><dd>{strategyLabel(result.selected_strategy_id)}</dd><dt className="text-[var(--ui-text-muted)]">Исходное состояние</dt><dd>{result.state_restored ? 'восстановлено' : 'не подтверждено'}</dd></dl>
            {result.status === 'COMPLETED_SELECTED' && result.apply_available && <button onClick={apply} className="btn-ui-primary w-full justify-center">Применить</button>}
            {result.status === 'COMPLETED_SELECTED' && !result.apply_available && <div className="text-sm text-red-300">Проверенная стратегия не получила разрешение на применение.</div>}
            <button onClick={onClose} className="btn-ui-secondary w-full justify-center">Закрыть результат</button>
          </div>
        )}
      </div>
    </div>
  );
};
