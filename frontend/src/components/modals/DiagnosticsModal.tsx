import React, { useState } from 'react';
import { cn } from '../../lib/cn';
import { UITerminal, UISpinner, UIX, UICheck, UIZap } from '../icons';
import { backendService } from '../../services/backend';
import { engine } from '../../../wailsjs/go/models';

interface LegacyDiagnosticResult {
  Component: string;
  Status: string;
  Details: string;
  IsError?: boolean;
}

interface DiagnosticsModalProps {
  isOpen: boolean;
  isRunning: boolean;
  results: LegacyDiagnosticResult[];
  doctorResult?: engine.DoctorResult | null;
  onClose: () => void;
  onRunMode?: (mode: string) => void;
}

export const DiagnosticsModal: React.FC<DiagnosticsModalProps> = ({
  isOpen,
  isRunning,
  results,
  doctorResult: initialDoctorResult,
  onClose,
  onRunMode,
}) => {
  const [activeTab, setActiveTab] = useState<'quick' | 'extended' | 'comparison'>('quick');
  const [doctorResult, setDoctorResult] = useState<engine.DoctorResult | null>(initialDoctorResult || null);
  const [comparisonResult, setComparisonResult] = useState<engine.BypassComparisonResult | null>(null);
  const [isLocalRunning, setIsLocalRunning] = useState(false);
  const [copied, setCopied] = useState(false);
  const [expandedProbes, setExpandedProbes] = useState<Record<string, boolean>>({});
  const [activeRunId, setActiveRunId] = useState<string>('');
  const [progress, setProgress] = useState<{
    runId: string;
    completed: number;
    total: number;
    percent: number;
    running: string[];
    lastCompleted: string;
    elapsedMs: number;
  }>({
    runId: '',
    completed: 0,
    total: 0,
    percent: 0,
    running: [],
    lastCompleted: '',
    elapsedMs: 0,
  });

  // Subscribe to real-time Doctor progress events
  React.useEffect(() => {
    const w = window as unknown as { runtime?: { EventsOn?: (event: string, cb: (data: unknown) => void) => () => void; EventsOff?: (event: string) => void } };
    if (!w.runtime?.EventsOn) return;

    const unregProgress = w.runtime.EventsOn('doctor_progress', (data: unknown) => {
      const p = data as { runId: string; completed: number; total: number; percent: number; running: string[]; lastCompleted: string; elapsedMs: number };
      if (p) {
        setProgress(p);
        if (p.runId) setActiveRunId(p.runId);
      }
    });

    const unregComplete = w.runtime.EventsOn('doctor_complete', (data: unknown) => {
      const d = data as { runId: string; result: engine.DoctorResult };
      if (d?.result) {
        setDoctorResult(d.result);
        setIsLocalRunning(false);
      }
    });

    const unregCancelled = w.runtime.EventsOn('doctor_cancelled', () => {
      setIsLocalRunning(false);
    });

    return () => {
      if (unregProgress) unregProgress();
      if (unregComplete) unregComplete();
      if (unregCancelled) unregCancelled();
    };
  }, []);

  // Sync initial prop when it updates
  React.useEffect(() => {
    if (initialDoctorResult) {
      setDoctorResult(initialDoctorResult);
    }
  }, [initialDoctorResult]);
  if (!isOpen) return null;

  const running = isRunning || isLocalRunning;

  const handleTabChange = async (tab: 'quick' | 'extended' | 'comparison') => {
    setActiveTab(tab);
    if (tab === 'comparison') {
      if (!comparisonResult) {
        setIsLocalRunning(true);
        try {
          const res = await backendService.runBypassComparison();
          setComparisonResult(res);
        } catch (err) {
          console.error('Bypass comparison error:', err);
        } finally {
          setIsLocalRunning(false);
        }
      }
    } else {
      if (onRunMode) {
        onRunMode(tab);
      } else {
        setIsLocalRunning(true);
        try {
          const res = await backendService.runDoctor(tab);
          setDoctorResult(res);
        } catch (err) {
          console.error('Doctor run error:', err);
        } finally {
          setIsLocalRunning(false);
        }
      }
    }
  };

  const handleCopyReport = async () => {
    try {
      let reportText = '';
      if (activeTab === 'comparison' && comparisonResult) {
        reportText = comparisonResult.overallSummary || 'A/B comparison completed';
      } else if (doctorResult) {
        reportText = await backendService.generateDiagnosticReport(doctorResult);
      } else {
        reportText = results.map(r => `[${r.Status}] ${r.Component}: ${r.Details}`).join('\n');
      }
      await navigator.clipboard.writeText(reportText);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy report:', err);
    }
  };

  const handleOpenLogs = async () => {
    try {
      await backendService.openLogsFolder();
    } catch (err) {
      console.error('Failed to open logs folder:', err);
    }
  };

  const toggleProbeExpand = (id: string) => {
    setExpandedProbes(prev => ({ ...prev, [id]: !prev[id] }));
  };

  const renderStatusBadge = (status: string) => {
    switch (status) {
      case 'PASS':
        return <span className="text-[10px] px-1.5 py-0.5 rounded font-mono bg-emerald-500/20 text-emerald-400 font-semibold">✓ PASS</span>;
      case 'FAIL':
        return <span className="text-[10px] px-1.5 py-0.5 rounded font-mono bg-red-500/20 text-red-400 font-semibold">✕ FAIL</span>;
      case 'WARNING':
        return <span className="text-[10px] px-1.5 py-0.5 rounded font-mono bg-amber-500/20 text-amber-400 font-semibold">! WARN</span>;
      case 'NOT_VERIFIED':
        return <span className="text-[10px] px-1.5 py-0.5 rounded font-mono bg-zinc-500/20 text-zinc-400 font-semibold">? MANUAL</span>;
      case 'INFO':
        return <span className="text-[10px] px-1.5 py-0.5 rounded font-mono bg-blue-500/20 text-blue-400 font-semibold">ℹ INFO</span>;
      default:
        return <span className="text-[10px] px-1.5 py-0.5 rounded font-mono bg-zinc-500/20 text-zinc-400">{status}</span>;
    }
  };

  const renderOverallBanner = () => {
    if (!doctorResult) return null;
    const { overallStatus, passCount, failCount, warnCount } = doctorResult;

    let bgClass = 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400';
    let text = 'Все системы и целевые сервисы доступны';
    if (overallStatus === 'CRITICAL') {
      bgClass = 'bg-red-500/20 border-red-500/40 text-red-400';
      text = 'Критическая ошибка: требуются права администратора или ядро повреждено';
    } else if (overallStatus === 'DEGRADED') {
      bgClass = 'bg-orange-500/15 border-orange-500/35 text-orange-400';
      text = 'Часть сервисов заблокирована (рекомендуется смена профиля или AutoTune)';
    } else if (overallStatus === 'WARNING') {
      bgClass = 'bg-amber-500/15 border-amber-500/35 text-amber-400';
      text = 'Обнаружены системные предупреждения (рекомендуется проверка стека)';
    }

    return (
      <div className={cn('p-3 rounded-xl border flex items-center justify-between text-xs mb-3', bgClass)}>
        <div>
          <div className="font-semibold">{text}</div>
          <div className="text-[11px] opacity-80 mt-0.5">
            Успешно: {passCount} | Ошибок: {failCount} | Предупреждений: {warnCount}
          </div>
        </div>
        <div className="text-right text-[11px] font-mono opacity-75">
          {doctorResult.activeProfile ? `Профиль: ${doctorResult.activeProfile}` : 'Движок остановлен'}
        </div>
      </div>
    );
  };

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 app-no-drag"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl flex flex-col max-h-[88vh] p-5 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--ui-border)] pb-3 mb-3">
          <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ui-text)]">
            <UITerminal className="w-4 h-4 text-emerald-400" />
            <span>Проверка системы — UNBOUND Doctor</span>
          </div>
          <button
            onClick={onClose}
            className="text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] transition-colors p-1"
          >
            <UIX className="w-4 h-4" />
          </button>
        </div>

        {/* Mode Selector Tabs */}
        <div className="flex items-center gap-2 mb-3 bg-[var(--ui-surface-elevated)] p-1 rounded-xl border border-[var(--ui-border)]">
          <button
            onClick={() => handleTabChange('quick')}
            disabled={running}
            className={cn(
              'flex-1 py-1.5 px-3 rounded-lg text-xs font-medium transition-all text-center',
              activeTab === 'quick'
                ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm border border-[var(--ui-border)]'
                : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
            )}
          >
            Быстрая проверка
          </button>
          <button
            onClick={() => handleTabChange('extended')}
            disabled={running}
            className={cn(
              'flex-1 py-1.5 px-3 rounded-lg text-xs font-medium transition-all text-center',
              activeTab === 'extended'
                ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm border border-[var(--ui-border)]'
                : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
            )}
          >
            Расширенная проверка
          </button>
          <button
            onClick={() => handleTabChange('comparison')}
            disabled={running}
            className={cn(
              'flex-1 py-1.5 px-3 rounded-lg text-xs font-medium transition-all text-center',
              activeTab === 'comparison'
                ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm border border-[var(--ui-border)]'
                : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
            )}
          >
            Сравнение A/B (До / После)
          </button>
        </div>

        {/* Content Body */}
        <div className="overflow-y-auto space-y-3 flex-1 pr-1">
          {running ? (
            <div className="flex flex-col py-8 px-4 gap-4 bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-2xl">
              <div className="flex items-center justify-between text-xs">
                <div className="flex items-center gap-2 font-medium text-[var(--ui-text)]">
                  <UISpinner className="w-4 h-4 animate-spin text-emerald-400" />
                  <span>
                    {activeTab === 'comparison'
                      ? 'Выполняется A/B тестирование соединений...'
                      : 'Диагностика проверяет окружение и сервисы...'}
                  </span>
                </div>
                <span className="font-mono text-xs font-semibold text-emerald-400">
                  {progress.percent}% ({progress.completed} / {progress.total || '?'})
                </span>
              </div>

              {/* Progress bar */}
              <div className="w-full bg-black/30 h-2 rounded-full overflow-hidden border border-white/5">
                <div
                  className="bg-emerald-500 h-full transition-all duration-300 rounded-full"
                  style={{ width: `${Math.max(5, Math.min(100, progress.percent))}%` }}
                />
              </div>

              {/* Concurrently running probes */}
              {progress.running && progress.running.length > 0 && (
                <div className="text-[11px] text-[var(--ui-text-muted)] space-y-1">
                  <span className="font-medium text-[var(--ui-text)]">Выполняется сейчас:</span>
                  <div className="flex flex-wrap gap-1.5 mt-1">
                    {progress.running.map((rName, idx) => (
                      <span key={idx} className="px-2 py-0.5 rounded-md bg-emerald-500/10 text-emerald-300 font-mono text-[10px] border border-emerald-500/20 animate-pulse">
                        {rName}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Last completed probe */}
              {progress.lastCompleted && (
                <div className="text-[11px] text-[var(--ui-text-muted)] flex items-center gap-1.5">
                  <UICheck className="w-3.5 h-3.5 text-emerald-400" />
                  <span>Завершено: <span className="text-[var(--ui-text)] font-mono">{progress.lastCompleted}</span></span>
                </div>
              )}

              {/* Elapsed time and Cancel button */}
              <div className="flex items-center justify-between pt-2 border-t border-[var(--ui-border)] text-xs">
                <span className="text-[11px] font-mono text-[var(--ui-text-muted)]">
                  Прошло: {((progress.elapsedMs || 0) / 1000).toFixed(1)}с
                </span>
                <button
                  onClick={async () => {
                    try {
                      await backendService.cancelDoctor(activeRunId);
                    } catch (err) {
                      console.error(err);
                    } finally {
                      setIsLocalRunning(false);
                    }
                  }}
                  className="btn-ui-secondary text-xs px-3 py-1 text-red-400 hover:text-red-300 border-red-500/30 hover:bg-red-500/10"
                >
                  Отменить проверку
                </button>
              </div>
            </div>
          ) : activeTab === 'comparison' ? (
            /* A/B Comparison View */
            <div className="space-y-3 text-xs">
              {comparisonResult ? (
                <>
                  <div className="p-3 bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-xl">
                    <div className="font-semibold text-[var(--ui-text)] mb-1">
                      Результаты A/B теста для профиля «{comparisonResult.profileName}»
                    </div>
                    <div className="text-[11px] text-[var(--ui-text-muted)]">
                      {comparisonResult.overallSummary}
                    </div>
                  </div>

                  <div className="border border-[var(--ui-border)] rounded-xl overflow-hidden">
                    <table className="w-full text-left border-collapse">
                      <thead>
                        <tr className="bg-[var(--ui-surface-elevated)] border-b border-[var(--ui-border)] text-[11px] text-[var(--ui-text-muted)]">
                          <th className="p-2.5">Сервис</th>
                          <th className="p-2.5">Проверка</th>
                          <th className="p-2.5 text-center">Прямой доступ</th>
                          <th className="p-2.5 text-center">С обходом</th>
                          <th className="p-2.5">Вердикт</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-[var(--ui-border)]">
                        {comparisonResult.items.map((item, idx) => (
                          <tr key={idx} className="hover:bg-white/[0.02] transition-colors">
                            <td className="p-2.5 font-medium text-[var(--ui-text)]">{item.service}</td>
                            <td className="p-2.5 text-[var(--ui-text-muted)]">{item.name}</td>
                            <td className="p-2.5 text-center">{renderStatusBadge(item.baseline.status)}</td>
                            <td className="p-2.5 text-center">{renderStatusBadge(item.profile.status)}</td>
                            <td className="p-2.5">
                              {item.verdict === 'FIXED_BY_PROFILE' && (
                                <span className="text-emerald-400 font-semibold">⭐ Восстановлено</span>
                              )}
                              {item.verdict === 'REACHABLE_DIRECTLY' && (
                                <span className="text-zinc-300">✓ Доступно</span>
                              )}
                              {item.verdict === 'STILL_BLOCKED' && (
                                <span className="text-red-400">✕ Заблокировано</span>
                              )}
                              {item.verdict === 'BROKEN_BY_PROFILE' && (
                                <span className="text-amber-400 font-bold">⚠ СЛОМАНО ПРОФИЛЕМ</span>
                              )}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </>
              ) : (
                <div className="text-center py-12 text-[var(--ui-text-muted)]">
                  Нажмите «Запустить сравнение», чтобы оценить эффективность профиля относительно прямого доступа без обхода.
                </div>
              )}
            </div>
          ) : doctorResult ? (
            /* Doctor Groups View */
            <div className="space-y-3">
              {renderOverallBanner()}

              {doctorResult.groups.map((group) => (
                <div
                  key={group.id}
                  className="border border-[var(--ui-border)] rounded-xl bg-[var(--ui-surface-elevated)] overflow-hidden"
                >
                  <div className="flex items-center justify-between p-3 border-b border-[var(--ui-border)] bg-[var(--ui-panel)]">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-xs text-[var(--ui-text)]">{group.name}</span>
                      <span className="text-[11px] text-[var(--ui-text-muted)]">({group.summary})</span>
                    </div>
                    {renderStatusBadge(group.status)}
                  </div>

                  <div className="divide-y divide-[var(--ui-border)]">
                    {group.probes.map((probe) => {
                      const isExpanded = expandedProbes[probe.id];
                      return (
                        <div key={probe.id} className="p-2.5 text-xs hover:bg-white/[0.015] transition-colors">
                          <div
                            className="flex items-center justify-between cursor-pointer select-none"
                            onClick={() => toggleProbeExpand(probe.id)}
                          >
                            <div className="flex items-center gap-2">
                              <span className="text-[var(--ui-text)] font-medium">{probe.name}</span>
                              {probe.latency > 0 && (
                                <span className="text-[10px] font-mono text-[var(--ui-text-muted)]">
                                  {Math.round(probe.latency / 1000000)}ms
                                </span>
                              )}
                            </div>
                            <div className="flex items-center gap-2">
                              {renderStatusBadge(probe.status)}
                              <span className="text-[10px] text-[var(--ui-text-muted)]">
                                {isExpanded ? '▲' : '▼'}
                              </span>
                            </div>
                          </div>

                          {probe.details && !isExpanded && (
                            <p className="text-[11px] text-[var(--ui-text-muted)] mt-1 truncate">
                              {probe.details}
                            </p>
                          )}

                          {isExpanded && (
                            <div className="mt-2 pt-2 border-t border-[var(--ui-border)] text-[11px] space-y-1 bg-black/20 p-2 rounded-lg font-mono">
                              {probe.target && <div><span className="text-[var(--ui-text-muted)]">Цель:</span> {probe.target}</div>}
                              {probe.transport && <div><span className="text-[var(--ui-text-muted)]">Протокол:</span> {probe.transport}</div>}
                              {probe.resolvedIp && <div><span className="text-[var(--ui-text-muted)]">IP:</span> {probe.resolvedIp}</div>}
                              {probe.details && <div><span className="text-[var(--ui-text-muted)]">Детали:</span> {probe.details}</div>}
                              {probe.error && <div className="text-red-400"><span className="text-red-400/80">Ошибка:</span> {probe.error}</div>}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}

              {/* Manual Acceptance Checklist (Extended mode only) */}
              {doctorResult.manualItems && doctorResult.manualItems.length > 0 && (
                <div className="p-3.5 border border-[var(--ui-border)] rounded-xl bg-blue-500/5 text-xs space-y-2">
                  <div className="font-semibold text-blue-300 flex items-center gap-1.5">
                    <span>Рекомендуемый чек-лист ручной приёмки</span>
                  </div>
                  <div className="space-y-1.5 text-[11px] text-[var(--ui-text-muted)]">
                    {doctorResult.manualItems.map((item, idx) => (
                      <div key={idx} className="flex items-center gap-2">
                        <span className="text-blue-400">○</span>
                        <span>{item}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            /* Fallback Legacy Results */
            <div className="space-y-2">
              {results.map((res, idx) => (
                <div
                  key={idx}
                  className="p-3 border rounded-xl bg-[var(--ui-surface-elevated)] border-[var(--ui-border)] text-xs"
                >
                  <div className="flex justify-between items-start mb-1 font-semibold text-[var(--ui-text)]">
                    <span>{res.Component}</span>
                    <span
                      className={cn(
                        'text-[10px] px-1.5 py-0.5 rounded uppercase font-mono',
                        res.IsError ? 'bg-red-500/20 text-red-400' : 'bg-emerald-500/20 text-emerald-400'
                      )}
                    >
                      {res.Status}
                    </span>
                  </div>
                  <p className="text-[11px] text-[var(--ui-text-muted)]">{res.Details}</p>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="pt-3 border-t border-[var(--ui-border)] mt-3 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <button
              onClick={handleCopyReport}
              disabled={running}
              className="btn-ui-secondary text-xs px-3 py-1.5 flex items-center gap-1.5"
            >
              {copied ? <UICheck className="w-3.5 h-3.5 text-emerald-400" /> : <UITerminal className="w-3.5 h-3.5" />}
              <span>{copied ? 'Отчёт скопирован' : 'Копировать отчёт'}</span>
            </button>
            <button
              onClick={handleOpenLogs}
              className="btn-ui-secondary text-xs px-3 py-1.5"
            >
              Открыть логи
            </button>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={() => handleTabChange(activeTab)}
              disabled={running}
              className="btn-ui-secondary text-xs px-3 py-1.5 flex items-center gap-1"
            >
              <UIZap className="w-3.5 h-3.5" />
              <span>Повторить</span>
            </button>
            <button onClick={onClose} className="btn-ui-primary text-xs px-4 py-1.5">
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
