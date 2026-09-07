import React, { useState, useEffect } from 'react';
import { UISelect } from '../UISelect';
import { UICheck, UISpinner } from '../icons';
import { cn } from '../../lib/cn';
import { backendService } from '../../services/backend';
import { engine } from '../../../wailsjs/go/models';

interface BypassListsViewProps {
  selectedList: string;
  hostlists: string[];
  handleSelectHostlist: (name: string) => void;
  hostlistContent: string;
  setHostlistContent: (content: string) => void;
  handleSaveHostlist: () => void;
  isSavingHostlist: boolean;
}

export const BypassListsView: React.FC<BypassListsViewProps> = ({
  selectedList,
  hostlists,
  handleSelectHostlist,
  hostlistContent,
  setHostlistContent,
  handleSaveHostlist,
  isSavingHostlist,
}) => {
  const [subTab, setSubTab] = useState<'curated' | 'autohostlist' | 'adaptive'>('curated');
  const [autoEntries, setAutoEntries] = useState<engine.AutoHostlistEntry[]>([]);
  const [adaptiveStates, setAdaptiveStates] = useState<engine.AdaptiveHostState[]>([]);
  const [newDomainInput, setNewDomainInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [actionFeedback, setActionFeedback] = useState<string | null>(null);

  const loadAutoHostlist = async () => {
    setIsLoading(true);
    try {
      const entries = await backendService.getAutoHostlistEntries();
      setAutoEntries(Array.isArray(entries) ? entries : []);
    } catch (err) {
      console.error('Failed to load autohostlist:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const loadAdaptiveStates = async () => {
    setIsLoading(true);
    try {
      const states = await backendService.getAdaptiveHostStates();
      setAdaptiveStates(Array.isArray(states) ? states : []);
    } catch (err) {
      console.error('Failed to load adaptive states:', err);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (subTab === 'autohostlist') {
      loadAutoHostlist();
    } else if (subTab === 'adaptive') {
      loadAdaptiveStates();
    }
  }, [subTab]);

  const handleAddDomain = async () => {
    if (!newDomainInput.trim()) return;
    try {
      await backendService.addAutoHostlistDomain(newDomainInput.trim(), 'Ручное добавление пользователем');
      setNewDomainInput('');
      setActionFeedback('Домен добавлен в автолист');
      loadAutoHostlist();
      setTimeout(() => setActionFeedback(null), 2500);
    } catch (err) {
      console.error(err);
      setActionFeedback(`Ошибка добавления: ${err}`);
    }
  };

  const handleRemoveDomain = async (domain: string) => {
    try {
      await backendService.removeAutoHostlistDomain(domain);
      setActionFeedback(`Домен ${domain} удалён`);
      loadAutoHostlist();
      setTimeout(() => setActionFeedback(null), 2500);
    } catch (err) {
      console.error(err);
    }
  };

  const handlePromoteDomain = async (domain: string) => {
    try {
      await backendService.promoteAutoHostlistDomain(domain, 'other.txt');
      setActionFeedback(`Домен ${domain} перенесён в постоянный список other.txt`);
      loadAutoHostlist();
      setTimeout(() => setActionFeedback(null), 2500);
    } catch (err) {
      console.error(err);
    }
  };

  const handleClearAutoList = async () => {
    if (!confirm('Вы уверены, что хотите очистить список автоматически обнаруженных доменов?')) return;
    try {
      await backendService.clearAutoHostlist();
      setActionFeedback('Автолист полностью очищен');
      loadAutoHostlist();
      setTimeout(() => setActionFeedback(null), 2500);
    } catch (err) {
      console.error(err);
    }
  };

  const handleResetAdaptive = async () => {
    try {
      await backendService.resetAdaptiveHostState();
      setActionFeedback('Адаптивное состояние хостов сброшено');
      loadAdaptiveStates();
      setTimeout(() => setActionFeedback(null), 2500);
    } catch (err) {
      console.error(err);
    }
  };

  return (
    <div className="flex-1 flex flex-col gap-3">
      {/* Sub-Tab Navigation */}
      <div className="flex bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-1 gap-1">
        <button
          type="button"
          onClick={() => setSubTab('curated')}
          className={cn(
            'flex-1 py-1.5 px-3 rounded-md text-xs font-semibold transition-all text-center',
            subTab === 'curated'
              ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm'
              : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
          )}
        >
          Курируемые списки
        </button>
        <button
          type="button"
          onClick={() => setSubTab('autohostlist')}
          className={cn(
            'flex-1 py-1.5 px-3 rounded-md text-xs font-semibold transition-all text-center',
            subTab === 'autohostlist'
              ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm'
              : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
          )}
        >
          Автолист (AutoHostlist)
        </button>
        <button
          type="button"
          onClick={() => setSubTab('adaptive')}
          className={cn(
            'flex-1 py-1.5 px-3 rounded-md text-xs font-semibold transition-all text-center',
            subTab === 'adaptive'
              ? 'bg-[var(--ui-panel)] text-[var(--ui-text)] shadow-sm'
              : 'text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]'
          )}
        >
          Адаптивное состояние
        </button>
      </div>

      {actionFeedback && (
        <div className="p-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-mono">
          {actionFeedback}
        </div>
      )}

      {subTab === 'curated' ? (
        /* Curated Hostlists Panel */
        <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <span className="text-xs font-semibold text-[var(--ui-text-muted)] uppercase">Файл списка:</span>
            <UISelect
              value={selectedList}
              options={hostlists}
              onChange={handleSelectHostlist}
            />
          </div>

          <div className="flex flex-wrap gap-1">
            <button
              type="button"
              onClick={() => {
                const preset = ['youtube.com', 'googlevideo.com', 'ytimg.com', 'youtu.be'];
                const existing = new Set(hostlistContent.split('\n').map((l) => l.trim().toLowerCase()));
                const toAdd = preset.filter((d) => !existing.has(d));
                if (toAdd.length > 0) {
                  setHostlistContent(
                    hostlistContent.trim() ? hostlistContent.trim() + '\n' + toAdd.join('\n') : toAdd.join('\n')
                  );
                }
              }}
              className="px-2 py-1 text-[11px] font-semibold rounded bg-[var(--ui-panel)] border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]"
            >
              + YouTube
            </button>
            <button
              type="button"
              onClick={() => {
                const preset = ['discord.com', 'discord.gg', 'discord.media', 'discordapp.com'];
                const existing = new Set(hostlistContent.split('\n').map((l) => l.trim().toLowerCase()));
                const toAdd = preset.filter((d) => !existing.has(d));
                if (toAdd.length > 0) {
                  setHostlistContent(
                    hostlistContent.trim() ? hostlistContent.trim() + '\n' + toAdd.join('\n') : toAdd.join('\n')
                  );
                }
              }}
              className="px-2 py-1 text-[11px] font-semibold rounded bg-[var(--ui-panel)] border border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:text-[var(--ui-text)]"
            >
              + Discord
            </button>
          </div>

          <textarea
            value={hostlistContent}
            onChange={(e) => setHostlistContent(e.target.value)}
            className="w-full h-48 p-3 font-mono text-xs border rounded-lg bg-[var(--ui-panel)] text-[var(--ui-text)] border-[var(--ui-border)] focus:outline-none resize-none"
            placeholder="domain.com"
            spellCheck={false}
          />

          <button
            onClick={handleSaveHostlist}
            disabled={isSavingHostlist}
            className="btn-ui-primary w-full"
          >
            {isSavingHostlist ? (
              <span className="flex items-center justify-center gap-2">
                <UISpinner className="w-4 h-4 animate-spin" />
                <span>Сохранение...</span>
              </span>
            ) : (
              <span className="flex items-center justify-center gap-2">
                <UICheck className="w-4 h-4" />
                <span>Сохранить список</span>
              </span>
            )}
          </button>
        </div>
      ) : subTab === 'autohostlist' ? (
        /* AutoHostlist Dynamic Panel */
        <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 flex flex-col gap-3 text-xs">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-semibold text-sm text-[var(--ui-text)]">Автоматически обнаруженные домены</h3>
              <p className="text-[11px] text-[var(--ui-text-muted)] mt-0.5">
                Домены, для которых winws2 зафиксировал блокировку по обратной связи (ретрансмиссии / RST).
              </p>
            </div>
            <button
              onClick={handleClearAutoList}
              className="text-[11px] text-red-400 hover:underline"
            >
              Очистить автолист
            </button>
          </div>

          {/* Manual Add Toolbar */}
          <div className="flex gap-2 pt-1">
            <input
              type="text"
              value={newDomainInput}
              onChange={(e) => setNewDomainInput(e.target.value)}
              placeholder="example.com"
              className="flex-1 p-2 rounded-xl border bg-[var(--ui-panel)] border-[var(--ui-border)] text-xs text-[var(--ui-text)] font-mono focus:outline-none"
            />
            <button
              onClick={handleAddDomain}
              className="btn-ui-secondary text-xs px-3"
            >
              + Добавить
            </button>
          </div>

          {/* Table of detected domains */}
          <div className="border border-[var(--ui-border)] rounded-xl overflow-hidden mt-1">
            {isLoading ? (
              <div className="p-8 flex justify-center"><UISpinner className="w-6 h-6 animate-spin text-emerald-400" /></div>
            ) : autoEntries.length === 0 ? (
              <div className="p-6 text-center text-[var(--ui-text-muted)] text-xs">
                Автоматически обнаруженных доменов пока нет. Они появятся здесь при обнаружении блокировок.
              </div>
            ) : (
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-[var(--ui-panel)] border-b border-[var(--ui-border)] text-[11px] text-[var(--ui-text-muted)]">
                    <th className="p-2.5">Домен</th>
                    <th className="p-2.5">Причина</th>
                    <th className="p-2.5 text-center">Хиты</th>
                    <th className="p-2.5 text-right">Действия</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--ui-border)]">
                  {autoEntries.map((item, idx) => (
                    <tr key={idx} className="hover:bg-white/[0.015] transition-colors font-mono text-[11px]">
                      <td className="p-2.5 font-semibold text-[var(--ui-text)]">{item.domain}</td>
                      <td className="p-2.5 text-[var(--ui-text-muted)]">{item.reason}</td>
                      <td className="p-2.5 text-center text-emerald-400">{item.hitCount}</td>
                      <td className="p-2.5 text-right space-x-2">
                        <button
                          onClick={() => handlePromoteDomain(item.domain)}
                          className="text-emerald-400 hover:underline text-[10px]"
                          title="Перенести в постоянный список"
                        >
                          В постоянный
                        </button>
                        <button
                          onClick={() => handleRemoveDomain(item.domain)}
                          className="text-red-400 hover:underline text-[10px]"
                          title="Удалить из автолиста"
                        >
                          Удалить
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      ) : (
        /* Adaptive Per-Host State Panel */
        <div className="bg-[var(--ui-surface-elevated)] border border-[var(--ui-border)] rounded-[var(--ui-radius)] p-4 flex flex-col gap-3 text-xs">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-semibold text-sm text-[var(--ui-text)]">Адаптивное состояние хостов (Circular)</h3>
              <p className="text-[11px] text-[var(--ui-text-muted)] mt-0.5">
                Текущая выбранная стратегия для каждого домена в режиме Adaptive (Experimental).
              </p>
            </div>
            <button
              onClick={handleResetAdaptive}
              className="text-[11px] text-amber-400 hover:underline"
            >
              Сбросить состояние
            </button>
          </div>

          <div className="border border-[var(--ui-border)] rounded-xl overflow-hidden mt-1">
            {isLoading ? (
              <div className="p-8 flex justify-center"><UISpinner className="w-6 h-6 animate-spin text-emerald-400" /></div>
            ) : adaptiveStates.length === 0 ? (
              <div className="p-6 text-center text-[var(--ui-text-muted)] text-xs">
                Адаптивное состояние пусто. Оно заполняется при активном профиле Adaptive (Experimental).
              </div>
            ) : (
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-[var(--ui-panel)] border-b border-[var(--ui-border)] text-[11px] text-[var(--ui-text-muted)]">
                    <th className="p-2.5">Хост</th>
                    <th className="p-2.5">Стратегия</th>
                    <th className="p-2.5 text-center">Уверенность</th>
                    <th className="p-2.5 text-center">Ошибок</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[var(--ui-border)]">
                  {adaptiveStates.map((st, idx) => (
                    <tr key={idx} className="hover:bg-white/[0.015] transition-colors font-mono text-[11px]">
                      <td className="p-2.5 font-semibold text-[var(--ui-text)]">{st.host}</td>
                      <td className="p-2.5 text-emerald-400">{st.strategyName || `Стратегия #${st.strategyIndex}`}</td>
                      <td className="p-2.5 text-center">
                        <span className={cn(
                          'px-1.5 py-0.5 rounded text-[10px] uppercase font-semibold',
                          st.confidence === 'high' ? 'bg-emerald-500/20 text-emerald-400' : (st.confidence === 'medium' ? 'bg-blue-500/20 text-blue-400' : 'bg-zinc-500/20 text-zinc-400')
                        )}>
                          {st.confidence}
                        </span>
                      </td>
                      <td className="p-2.5 text-center text-[var(--ui-text-muted)]">{st.failureCount}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
