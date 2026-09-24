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
  COMPLETED_SELECTED: 'Найдена стратегия, подтвердившая восстановление соединения.',
  PREFLIGHT_FAILED: 'Среда не готова к безопасной проверке.',
  CANCELLED: 'Проверка отменена.',
  INCONCLUSIVE: 'Недостаточно данных для вывода.',
  LIFECYCLE_FAILED: 'Ошибка во время тестирования стратегии.',
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
  if (!strategyID) return '—';
  return strategyLabels[strategyID] || 'Проверенная стратегия';
}

function shortFingerprint(fingerprint?: string): string {
  return fingerprint ? fingerprint.slice(0, 12) : '—';
}

export const AutoTuneVNextModal: React.FC<AutoTuneVNextModalProps> = ({ isOpen, onClose, onRunningChange }) => {
  const [config, setConfig] = useState<main.AutoTuneVNextExperimentConfig | null>(null);
  const [presetID, setPresetID] = useState('');
  const [customTarget, setCustomTarget] = useState('');
  const [result, setResult] = useState<main.AutoTuneVNextResult | null>(null);
  const [running, setRunning] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [loadError, setLoadError] = useState('');

  useEffect(() => {
    if (!isOpen) return;
    let active = true;
    setResult(null);
    setLoadError('');
    setCustomTarget('');
    backendService.getAutoTuneVNextExperimentConfig()
      .then((nextConfig) => {
        if (!active) return;
        setConfig(nextConfig);
        setPresetID(nextConfig.targets?.[0]?.id || '');
      })
      .catch(() => {
        if (active) setLoadError('Не удалось загрузить экспериментальные цели.');
      });
    return () => {
      active = false;
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const selectedCustom = presetID === 'custom';
  const canStart = !running && Boolean(config) && Boolean(presetID) && (!selectedCustom || customTarget.trim() !== '');
  const resultMessage = result ? resultMessages[result.status] || 'Получен неизвестный результат проверки.' : '';
  const restoreFailure = result?.status === 'STATE_RESTORE_FAILED';

  const start = async () => {
    if (!canStart) return;
    setResult(null);
    setRunning(true);
    setCancelling(false);
    onRunningChange(true);
    try {
      const nextResult = await backendService.runExperimentalAutoTuneVNext(presetID, selectedCustom ? customTarget : '');
      setResult(nextResult);
    } catch {
      setResult(new main.AutoTuneVNextResult({
        status: 'LIFECYCLE_FAILED',
        state_restored: false,
        catalog_status: 'UNAVAILABLE',
        limitations: ['PRODUCT_CALL_FAILED'],
      }));
    } finally {
      setRunning(false);
      onRunningChange(false);
    }
  };

  const cancel = async () => {
    if (!running || cancelling) return;
    setCancelling(true);
    try {
      await backendService.cancelAutoTune();
    } catch {
      // The active operation remains authoritative; wait for its terminal result.
    }
  };

  return (
    <div className="fixed inset-0 z-[10020] bg-black/60 flex items-center justify-center p-4 app-no-drag" role="dialog" aria-modal="true" aria-label="AutoTune vNext эксперимент">
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-[var(--ui-border-strong)] bg-[var(--ui-panel)] p-5 space-y-4 shadow-xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-base font-semibold">AutoTune vNext (эксперимент)</h2>
            <p className="mt-1 text-xs text-[var(--ui-text-muted)]">Проверка не активирует стратегию постоянно. После тестирования исходное состояние восстанавливается.</p>
          </div>
          <button onClick={onClose} disabled={running} className="btn-ui-secondary text-xs" aria-label="Закрыть эксперимент AutoTune vNext">Закрыть</button>
        </div>

        {loadError && <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-300">{loadError}</div>}

        {config && !running && !result && (
          <div className="space-y-3">
            <div>
              <div className="text-xs font-semibold text-[var(--ui-text-muted)] mb-2">Целевой сервис</div>
              <div className="flex flex-wrap gap-2">
                {config.targets.map((preset) => (
                  <button
                    key={preset.id}
                    onClick={() => setPresetID(preset.id)}
                    className={presetID === preset.id ? 'btn-ui-primary text-xs' : 'btn-ui-secondary text-xs'}
                  >
                    {preset.label}
                  </button>
                ))}
                <button onClick={() => setPresetID('custom')} className={selectedCustom ? 'btn-ui-primary text-xs' : 'btn-ui-secondary text-xs'}>Другой HTTPS URL</button>
              </div>
            </div>
            {selectedCustom && (
              <label className="block text-xs text-[var(--ui-text-muted)]">
                HTTPS URL
                <input
                  value={customTarget}
                  onChange={(event) => setCustomTarget(event.target.value)}
                  placeholder="https://example.com/path"
                  className="mt-1 w-full rounded-lg border border-[var(--ui-border)] bg-[var(--ui-bg)] px-3 py-2 text-sm text-[var(--ui-text)]"
                />
              </label>
            )}
            <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 text-xs text-[var(--ui-text-muted)]">
              Защищённый контроль: {redactTarget(config.default_control?.target)}
            </div>
            <button onClick={start} disabled={!canStart} className="btn-ui-primary w-full justify-center">Начать экспериментальную проверку</button>
          </div>
        )}

        {running && (
          <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4 space-y-3">
            <div className="font-medium">{cancelling ? 'Восстанавливаем исходное состояние...' : 'Проверяем прямое соединение...'}</div>
            {!cancelling && <div className="text-xs text-[var(--ui-text-muted)]">Проверяем безопасные стратегии...</div>}
            <div className="text-xs text-[var(--ui-text-muted)]">Индикатор без процентов: фактические фазы определяет продуктовый жизненный цикл.</div>
            <button onClick={cancel} disabled={cancelling} className="text-sm text-red-300 hover:underline disabled:opacity-60">{cancelling ? 'Отмена принята, ожидаем завершения' : 'Отменить проверку'}</button>
          </div>
        )}

        {result && !running && (
          <div className="space-y-3">
            {restoreFailure ? (
              <div className="rounded-lg border-2 border-red-500 bg-red-500/15 p-4 text-red-100">
                <div className="font-semibold">Критично: {resultMessage}</div>
                <div className="mt-1 text-sm">Не запускайте новую проверку, пока исходное состояние не будет подтверждено.</div>
              </div>
            ) : (
              <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4 font-medium">{resultMessage}</div>
            )}

            {result.status === 'COMPLETED_SELECTED' && (
              <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 p-3 text-sm text-emerald-100">
                Найдена проверенная стратегия. После тестирования исходное состояние восстановлено.
              </div>
            )}

            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-xs">
              <dt className="text-[var(--ui-text-muted)]">Цель</dt><dd>{redactTarget(result.target)}</dd>
              <dt className="text-[var(--ui-text-muted)]">Статус</dt><dd>{result.status}</dd>
              <dt className="text-[var(--ui-text-muted)]">Исходное состояние</dt><dd>{result.state_restored ? 'восстановлено' : 'не подтверждено'}</dd>
              <dt className="text-[var(--ui-text-muted)]">Планировщик</dt><dd>{result.planner_disposition || '—'}</dd>
              <dt className="text-[var(--ui-text-muted)]">Каталог</dt><dd>{result.catalog_status || '—'}</dd>
              {result.selected_strategy_id && <><dt className="text-[var(--ui-text-muted)]">Найденная стратегия</dt><dd>{strategyLabel(result.selected_strategy_id)}</dd></>}
            </dl>

            {(result.candidate_outcomes || []).length > 0 && (
              <div className="space-y-2">
                <div className="text-xs font-semibold text-[var(--ui-text-muted)]">Результаты кандидатов</div>
                {result.candidate_outcomes?.map((candidate) => (
                  <div key={`${candidate.strategy_id}:${candidate.fingerprint || ''}`} className="rounded border border-[var(--ui-border)] p-2 text-xs">
                    <div>{strategyLabel(candidate.strategy_id)} — {candidate.outcome}</div>
                    <details className="mt-1 text-[var(--ui-text-muted)]">
                      <summary>Технические сведения</summary>
                      <div>ID: {candidate.strategy_id}</div>
                      <div>Fingerprint: {shortFingerprint(candidate.fingerprint)}</div>
                      <div>Planner: {candidate.planner_status || '—'}</div>
                    </details>
                  </div>
                ))}
              </div>
            )}

            {(result.limitations || []).length > 0 && <div className="text-xs text-[var(--ui-text-muted)]">Ограничения: {result.limitations?.join(', ')}</div>}
            <button onClick={onClose} className="btn-ui-secondary w-full justify-center">Закрыть результат</button>
          </div>
        )}
      </div>
    </div>
  );
};
