import React, { useState, useEffect } from 'react';
import { cn } from '../../lib/cn';
import { UISpinner, UIX, UICheck, UIZap } from '../icons';
import { backendService } from '../../services/backend';
import { eventBus } from '../../services/window';
import { engine } from '../../../wailsjs/go/models';

interface StrategyLabModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaveProfileSuccess?: (profileName: string) => void;
}

export const StrategyLabModal: React.FC<StrategyLabModalProps> = ({
  isOpen,
  onClose,
  onSaveProfileSuccess,
}) => {
  const [targetHost, setTargetHost] = useState('youtube.com');
  const [servicePreset, setServicePreset] = useState('YouTube');
  const [protocol, setProtocol] = useState('TLS1.3');
  const [isRunning, setIsRunning] = useState(false);
  const [progress, setProgress] = useState<{
    currentCandidateIndex: number;
    totalCandidates: number;
    currentCandidateName: string;
    baselineStatus: string;
    percent: number;
    elapsedMs: number;
  }>({
    currentCandidateIndex: 0,
    totalCandidates: 0,
    currentCandidateName: '',
    baselineStatus: '',
    percent: 0,
    elapsedMs: 0,
  });
  const [report, setReport] = useState<engine.StrategyLabReport | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [saveSuccessMsg, setSaveSuccessMsg] = useState<string | null>(null);
  const [profileNameInput, setProfileNameInput] = useState('');
  const [acknowledgeWarning, setAcknowledgeWarning] = useState<boolean>(false);
  // Subscribe to Strategy Lab progress events
  useEffect(() => {
    if (!eventBus?.onStrategyLabProgress) return;
    const unsub = eventBus.onStrategyLabProgress((data: unknown) => {
      const p = data as {
        currentCandidateIndex: number;
        totalCandidates: number;
        currentCandidateName: string;
        baselineStatus: string;
        percent: number;
        elapsedMs: number;
      };
      if (p) {
        setProgress(p);
      }
    });
    return () => {
      unsub?.();
    };
  }, []);

  if (!isOpen) return null;

  const handlePresetSelect = (preset: string, defaultDomain: string) => {
    setServicePreset(preset);
    setTargetHost(defaultDomain);
  };

  const handleStartDiscovery = async () => {
    if (!targetHost.trim()) {
      setErrorMsg('Укажите домен или IP цели');
      return;
    }
    setIsRunning(true);
    setReport(null);
    setErrorMsg(null);
    setSaveSuccessMsg(null);
    setAcknowledgeWarning(false);
    setProgress({ currentCandidateIndex: 0, totalCandidates: 0, currentCandidateName: '', baselineStatus: '', percent: 0, elapsedMs: 0 });

    try {
      const res = await backendService.runStrategyLab(targetHost.trim(), servicePreset, protocol, []);
      setReport(res);
      setAcknowledgeWarning(false);
      if (res?.bestCandidate) {
        setProfileNameInput(`Discovered - ${res.bestCandidate.candidate.name}`);
      }
    } catch (err) {
      console.error('Strategy Lab error:', err);
      setErrorMsg(String(err));
    } finally {
      setIsRunning(false);
    }
  };

  const handleSaveDiscoveredProfile = async () => {
    if (!report?.bestCandidate) return;
    const name = profileNameInput.trim() || `Discovered (${report.targetHost})`;
    try {
      await backendService.saveDiscoveredProfile(name, report.bestCandidate.candidate, report.targetHost);
      setSaveSuccessMsg(`Стратегия сохранена в пользовательский профиль «${name}»`);
      onSaveProfileSuccess?.(name);
    } catch (err) {
      console.error('Save profile error:', err);
      setErrorMsg(`Не удалось сохранить профиль: ${err}`);
    }
  };

  const renderAggressivenessBadge = (aggressiveness: number) => {
    switch (aggressiveness) {
      case 1:
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">Низкая</span>;
      case 2:
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-mono bg-blue-500/10 text-blue-400 border border-blue-500/20">Средняя</span>;
      case 3:
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/20">Высокая</span>;
      case 4:
      default:
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-mono bg-purple-500/10 text-purple-400 border border-purple-500/20">Эксперимент</span>;
    }
  };

  const renderValidationBadge = (status?: string, details?: string) => {
    switch (status) {
      case 'VERIFIED':
        return (
          <div className="flex flex-col gap-1 p-2.5 bg-emerald-500/10 border border-emerald-500/30 rounded-xl">
            <div className="text-[11px] text-emerald-400 font-mono font-semibold flex items-center gap-1.5">
              <UICheck className="w-3.5 h-3.5 text-emerald-400" />
              <span>✓ Проверено (VERIFIED)</span>
            </div>
            {details && <div className="text-[10px] text-emerald-300/80 pl-5">{details}</div>}
          </div>
        );
      case 'PARTIAL':
        return (
          <div className="flex flex-col gap-1 p-2.5 bg-amber-500/10 border border-amber-500/30 rounded-xl">
            <div className="text-[11px] text-amber-400 font-mono font-semibold flex items-center gap-1.5">
              <span className="text-amber-400">⚠</span>
              <span>⚠ Частично проверено (PARTIAL)</span>
            </div>
            {details && <div className="text-[10px] text-amber-200/90 pl-5">{details}</div>}
          </div>
        );
      case 'NOT_VERIFIED':
        return (
          <div className="flex flex-col gap-1 p-2.5 bg-blue-500/10 border border-blue-500/30 rounded-xl">
            <div className="text-[11px] text-blue-400 font-mono font-semibold flex items-center gap-1.5">
              <span className="text-blue-400">○</span>
              <span>○ Не проверено (NOT VERIFIED)</span>
            </div>
            {details && <div className="text-[10px] text-blue-200/90 pl-5">{details}</div>}
          </div>
        );
      case 'FAILED':
      default:
        return (
          <div className="flex flex-col gap-1 p-2.5 bg-red-500/10 border border-red-500/30 rounded-xl">
            <div className="text-[11px] text-red-400 font-mono font-semibold flex items-center gap-1.5">
              <span className="text-red-400">✕</span>
              <span>✕ Ошибка валидации (FAILED)</span>
            </div>
            {details && <div className="text-[10px] text-red-200/90 pl-5">{details}</div>}
          </div>
        );
    }
  };

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/80 backdrop-blur-sm p-4 app-no-drag"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl bg-[var(--ui-panel)] border border-[var(--ui-border-strong)] rounded-2xl flex flex-col max-h-[88vh] p-5 shadow-2xl space-y-4"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-[var(--ui-border)] pb-3">
          <div className="flex items-center gap-2 text-sm font-semibold text-[var(--ui-text)]">
            <UIZap className="w-4 h-4 text-emerald-400" />
            <span>Strategy Lab — Лаборатория поиска стратегий (Изолированный тест)</span>
          </div>
          <button
            onClick={onClose}
            className="text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] transition-colors p-1"
          >
            <UIX className="w-4 h-4" />
          </button>
        </div>

        {/* Form Inputs (Target & Protocol) */}
        {!isRunning && !report && (
          <div className="space-y-3 text-xs">
            <div>
              <span className="text-xs font-semibold text-[var(--ui-text-muted)]">Целевой сервис:</span>
              <div className="grid grid-cols-4 gap-2 mt-1.5">
                <button
                  type="button"
                  onClick={() => handlePresetSelect('YouTube', 'youtube.com')}
                  className={cn(
                    'p-2 rounded-xl border text-center transition-all',
                    servicePreset === 'YouTube' ? 'bg-[var(--ui-surface-elevated)] border-emerald-500 text-emerald-400 font-semibold' : 'border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
                  )}
                >
                  YouTube
                </button>
                <button
                  type="button"
                  onClick={() => handlePresetSelect('Discord', 'discord.com')}
                  className={cn(
                    'p-2 rounded-xl border text-center transition-all',
                    servicePreset === 'Discord' ? 'bg-[var(--ui-surface-elevated)] border-emerald-500 text-emerald-400 font-semibold' : 'border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
                  )}
                >
                  Discord
                </button>
                <button
                  type="button"
                  onClick={() => handlePresetSelect('Steam', 'store.steampowered.com')}
                  className={cn(
                    'p-2 rounded-xl border text-center transition-all',
                    servicePreset === 'Steam' ? 'bg-[var(--ui-surface-elevated)] border-emerald-500 text-emerald-400 font-semibold' : 'border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
                  )}
                >
                  Steam
                </button>
                <button
                  type="button"
                  onClick={() => setServicePreset('Custom')}
                  className={cn(
                    'p-2 rounded-xl border text-center transition-all',
                    servicePreset === 'Custom' ? 'bg-[var(--ui-surface-elevated)] border-emerald-500 text-emerald-400 font-semibold' : 'border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
                  )}
                >
                  Произвольный
                </button>
              </div>
            </div>

            <div className="grid grid-cols-3 gap-3">
              <div className="col-span-2 space-y-1">
                <span className="text-xs font-semibold text-[var(--ui-text-muted)]">Домен или IP цели:</span>
                <input
                  type="text"
                  value={targetHost}
                  onChange={(e) => setTargetHost(e.target.value)}
                  placeholder="domain.com"
                  className="w-full p-2.5 rounded-xl border bg-[var(--ui-surface-elevated)] border-[var(--ui-border)] text-[var(--ui-text)] font-mono text-xs focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div className="space-y-1">
                <span className="text-xs font-semibold text-[var(--ui-text-muted)]">Протокол:</span>
                <select
                  value={protocol}
                  onChange={(e) => setProtocol(e.target.value)}
                  className="w-full p-2.5 rounded-xl border bg-[var(--ui-surface-elevated)] border-[var(--ui-border)] text-[var(--ui-text)] text-xs focus:outline-none"
                >
                  <option value="TLS1.3">HTTPS / TLS 1.3</option>
                  <option value="TLS1.2">HTTPS / TLS 1.2</option>
                  <option value="HTTP">HTTP (порт 80)</option>
                  <option value="QUIC">QUIC / UDP</option>
                </select>
              </div>
            </div>

            <div className="p-3 bg-blue-500/10 border border-blue-500/20 rounded-xl text-[11px] text-blue-300 space-y-1">
              <div className="font-semibold">Безопасная изоляция WinDivert:</div>
              <p className="text-[var(--ui-text-muted)]">
                Strategy Lab автоматически изолирует процесс через строгий WinDivert raw-фильтр (<span className="font-mono text-white/90">--wf-raw=</span>) только для IP-адресов выбранной цели. Экспериментальные стратегии не затронут остальной интернет-трафик вашего компьютера.
              </p>
            </div>
          </div>
        )}

        {/* Live Progress View */}
        {isRunning && (
          <div className="flex flex-col py-8 px-4 gap-4 bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-2xl text-xs">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 font-medium text-[var(--ui-text)]">
                <UISpinner className="w-4 h-4 animate-spin text-emerald-400" />
                <span>Исследование стратегий для {targetHost}...</span>
              </div>
              <span className="font-mono font-semibold text-emerald-400">
                {progress.percent}% (Кандидат {progress.currentCandidateIndex} / {progress.totalCandidates || '?'})
              </span>
            </div>

            {/* Progress Bar */}
            <div className="w-full bg-black/30 h-2 rounded-full overflow-hidden border border-white/5">
              <div
                className="bg-emerald-500 h-full transition-all duration-300 rounded-full"
                style={{ width: `${Math.max(5, Math.min(100, progress.percent))}%` }}
              />
            </div>

            <div className="space-y-1 bg-black/20 p-3 rounded-xl border border-[var(--ui-border)] font-mono text-[11px]">
              <div><span className="text-[var(--ui-text-muted)]">Тестируемый кандидат:</span> <span className="text-white font-semibold">{progress.currentCandidateName || 'Подготовка...'}</span></div>
              <div><span className="text-[var(--ui-text-muted)]">{protocol === 'QUIC' ? 'Baseline QUIC:' : protocol === 'TLS1.3' ? 'Baseline TLS 1.3:' : protocol === 'TLS1.2' ? 'Baseline TLS 1.2:' : protocol === 'HTTP' ? 'Baseline HTTP:' : 'Прямой доступ (без обхода):'}</span> <span className={progress.baselineStatus === 'PASS' ? 'text-emerald-400' : 'text-red-400'}>{progress.baselineStatus || 'Проверка...'}</span></div>
              <div><span className="text-[var(--ui-text-muted)]">Прошедшее время:</span> {((progress.elapsedMs || 0) / 1000).toFixed(1)}с</div>
            </div>
          </div>
        )}

        {/* Results View */}
        {report && !isRunning && (
          <div className="space-y-3 overflow-y-auto max-h-[60vh] pr-1 text-xs">
            {report.baselineReachable && (
              <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl text-emerald-400 text-xs">
                ✓ Цель <span className="font-mono font-semibold">{report.targetHost}</span> доступна напрямую без обхода DPI.
              </div>
            )}

            {/* Best Candidate Banner */}
            {report.bestCandidate ? (
              <div className="p-4 bg-[var(--ui-surface-elevated)] border border-emerald-500/40 rounded-2xl space-y-3">
                <div className="flex items-start justify-between">
                  <div>
                    <div className="text-[10px] text-emerald-400 font-mono uppercase">Рекомендуемая стратегия</div>
                    <h4 className="text-sm font-semibold text-[var(--ui-text)] mt-0.5">{report.bestCandidate.candidate.name}</h4>
                    <div className="text-[11px] text-[var(--ui-text-muted)] mt-1">
                      {report.bestCandidate.details} (Источник: {report.bestCandidate.candidate.source})
                    </div>
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    {renderAggressivenessBadge(report.bestCandidate.candidate.aggressiveness)}
                    <span className="text-[10px] font-mono text-emerald-400">Счёт: {report.bestCandidate.score}</span>
                  </div>
                </div>

                {/* Service Validation Outcome */}
                {renderValidationBadge(report.validationStatus, report.validationDetails)}

                {/* Save as Profile controls based on ValidationStatus */}
                {report.validationStatus === 'FAILED' ? (
                  <div className="pt-2 border-t border-[var(--ui-border)] text-xs text-red-400 font-mono flex items-center gap-2">
                    <span>✕ Сохранение недоступно: найденная стратегия не прошла валидацию сервиса.</span>
                  </div>
                ) : (
                  <div className="space-y-2 pt-2 border-t border-[var(--ui-border)]">
                    {(report.validationStatus === 'PARTIAL' || report.validationStatus === 'NOT_VERIFIED') && (
                      <div className="flex items-center gap-2 text-[11px] text-amber-300">
                        <input
                          type="checkbox"
                          id="ackWarning"
                          checked={acknowledgeWarning}
                          onChange={(e) => setAcknowledgeWarning(e.target.checked)}
                          className="rounded border-[var(--ui-border)] bg-[var(--ui-panel)] text-emerald-500 focus:ring-0"
                        />
                        <label htmlFor="ackWarning" className="cursor-pointer select-none">
                          Я понимаю, что стратегия подтверждена только частично ({report.validationStatus}), и хочу сохранить её всё равно
                        </label>
                      </div>
                    )}

                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={profileNameInput}
                        onChange={(e) => setProfileNameInput(e.target.value)}
                        placeholder="Название нового профиля"
                        className="flex-1 p-2 rounded-xl border bg-[var(--ui-panel)] border-[var(--ui-border)] text-xs text-[var(--ui-text)] focus:outline-none"
                      />
                      <button
                        onClick={handleSaveDiscoveredProfile}
                        disabled={
                          (report.validationStatus === 'PARTIAL' || report.validationStatus === 'NOT_VERIFIED') && !acknowledgeWarning
                        }
                        className={cn(
                          "btn-ui-primary text-xs px-4 whitespace-nowrap",
                          ((report.validationStatus === 'PARTIAL' || report.validationStatus === 'NOT_VERIFIED') && !acknowledgeWarning) && "opacity-50 cursor-not-allowed"
                        )}
                      >
                        {report.validationStatus === 'VERIFIED' ? 'Сохранить профиль' : 'Сохранить всё равно'}
                      </button>
                    </div>
                  </div>
                )}
                {saveSuccessMsg && (
                  <div className="text-[11px] text-emerald-400 font-mono">{saveSuccessMsg}</div>
                )}
              </div>
            ) : (
              !report.baselineReachable && (
                <div className="p-4 bg-red-500/10 border border-red-500/30 rounded-xl text-red-400 text-xs">
                  Ни один из {report.totalCandidates} протестированных кандидатов не смог восстановить соединение. Рекомендуется проверить доступность цели или сменить протокол.
                </div>
              )
            )}

            {/* Table of all tested candidates */}
            {report.workingCandidates.length > 0 && (
              <div className="border border-[var(--ui-border)] rounded-xl overflow-hidden">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="bg-[var(--ui-surface-elevated)] border-b border-[var(--ui-border)] text-[11px] text-[var(--ui-text-muted)]">
                      <th className="p-2.5">Кандидат</th>
                      <th className="p-2.5 text-center">Агрессивность</th>
                      <th className="p-2.5 text-center">Успешно</th>
                      <th className="p-2.5 text-right">Латентность</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--ui-border)]">
                    {report.workingCandidates.map((c, idx) => (
                      <tr key={idx} className="hover:bg-white/[0.015] transition-colors">
                        <td className="p-2.5 font-medium text-[var(--ui-text)]">{c.candidate.name}</td>
                        <td className="p-2.5 text-center">{renderAggressivenessBadge(c.candidate.aggressiveness)}</td>
                        <td className="p-2.5 text-center font-mono text-emerald-400">{c.passCount}/{c.totalAttempts}</td>
                        <td className="p-2.5 text-right font-mono text-[var(--ui-text-muted)]">{Math.round(c.avgLatency / 1000000)}мс</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}

        {errorMsg && (
          <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-xl text-red-400 text-xs">
            {errorMsg}
          </div>
        )}

        {/* Footer Actions */}
        <div className="pt-3 border-t border-[var(--ui-border)] flex items-center justify-between text-xs">
          <button
            onClick={() => {
              setReport(null);
              setErrorMsg(null);
            }}
            disabled={isRunning}
            className="btn-ui-secondary text-xs px-3 py-1.5"
          >
            {report ? 'Новый поиск' : 'Сбросить'}
          </button>

          <div className="flex items-center gap-2">
            {!isRunning && !report && (
              <button
                onClick={handleStartDiscovery}
                className="btn-ui-primary text-xs px-4 py-1.5 flex items-center gap-1.5"
              >
                <UIZap className="w-3.5 h-3.5" />
                <span>Начать поиск</span>
              </button>
            )}
            <button onClick={onClose} className="btn-ui-secondary text-xs px-4 py-1.5">
              Закрыть
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
