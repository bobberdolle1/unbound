import React, { useEffect, useRef, useState } from 'react';
import {
  AutoTuneVNextHistorySummary,
  AutoTuneVNextServiceGraphResult,
  AutoTuneVNextServicePreset,
  backendService,
} from '../../services/backend';
import { AutoTuneVNextProgressData, eventBus } from '../../services/window';
import { main } from '../../../wailsjs/go/models';
import { cn } from '../../lib/cn';

interface AutoTuneVNextModalProps {
  isOpen: boolean;
  onClose: () => void;
  onRunningChange: (running: boolean) => void;
  platform?: string;
}

const strategyLabels: Record<string, string> = {
  'prod-tls-multisplit-1-v1': 'TLS Split (позиция 1)',
  'prod-tls-multisplit-overlap-v1': 'TLS Split + overlap',
  'prod-tls-hostfakesplit-v1': 'HostFakeSplit (подмена SNI)',
  'prod-tls-multidisorder-v1': 'TLS MultiDisorder',
  'prod-tls-multidisorder-sni-v1': 'TLS MultiDisorder (SNI)',
  'prod-tls-tlsrec-multisplit-v1': 'TLS Record Split + MultiSplit',
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

const stageLabels: Record<string, string> = {
  preflight: 'Предпроверка среды',
  baseline: 'Базовое измерение прямого доступа',
  diagnosis: 'Классификация сетевого сбоя',
  planning: 'Отбор и проверка стратегий',
  applying: 'Применение и контроль состояния',
  complete: 'Завершено',
};

const diagnosisExplanations: Record<string, string> = {
  NO_ANOMALY: 'Аномалий не выявлено — прямое соединение работает штатно',
  DNS_PATH_ANOMALY: 'Аномалия DNS-разрешения на маршруте к целевому узлу',
  TCP_PATH_FAILURE: 'Сбой установки TCP-соединения на маршруте',
  TLS_HANDSHAKE_PATH_FAILURE: 'Сбой TLS-рукопожатия (фильтрация ClientHello / SNI)',
  HTTP_APPLICATION_FAILURE: 'Нарушение ответа на уровне HTTP при успешном TLS',
  PARTIAL_TRANSFER_ANOMALY: 'Прерывание передачи данных после установки соединения',
  EDGE_DEPENDENT_FAILURE: 'Частичная недоступность отдельных CDN-узлов (зависит от IP)',
  NETWORK_CONTEXT_FAILURE: 'Общий сбой сетевого подключения (контрольный узел также недоступен)',
  INSUFFICIENT_EVIDENCE: 'Недостаточно диагностических данных для точной классификации',
  UNKNOWN: 'Неизвестный характер сетевого сбоя',
};

const confidenceLabels: Record<string, string> = {
  HIGH: 'высокая достоверность',
  MEDIUM: 'средняя достоверность',
  LOW: 'низкая достоверность',
};

const candidateOutcomeLabels: Record<string, string> = {
  VERIFIED_FIXED: 'Подтверждено — соединение восстановлено',
  STILL_FAILING: 'Сбой сохраняется',
  REGRESSION_OBSERVED: 'Отклонено — ухудшение контрольного соединения',
  DIRECT_BECAME_REACHABLE: 'Прямой доступ восстановился самостоятельно',
  INCONCLUSIVE: 'Неоднозначный результат проверки',
  BACKEND_UNSUPPORTED: 'Не поддерживается текущим пакетным движком',
  HOST_UNSUPPORTED: 'Не поддерживается окружением ОС',
  LIFECYCLE_FAILURE: 'Ошибка запуска или возврата состояния',
  NOT_RUN_BUDGET: 'Пропущено по лимиту числа проверок',
  NOT_RUN_POLICY: 'Пропущено правилами отбора',
};

const limitationExplanations: Record<string, string> = {
  MEASUREMENT_PATH_UNSUPPORTED: 'Пакетный перехват L3/L4 не поддерживается на данной платформе (например, режим tpws SOCKS5 на macOS).',
  MANAGED_ACTIVATION_ACTIVE: 'Уже активна управляемая стратегия — сначала отключите её.',
  OPERATION_CONFLICT: 'Конфликт с другой выполняющейся сетевой операцией.',
  INVALID_TARGET: 'Указан некорректный HTTPS-адрес цели.',
  INVALID_CONTROL: 'Некорректный адрес контрольного узла.',
  PRODUCT_CATALOG_INVALID: 'Каталог проверенных стратегий недоступен или повреждён.',
  PRODUCT_STATE_UNCERTAIN: 'Не удалось зафиксировать исходное состояние движка для безопасного отката.',
  PRODUCT_RUNTIME_UNAVAILABLE: 'Пакетный движок недоступен или отсутствуют необходимые права.',
  PRODUCT_ASSETS_UNAVAILABLE: 'Не найдены требуемые системные компоненты движка.',
  PRODUCT_PROBE_CONTRACT_INVALID: 'Ошибка параметров проверочного запроса.',
  SERVICE_SCOPE_TOO_LARGE: 'Слишком много сетевых узлов (IP) у целевого сервиса для ограниченной проверки.',
  SERVICE_SCOPE_EMPTY: 'Не удалось разрешить IP-адреса целевого домена.',
  SERVICE_SCOPE_INVALID: 'Некорректная область сетевых узлов целевого сервиса.',
  SAME_EDGE_REQUIRED: 'Во время проверки сменился IP-адрес CDN-узла, требуется повтор.',
  V2_EVIDENCE_UNAVAILABLE: 'Расширенные телеметрические данные недоступны.',
  RECOMMENDATION_INVALID_FALLBACK: 'История подбора не совпала с текущим набором стратегий, применён базовый порядок.',
};

const lifecycleErrorExplanations: Record<string, string> = {
  SNAPSHOT_FAILED: 'Не удалось сохранить снимок исходного состояния движка.',
  ESTABLISH_DIRECT_FAILED: 'Не удалось перевести систему в режим прямого соединения перед проверкой.',
  ACTIVATION_FAILED: 'Ошибка запуска проверяемой стратегии.',
  ACTIVE_VERIFICATION_FAILED: 'Процесс стратегии завершился преждевременно при проверке.',
  DEACTIVATE_FAILED: 'Ошибка остановки временного процесса проверки.',
  RESTORE_FAILED: 'Ошибка возврата исходного профиля после проверки.',
  RESTORE_VERIFICATION_FAILED: 'Не удалось подтвердить возврат исходного сетевого состояния.',
};

function redactTarget(raw?: string): string {
  if (!raw) return '—';
  try {
    const parsed = new URL(raw);
    return `${parsed.protocol}//${parsed.host}${parsed.pathname || '/'}`;
  } catch {
    return raw;
  }
}

function strategyLabel(strategyID?: string): string {
  if (!strategyID) return '—';
  return strategyLabels[strategyID] || strategyID;
}

function explainLimitation(raw: string): string {
  const code = raw.split(':')[0]?.trim() || raw;
  return limitationExplanations[raw] || limitationExplanations[code] || raw;
}

function explainLifecycleError(code: string): string {
  return lifecycleErrorExplanations[code] || `Код сбоя жизненного цикла: ${code}`;
}

export const AutoTuneVNextModal: React.FC<AutoTuneVNextModalProps> = ({
  isOpen,
  onClose,
  onRunningChange,
  platform = '',
}) => {
  const [mode, setMode] = useState<'single' | 'service_graph'>('single');
  const [config, setConfig] = useState<main.AutoTuneVNextExperimentConfig | null>(null);
  const [presetID, setPresetID] = useState('');
  const [customTarget, setCustomTarget] = useState('');
  const [result, setResult] = useState<main.AutoTuneVNextResult | null>(null);
  const [managed, setManaged] = useState<main.AutoTuneVNextManagedStatus | null>(null);

  const [servicePresets, setServicePresets] = useState<AutoTuneVNextServicePreset[]>([]);
  const [selectedServicePresetID, setSelectedServicePresetID] = useState<string>('youtube');
  const [serviceGraphResult, setServiceGraphResult] = useState<AutoTuneVNextServiceGraphResult | null>(null);

  const [historySummary, setHistorySummary] = useState<AutoTuneVNextHistorySummary | null>(null);
  const [clearingHistory, setClearingHistory] = useState(false);

  const [progress, setProgress] = useState<AutoTuneVNextProgressData | null>(null);
  const [running, setRunning] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [loadError, setLoadError] = useState('');

  const isAliveRef = useRef<boolean>(false);
  isAliveRef.current = isOpen;

  useEffect(() => {
    return () => {
      isAliveRef.current = false;
    };
  }, []);

  const refreshStatusAndHistory = async () => {
    try {
      const [nextStatus, nextHistory] = await Promise.all([
        backendService.getAutoTuneVNextManagedStatus(),
        backendService.getAutoTuneVNextHistorySummary(),
      ]);
      if (!isAliveRef.current) return;
      setManaged(nextStatus);
      setHistorySummary(nextHistory);
    } catch {
      if (isAliveRef.current) {
        setManaged(null);
      }
    }
  };

  useEffect(() => {
    if (!isOpen) return;
    let active = true;
    setResult(null);
    setServiceGraphResult(null);
    setProgress(null);
    setLoadError('');
    setCustomTarget('');
    setMode('single');

    Promise.all([
      backendService.getAutoTuneVNextExperimentConfig(),
      backendService.getAutoTuneVNextManagedStatus(),
      backendService.getAutoTuneVNextHistorySummary().catch(() => null),
      backendService.getAutoTuneVNextServicePresets().catch(() => []),
    ])
      .then(([nextConfig, nextStatus, nextHistory, nextServicePresets]) => {
        if (!active) return;
        setConfig(nextConfig);
        setPresetID(nextConfig.targets?.[0]?.id || '');
        setManaged(nextStatus);
        if (nextHistory) setHistorySummary(nextHistory);
        if (Array.isArray(nextServicePresets) && nextServicePresets.length > 0) {
          setServicePresets(nextServicePresets);
          setSelectedServicePresetID(nextServicePresets[0].id || 'youtube');
        }
      })
      .catch(() => {
        if (active) setLoadError('Не удалось загрузить настройки автоподбора.');
      });

    return () => {
      active = false;
    };
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen || typeof eventBus?.onAutoTuneVNextProgress !== 'function') return;
    const unsub = eventBus.onAutoTuneVNextProgress((data) => {
      if (isAliveRef.current && data) {
        setProgress(data);
      }
    });
    return () => {
      if (typeof unsub === 'function') unsub();
    };
  }, [isOpen]);

  if (!isOpen) return null;

  const isMac = platform === 'darwin';
  const selectedCustom = presetID === 'custom';
  const canStartSingle =
    !running &&
    Boolean(config) &&
    Boolean(presetID) &&
    (!selectedCustom || customTarget.trim() !== '') &&
    !managed?.active;
  const canStartGraph =
    !running && servicePresets.length > 0 && Boolean(selectedServicePresetID) && !managed?.active;
  const restoreFailure = result?.status === 'STATE_RESTORE_FAILED' || managed?.state === 'STATE_RESTORE_FAILED';
  const canRevert = Boolean(managed?.active || restoreFailure);

  const selectedServicePreset = servicePresets.find((p) => p.id === selectedServicePresetID) || servicePresets[0];

  const start = async () => {
    if (!canStartSingle) return;
    setResult(null);
    setServiceGraphResult(null);
    setProgress({ stage: 'preflight', message: 'Подготовка безопасной проверки...', percent: 10 });
    setRunning(true);
    setCancelling(false);
    onRunningChange(true);
    try {
      const nextResult = await backendService.runExperimentalAutoTuneVNext(
        presetID,
        selectedCustom ? customTarget.trim() : ''
      );
      if (isAliveRef.current) {
        setResult(nextResult);
      }
    } catch {
      if (isAliveRef.current) {
        setResult(
          new main.AutoTuneVNextResult({
            status: 'LIFECYCLE_FAILED',
            state_restored: false,
            catalog_status: 'UNAVAILABLE',
          })
        );
      }
    } finally {
      if (isAliveRef.current) {
        setRunning(false);
        setCancelling(false);
      }
      onRunningChange(false);
      if (isAliveRef.current) {
        await refreshStatusAndHistory();
      }
    }
  };

  const startServiceGraph = async () => {
    if (!canStartGraph) return;
    setResult(null);
    setServiceGraphResult(null);
    setProgress({ stage: 'preflight', message: 'Подготовка пакетной проверки доменов...', percent: 10 });
    setRunning(true);
    setCancelling(false);
    onRunningChange(true);
    try {
      const graphRes = await backendService.autoTuneVNextServiceGraph(selectedServicePresetID);
      if (isAliveRef.current) {
        setServiceGraphResult(graphRes);
      }
    } catch (err) {
      if (isAliveRef.current) {
        setServiceGraphResult({
          status: 'LIFECYCLE_FAILED',
          preset_id: selectedServicePresetID,
          hosts: selectedServicePreset?.hosts || [],
          verified_hosts: [],
          applied: false,
          summary: 'Не удалось выполнить пакетную проверку ServiceGraph.',
          errors: [err instanceof Error ? err.message : String(err)],
        });
      }
    } finally {
      if (isAliveRef.current) {
        setRunning(false);
        setCancelling(false);
      }
      onRunningChange(false);
      if (isAliveRef.current) {
        await refreshStatusAndHistory();
      }
    }
  };

  const handleCancel = async () => {
    setCancelling(true);
    try {
      await backendService.cancelAutoTuneVNext();
    } catch {
      // ignore
    }
  };

  const apply = async () => {
    if (!result?.apply_available || !result.apply_token || running) return;
    setRunning(true);
    onRunningChange(true);
    try {
      const nextManaged = await backendService.applyAutoTuneVNextSelection(result.apply_token);
      if (isAliveRef.current) {
        setManaged(nextManaged);
      }
    } finally {
      if (isAliveRef.current) {
        setRunning(false);
      }
      onRunningChange(false);
      if (isAliveRef.current) {
        await refreshStatusAndHistory();
      }
    }
  };

  const revert = async () => {
    if (!canRevert || running) return;
    setRunning(true);
    onRunningChange(true);
    try {
      const nextManaged = await backendService.revertAutoTuneVNext();
      if (isAliveRef.current) {
        setManaged(nextManaged);
        setServiceGraphResult(null);
      }
    } finally {
      if (isAliveRef.current) {
        setRunning(false);
      }
      onRunningChange(false);
      if (isAliveRef.current) {
        await refreshStatusAndHistory();
      }
    }
  };

  const revertServiceGraph = async () => {
    if (running) return;
    setRunning(true);
    onRunningChange(true);
    try {
      await backendService.revertAutoTuneVNextServiceGraph();
      if (isAliveRef.current) {
        setServiceGraphResult(null);
      }
    } finally {
      if (isAliveRef.current) {
        setRunning(false);
      }
      onRunningChange(false);
      if (isAliveRef.current) {
        await refreshStatusAndHistory();
      }
    }
  };

  const handleClearHistory = async () => {
    if (running || clearingHistory) return;
    setClearingHistory(true);
    try {
      await backendService.clearAutoTuneVNextHistory();
      const updated = await backendService.getAutoTuneVNextHistorySummary();
      if (isAliveRef.current) {
        setHistorySummary(updated);
      }
    } catch {
      // ignore
    } finally {
      if (isAliveRef.current) {
        setClearingHistory(false);
      }
    }
  };

  const diagnosisKind = result?.diagnosis?.kind || '';
  const diagnosisConfidence = result?.diagnosis?.confidence || 'LOW';
  const progressPercent = Math.max(5, Math.min(100, progress?.percent || 10));

  return (
    <div
      className="fixed inset-0 z-[10020] bg-black/60 flex items-center justify-center p-4 app-no-drag"
      role="dialog"
      aria-modal="true"
      aria-label="Автоподбор стратегии"
    >
      <div className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-xl border border-[var(--ui-border-strong)] bg-[var(--ui-panel)] p-5 space-y-4 shadow-xl text-[var(--ui-text)]">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 className="text-base font-semibold">Автоподбор стратегии</h2>
            <p className="mt-1 text-xs text-[var(--ui-text-muted)]">
              Проверка сначала восстанавливает исходное состояние. Применение повторно проверяет стратегию на текущем соединении.
            </p>
          </div>
          <button onClick={onClose} disabled={running} className="btn-ui-secondary text-xs">
            Закрыть
          </button>
        </div>

        {/* macOS tpws guidance banner */}
        {isMac && (
          <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3.5 text-xs space-y-1.5">
            <div className="font-semibold text-amber-400">
              Особенность macOS: режим проксирования tpws (SOCKS5 / PF)
            </div>
            <p className="text-[var(--ui-text-muted)] leading-relaxed">
              На macOS движок работает через локальный прокси <code className="font-mono">tpws</code>, где низкоуровневая пакетная модификация L3/L4 (<code className="font-mono">winws2</code> / <code className="font-mono">nfqws2</code>) недоступна. Для подбора рабочего профиля на macOS рекомендуется использовать <strong>Strategy Lab</strong> (вкладка «Профили &amp; LUA») или кнопку <strong>Legacy AutoTune</strong> на главном экране.
            </p>
          </div>
        )}

        {loadError && (
          <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-3 text-sm text-red-400">{loadError}</div>
        )}

        {restoreFailure && (
          <div className="rounded-lg border-2 border-red-500 bg-red-500/15 p-4 text-red-300">
            <div className="font-semibold">
              Критично: Не удалось подтвердить восстановление исходного состояния.
            </div>
          </div>
        )}

        {managed?.active && !serviceGraphResult?.applied && (
          <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 p-4 space-y-2 text-sm">
            <div className="font-semibold text-emerald-400">Стратегия применена</div>
            <div className="text-xs">Стратегия: {strategyLabel(managed.strategy_id)}</div>
            <div className="text-xs text-[var(--ui-text-muted)]">Цель: {redactTarget(managed.target)}</div>
            <button onClick={revert} disabled={running} className="btn-ui-secondary w-full justify-center">
              Отключить / Вернуть прямое подключение
            </button>
          </div>
        )}

        {restoreFailure && !managed?.active && (
          <div className="rounded-lg border border-red-500/40 bg-red-500/10 p-4 space-y-2 text-sm">
            <div className="font-semibold text-red-400">Восстановление состояния ожидает повторной проверки</div>
            <button onClick={revert} disabled={running} className="btn-ui-secondary w-full justify-center">
              Повторить восстановление
            </button>
          </div>
        )}

        {/* Mode Switch & Target Selection */}
        {config && !running && !result && !serviceGraphResult && !managed?.active && !restoreFailure && (
          <div className="space-y-4">
            <div className="flex items-center gap-2 bg-[var(--ui-surface-elevated)] p-1 rounded-xl border border-[var(--ui-border)]">
              <button
                type="button"
                onClick={() => setMode('single')}
                className={cn(
                  'flex-1 py-1.5 px-3 rounded-lg text-xs font-medium transition-all text-center',
                  mode === 'single'
                    ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm border border-[var(--ui-border)]'
                    : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
                )}
              >
                Один узел (HTTPS)
              </button>
              <button
                type="button"
                onClick={() => setMode('service_graph')}
                className={cn(
                  'flex-1 py-1.5 px-3 rounded-lg text-xs font-medium transition-all text-center',
                  mode === 'service_graph'
                    ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm border border-[var(--ui-border)]'
                    : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
                )}
              >
                Мульти-хост (ServiceGraph)
              </button>
            </div>

            {mode === 'single' ? (
              <div className="space-y-3">
                <div>
                  <div className="text-xs font-semibold text-[var(--ui-text-muted)] mb-2">Целевой сервис</div>
                  <div className="flex flex-wrap gap-2">
                    {config.targets.map((preset) => (
                      <button
                        key={preset.id}
                        type="button"
                        onClick={() => setPresetID(preset.id)}
                        className={presetID === preset.id ? 'btn-ui-primary text-xs w-auto' : 'btn-ui-secondary text-xs'}
                      >
                        {preset.label}
                      </button>
                    ))}
                    <button
                      type="button"
                      onClick={() => setPresetID('custom')}
                      className={selectedCustom ? 'btn-ui-primary text-xs w-auto' : 'btn-ui-secondary text-xs'}
                    >
                      Другой HTTPS URL
                    </button>
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

                <button onClick={start} disabled={!canStartSingle} className="btn-ui-primary w-full justify-center">
                  Проверить стратегии
                </button>
              </div>
            ) : (
              <div className="space-y-3">
                <div>
                  <div className="text-xs font-semibold text-[var(--ui-text-muted)] mb-2">
                    Пакет связанных доменов (ServiceGraph)
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {servicePresets.map((preset) => (
                      <button
                        key={preset.id}
                        type="button"
                        onClick={() => setSelectedServicePresetID(preset.id)}
                        className={
                          selectedServicePresetID === preset.id
                            ? 'btn-ui-primary text-xs w-auto'
                            : 'btn-ui-secondary text-xs'
                        }
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </div>

                {selectedServicePreset && (
                  <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 space-y-2 text-xs">
                    <div className="text-[var(--ui-text)] font-medium">{selectedServicePreset.description}</div>
                    <div className="flex flex-wrap gap-1.5">
                      {selectedServicePreset.hosts.map((host) => (
                        <span
                          key={host}
                          className="px-2 py-0.5 rounded bg-[var(--ui-panel)] border border-[var(--ui-border)] font-mono text-[11px] text-[var(--ui-text-muted)]"
                        >
                          {host}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                <button
                  type="button"
                  onClick={startServiceGraph}
                  disabled={!canStartGraph}
                  className="btn-ui-primary w-full justify-center"
                >
                  Запустить подбор ServiceGraph
                </button>
              </div>
            )}
          </div>
        )}

        {/* Live Running Progress */}
        {running && (
          <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4 space-y-3">
            <div className="flex items-center justify-between gap-2 text-xs">
              <div className="font-semibold text-[var(--ui-text)]">
                {progress?.stage ? stageLabels[progress.stage] || progress.stage : 'Выполняется проверка'}
              </div>
              <span className="font-mono text-[var(--ui-text-muted)]">{progress?.percent ?? 10}%</span>
            </div>

            <div className="w-full h-2 rounded-full bg-[var(--ui-bg)] overflow-hidden border border-[var(--ui-border)]">
              <div
                className="h-full bg-emerald-500 transition-all duration-300"
                style={{ width: `${progressPercent}%` }}
              />
            </div>

            <div className="flex items-center justify-between gap-3 text-xs">
              <span className="text-[var(--ui-text-muted)]">
                {progress?.message || 'Проверяем и безопасно восстанавливаем состояние...'}
              </span>
              <button
                type="button"
                onClick={handleCancel}
                disabled={cancelling}
                className="text-red-400 hover:underline shrink-0 font-medium"
              >
                {cancelling ? 'Отмена...' : 'Отменить'}
              </button>
            </div>
          </div>
        )}

        {/* Single-Host Result */}
        {result && !running && !managed?.active && (
          <div className="space-y-3">
            <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4 font-medium text-sm">
              {resultMessages[result.status] || 'Получен неизвестный результат проверки.'}
            </div>

            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-xs">
              <dt className="text-[var(--ui-text-muted)]">Цель</dt>
              <dd>{redactTarget(result.target)}</dd>

              <dt className="text-[var(--ui-text-muted)]">Диагноз</dt>
              <dd>
                {diagnosisKind ? (
                  <div className="space-y-0.5">
                    <div>{diagnosisExplanations[diagnosisKind] || diagnosisKind}</div>
                    <div className="font-mono text-[11px] text-[var(--ui-text-muted)]">
                      {diagnosisKind} · {confidenceLabels[diagnosisConfidence] || diagnosisConfidence} ({diagnosisConfidence})
                    </div>
                  </div>
                ) : (
                  'не сформирован'
                )}
              </dd>

              <dt className="text-[var(--ui-text-muted)]">Текущие endpoints</dt>
              <dd>
                {result.scope_edge_count
                  ? `${result.scope_edge_count} проверено · ${result.scope_status || 'PENDING'}`
                  : 'не сформировано'}
              </dd>

              <dt className="text-[var(--ui-text-muted)]">Стратегия</dt>
              <dd>{strategyLabel(result.selected_strategy_id)}</dd>

              <dt className="text-[var(--ui-text-muted)]">Исходное состояние</dt>
              <dd>{result.state_restored ? 'восстановлено' : 'не подтверждено'}</dd>
            </dl>

            {/* Candidate Outcomes */}
            {result.candidate_outcomes && result.candidate_outcomes.length > 0 && (
              <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 space-y-2 text-xs">
                <div className="font-semibold text-[var(--ui-text)]">Проверенные стратегии</div>
                <div className="divide-y divide-[var(--ui-border)]">
                  {result.candidate_outcomes.map((item, idx) => (
                    <div key={`${item.strategy_id}-${idx}`} className="py-1.5 flex items-center justify-between gap-3">
                      <span className="font-medium">{strategyLabel(item.strategy_id)}</span>
                      <span
                        className={cn(
                          'text-[11px]',
                          item.outcome === 'VERIFIED_FIXED'
                            ? 'text-emerald-400 font-semibold'
                            : 'text-[var(--ui-text-muted)]'
                        )}
                      >
                        {candidateOutcomeLabels[item.outcome] || item.outcome}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Limitations */}
            {result.limitations && result.limitations.length > 0 && (
              <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 space-y-1 text-xs">
                <div className="font-semibold text-amber-400">Ограничения и примечания проверки</div>
                <ul className="space-y-1 text-[var(--ui-text-muted)] list-disc list-inside">
                  {result.limitations.map((lim, idx) => (
                    <li key={idx}>{explainLimitation(lim)}</li>
                  ))}
                </ul>
              </div>
            )}

            {/* Lifecycle Errors */}
            {result.lifecycle_errors && result.lifecycle_errors.length > 0 && (
              <div className="rounded-lg border border-red-500/35 bg-red-500/10 p-3 space-y-1 text-xs">
                <div className="font-semibold text-red-400">Ошибки жизненного цикла</div>
                <ul className="space-y-1 text-[var(--ui-text)] list-disc list-inside">
                  {result.lifecycle_errors.map((errItem, idx) => (
                    <li key={idx}>{explainLifecycleError(errItem.code)}</li>
                  ))}
                </ul>
              </div>
            )}

            {result.recommendation?.history_used && (
              <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 text-xs text-[var(--ui-text-muted)]">
                Порядок проверки учитывал предыдущий совместимый результат:{' '}
                {(result.recommendation.reason_codes || []).join(', ') || 'совместимая история'}. Это не является гарантией результата.
              </div>
            )}

            {result.status === 'COMPLETED_SELECTED' && result.apply_available && (
              <button onClick={apply} className="btn-ui-primary w-full justify-center">
                Применить
              </button>
            )}
            {result.status === 'COMPLETED_SELECTED' && !result.apply_available && (
              <div className="text-sm text-red-400">
                Проверенная стратегия не получила разрешение на применение.
              </div>
            )}
            <button onClick={onClose} className="btn-ui-secondary w-full justify-center">
              Закрыть результат
            </button>
          </div>
        )}

        {/* Multi-Host ServiceGraph Result */}
        {serviceGraphResult && !running && (
          <div className="space-y-3">
            <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-4 space-y-2 text-xs">
              <div className="flex items-center justify-between gap-2">
                <span className="font-semibold text-sm text-[var(--ui-text)]">
                  Результат ServiceGraph ({serviceGraphResult.preset_id})
                </span>
                <span
                  className={cn(
                    'px-2 py-0.5 rounded font-mono text-[10px] font-semibold',
                    serviceGraphResult.applied
                      ? 'bg-emerald-500/20 text-emerald-400'
                      : 'bg-amber-500/20 text-amber-400'
                  )}
                >
                  {serviceGraphResult.applied ? 'ПРИМЕНЕНО' : serviceGraphResult.status}
                </span>
              </div>
              <div className="text-[var(--ui-text-muted)]">{serviceGraphResult.summary}</div>

              {serviceGraphResult.hosts && serviceGraphResult.hosts.length > 0 && (
                <div className="pt-1 space-y-1">
                  <div className="font-medium text-[var(--ui-text-muted)]">Статус узлов пакета:</div>
                  <div className="flex flex-wrap gap-1.5">
                    {serviceGraphResult.hosts.map((host) => {
                      const verified = serviceGraphResult.verified_hosts?.includes(host);
                      return (
                        <span
                          key={host}
                          className={cn(
                            'px-2 py-0.5 rounded border font-mono text-[11px]',
                            verified
                              ? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-400'
                              : 'border-[var(--ui-border)] bg-[var(--ui-panel)] text-[var(--ui-text-muted)]'
                          )}
                        >
                          {verified ? '✓ ' : '○ '}
                          {host}
                        </span>
                      );
                    })}
                  </div>
                </div>
              )}

              {serviceGraphResult.errors && serviceGraphResult.errors.length > 0 && (
                <div className="rounded border border-red-500/30 bg-red-500/10 p-2 text-red-400 space-y-1">
                  {serviceGraphResult.errors.map((errText, idx) => (
                    <div key={idx}>{errText}</div>
                  ))}
                </div>
              )}
            </div>

            {serviceGraphResult.applied && (
              <button
                type="button"
                onClick={revertServiceGraph}
                disabled={running}
                className="btn-ui-secondary w-full justify-center"
              >
                Отключить ServiceGraph
              </button>
            )}

            <button
              type="button"
              onClick={() => setServiceGraphResult(null)}
              className="btn-ui-secondary w-full justify-center"
            >
              Новый подбор
            </button>
          </div>
        )}

        {/* History Ledger Summary */}
        {historySummary && (
          <div className="rounded-lg border border-[var(--ui-border)] bg-[var(--ui-surface-elevated)] p-3 flex items-center justify-between gap-3 text-xs">
            <div className="space-y-0.5">
              <div className="font-semibold text-[var(--ui-text)]">История подбора на текущей сети</div>
              <div className="text-[11px] text-[var(--ui-text-muted)]">
                Записей: {historySummary.record_count} · Успешных: {historySummary.verified_count} · Неудачных:{' '}
                {historySummary.negative_count} ·{' '}
                {historySummary.context_ready ? 'Контекст активен' : 'Контекст пуст'}
              </div>
            </div>
            <button
              type="button"
              onClick={handleClearHistory}
              disabled={running || clearingHistory || historySummary.record_count === 0}
              className="btn-ui-secondary text-xs py-1.5 px-3 shrink-0"
            >
              {clearingHistory ? 'Очистка...' : 'Очистить историю'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
};
