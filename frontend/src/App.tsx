import { useState, useEffect, useCallback, useRef } from 'react';
import { UIX } from './components/icons';
import { PlatformTitlebar } from './components/shell/PlatformTitlebar';
import { AppNavigation, TabType } from './components/shell/AppNavigation';
import { MainControlView } from './components/views/MainControlView';
import { ProfilesView } from './components/views/ProfilesView';
import { BypassListsView } from './components/views/BypassListsView';
import { AppSettings, SettingsView } from './components/views/SettingsView';
import { LogJournalDrawer } from './components/terminal/LogJournalDrawer';
import { ModalHost } from './components/modals/ModalHost';

import { backendService } from './services/backend';
import { AutotuneProgressData, eventBus, windowService } from './services/window';
import { engine as WailsEngine, main as WailsMain } from '../wailsjs/go/models';

import { usePlatform } from './hooks/usePlatform';
import { useEngineController } from './hooks/useEngineController';
import { useEngineEvents } from './hooks/useEngineEvents';
import { usePingPolling } from './hooks/usePingPolling';
import { useLogJournal } from './hooks/useLogJournal';
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts';
import { useHostlists } from './hooks/useHostlists';
import { useLuaEditor } from './hooks/useLuaEditor';

const normalizeStartupProfileMode = (mode?: string) => {
  if (!mode || mode === 'Last Used' || mode === 'last_used') return 'Последний использованный';
  if (mode === 'AutoTune' || mode === 'autotune') return 'Автоподбор';
  return mode;
};

export default function App() {
  const searchParams = typeof window !== 'undefined' ? new URLSearchParams(window.location.search) : null;
  const paramTab = searchParams?.get('tab') as TabType | null;
  const paramTheme = searchParams?.get('theme');
  const paramStatus = searchParams?.get('status');

  const [activeTab, setActiveTab] = useState<TabType>(paramTab || 'main');
  const [theme, setThemeState] = useState<string>(() => paramTheme || localStorage.getItem('unbound-theme') || 'monolith');

  // Custom Domain Hooks
  const { platform, appVersion } = usePlatform();
  const engine = useEngineController(paramStatus === 'active' ? 'Running' : 'Stopped');
  const { state: engineState, actions: engineActions } = engine;

  const [managedVNextState, setManagedVNextState] = useState<WailsMain.AutoTuneVNextManagedStatus>(
    new WailsMain.AutoTuneVNextManagedStatus({ state: 'DIRECT', active: false, needs_revalidation: false })
  );
  const { livePingData, pingHistory } = usePingPolling(engineState.status, managedVNextState.active);
  const logJournal = useLogJournal(engineState.status, engineState.isScanning);
  const hostlistsEditor = useHostlists();
  const { state: hostlistState, actions: hostlistActions } = hostlistsEditor;

  // Toasts
  const [toasts, setToasts] = useState<Array<{ id: number; type: string; title: string; message: string }>>([]);
  const addToast = useCallback((toast: { id: number; type: string; title: string; message: string }) => {
    setToasts((prev) => {
      if (prev.some((t) => t.title === toast.title && t.message === toast.message)) return prev;
      return [...prev, toast];
    });
  }, []);
  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const luaEditor = useLuaEditor(engineState.selectedProfile, engineActions.setSelectedProfile, addToast);
  const { state: luaState, actions: luaActions } = luaEditor;

  // Security & Diagnostics Overlays
  const [privilegeError, setPrivilegeError] = useState<string>('');
  const [conflictWarning, setConflictWarning] = useState<string[]>([]);
  const [isDiagOpen, setIsDiagOpen] = useState<boolean>(false);
  const [diagResults, setDiagResults] = useState<WailsEngine.DiagnosticResult[]>([]);
  const [isDiagRunning] = useState<boolean>(false);
  const [doctorResult, setDoctorResult] = useState<WailsEngine.DoctorResult | null>(null);
  const [isDiscordCleanModalOpen, setIsDiscordCleanModalOpen] = useState(false);
  const [discordRunningProcs, setDiscordRunningProcs] = useState<string[]>([]);
  const [isStrategyLabOpen, setIsStrategyLabOpen] = useState(false);
  const [isAutoTuneVNextOpen, setIsAutoTuneVNextOpen] = useState(false);
  const [isAutoTuneVNextRunning, setIsAutoTuneVNextRunning] = useState(false);

  // Operations States
  const [isVerifyingAssets, setIsVerifyingAssets] = useState<boolean>(false);
  const [isGeneratingReport, setIsGeneratingReport] = useState<boolean>(false);
  const [isUpdatingHostlists, setIsUpdatingHostlists] = useState<boolean>(false);

  // Full Persisted Settings State (preserves all WailsEngine.Settings fields on save)
  const [settings, setSettings] = useState<AppSettings>({
    autoStart: false,
    startMinimized: false,
    autoStartProfile: false,
    defaultProfile: '',
    startupProfileMode: 'Последний использованный',
    gameFilter: true,
    autoUpdateEnabled: true,
    showLogs: true,
    enableTCPTimestamps: false,
    discordCacheAutoClean: false,
    secureDns: false,
    autoReconnect: true,
    favoriteProfiles: [],
    autoTuneTargets: [],
    diagnosticsMode: 'quick',
    autoUpdatePolicy: 'notify',
  });
  const initialSettingsLoaded = useRef(false);
  const rawSettingsRef = useRef<Record<string, unknown>>({});

  const updateSettings = useCallback((newSettingsOrUpdater: React.SetStateAction<AppSettings>) => {
    setSettings((prev) => {
      const next = typeof newSettingsOrUpdater === 'function' ? newSettingsOrUpdater(prev) : newSettingsOrUpdater;
      const mergedPayload: Record<string, unknown> = {
        ...rawSettingsRef.current,
        ...next,
      };
      rawSettingsRef.current = mergedPayload;
      if (initialSettingsLoaded.current) {
        backendService.saveSettings(new WailsEngine.Settings(mergedPayload)).catch((err) => {
          console.error('Failed to persist settings:', err);
        });
      }
      return next as AppSettings;
    });
  }, []);

  // Theme sync
  useEffect(() => {
    const applyTheme = () => {
      let activeTheme = theme;
      if (theme === 'system') {
        const prefersDark =
          typeof window.matchMedia === 'function'
            ? window.matchMedia('(prefers-color-scheme: dark)').matches
            : true;
        activeTheme = prefersDark ? 'monolith' : 'paper';
      }
      const classes = Array.from(document.body.classList).filter((c) => c.startsWith('theme-'));
      classes.forEach((c) => document.body.classList.remove(c));
      document.body.classList.add(`theme-${activeTheme}`);
    };

    applyTheme();
    localStorage.setItem('unbound-theme', theme);

    if (theme === 'system' && typeof window.matchMedia === 'function') {
      const media = window.matchMedia('(prefers-color-scheme: dark)');
      if (typeof media.addEventListener === 'function') {
        media.addEventListener('change', applyTheme);
        return () => media.removeEventListener('change', applyTheme);
      }
    }
    return undefined;
  }, [theme]);

  // System Checks & Settings load
  useEffect(() => {
    let active = true;
    backendService
      .checkPrivileges()
      .then((hasPriv) => {
        if (!active) return;
        if (!hasPriv) {
          const msg = platform === 'darwin' ? 'Требуются права root (sudo).' : 'Требуются права администратора.';
          setPrivilegeError(msg);
        } else {
          setPrivilegeError('');
        }
      })
      .catch(() => {});

    backendService
      .checkConflicts()
      .then((conflicts) => {
        if (active && conflicts && conflicts.length > 0) setConflictWarning(conflicts);
      })
      .catch(() => {});

    backendService
      .getSettings()
      .then((s: WailsEngine.Settings) => {
        if (!active) return;
        const raw = (s || {}) as unknown as Record<string, unknown>;
        rawSettingsRef.current = { ...raw };
        setSettings({
          autoStart: Boolean(s?.autoStart),
          startMinimized: Boolean(s?.startMinimized),
          autoStartProfile: Boolean(s?.autoStartProfile),
          defaultProfile: s?.defaultProfile ?? '',
          startupProfileMode: normalizeStartupProfileMode(s?.startupProfileMode),
          gameFilter: s?.gameFilter !== undefined ? Boolean(s.gameFilter) : (raw.enableGameFilter !== undefined ? Boolean(raw.enableGameFilter) : false),
          autoUpdateEnabled: s?.autoUpdateEnabled !== undefined ? Boolean(s.autoUpdateEnabled) : true,
          showLogs: s?.showLogs !== undefined ? Boolean(s.showLogs) : true,
          enableTCPTimestamps: Boolean(s?.enableTCPTimestamps),
          discordCacheAutoClean: Boolean(s?.discordCacheAutoClean),
          secureDns: Boolean(s?.secureDns),
          autoReconnect: s?.autoReconnect !== undefined ? Boolean(s.autoReconnect) : true,
          favoriteProfiles: Array.isArray(s?.favoriteProfiles) ? s.favoriteProfiles : [],
          autoTuneTargets: Array.isArray(s?.autoTuneTargets) ? s.autoTuneTargets : [],
          diagnosticsMode: s?.diagnosticsMode ?? 'quick',
          autoUpdatePolicy: s?.autoUpdatePolicy ?? 'notify',
        });
        initialSettingsLoaded.current = true;
      })
      .catch(() => {
        if (active) {
          initialSettingsLoaded.current = true;
        }
      });

    return () => {
      active = false;
    };
  }, [platform]);

  // Sync favoriteProfiles changes into rawSettingsRef so saving settings never clobbers favorites
  useEffect(() => {
    if (engineState.favoriteProfiles.length > 0) {
      rawSettingsRef.current = {
        ...rawSettingsRef.current,
        favoriteProfiles: engineState.favoriteProfiles,
      };
      setSettings((prev) =>
        prev.favoriteProfiles === engineState.favoriteProfiles
          ? prev
          : { ...prev, favoriteProfiles: engineState.favoriteProfiles }
      );
    }
  }, [engineState.favoriteProfiles]);

  useEffect(() => {
    let mounted = true;
    const refresh = () =>
      Promise.resolve()
        .then(() => backendService.getAutoTuneVNextManagedStatus())
        .then((status) => {
          if (mounted && status) setManagedVNextState(status);
        })
        .catch(() => {});
    refresh();
    const interval = window.setInterval(refresh, 2000);
    return () => {
      mounted = false;
      window.clearInterval(interval);
    };
  }, []);

  // Stable Event Callbacks
  const handleAutotuneProgress = useCallback(
    (data: AutotuneProgressData) => {
      engineActions.setAutotuneProgress(data);
      if (data?.msg) {
        engineActions.setScanProgress(data.msg);
        logJournal.addScanLog(data.msg);
      }
    },
    [engineActions, logJournal]
  );

  const handleAutotuneComplete = useCallback(
    (data: { success: boolean; profile?: string; error?: string }) => {
      engineActions.setIsScanning(false);
      if (data.success && data.profile) {
        engineActions.setSelectedProfile(data.profile);
        engineActions.setScanProgress(`✅ Готово! Профиль: ${data.profile}`);
      } else {
        engineActions.setScanProgress(`❌ ${data.error || 'Рабочий профиль не найден'}`);
      }
    },
    [engineActions]
  );

  const handleProfileChanged = useCallback(
    (profile: string) => {
      if (!profile) return;
      engineActions.setProfiles((prev) => (prev.includes(profile) ? prev : [...prev, profile]));
      engineActions.setSelectedProfile(profile);
    },
    [engineActions]
  );

  const handleProfileError = useCallback(
    (msg: string) => {
      const errorText = msg || 'Не удалось подобрать рабочий профиль при автопереключении.';
      logJournal.addEngineLog(`[ERROR] ${errorText}`);
      addToast({
        id: Date.now() + Math.random(),
        type: 'error',
        title: 'Сбой переключения профиля',
        message: errorText,
      });
    },
    [addToast, logJournal]
  );

  // Event Subscriptions (stabilized inside useEngineEvents)
  useEngineEvents({
    onStatusChange: engineActions.setStatus,
    onEnginesChange: engineActions.setEngines,
    onPrivilegeError: setPrivilegeError,
    onAddToast: addToast,
    onAutotuneStart: engineActions.setIsScanning,
    onAutotuneProgress: handleAutotuneProgress,
    onAutotuneLog: logJournal.addScanLog,
    onEngineLog: logJournal.addEngineLog,
    onAutotuneComplete: handleAutotuneComplete,
    onProfileChanged: handleProfileChanged,
    onProfileError: handleProfileError,
  });

  useEffect(
    () =>
      eventBus.onOpenAutoTuneVNext(() => {
        setActiveTab('main');
        setIsAutoTuneVNextOpen(true);
      }),
    []
  );

  // Shortcuts Hook (Escape + Ctrl+J / Ctrl+` for Log Journal)
  useKeyboardShortcuts(
    useCallback(() => {
      setIsDiagOpen(false);
      luaActions.setIsLuaOpen(false);
    }, [luaActions]),
    logJournal.toggleExpanded
  );

  // Operation Handlers
  const handleVerifyAssets = async () => {
    setIsVerifyingAssets(true);
    try {
      const res = await backendService.verifyEngineAssets();
      if (res.verified) {
        addToast({
          id: Date.now(),
          type: 'success',
          title: 'Безопасность подтверждена',
          message: `Проверено ${res.totalFiles} файлов.`,
        });
      } else {
        addToast({
          id: Date.now(),
          type: 'error',
          title: 'Ошибка целостности',
          message: res.error || 'Файлы повреждены.',
        });
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      addToast({
        id: Date.now(),
        type: 'error',
        title: 'Ошибка проверки',
        message: msg || 'Не удалось выполнить проверку.',
      });
    } finally {
      setIsVerifyingAssets(false);
    }
  };

  const handleDiagnosticReport = async () => {
    setIsGeneratingReport(true);
    try {
      const report = await backendService.generateDiagnosticReport();
      await backendService.exportLogs(report);
      addToast({ id: Date.now(), type: 'success', title: 'Диагностический отчёт', message: 'Отчёт сохранён в файл.' });
    } catch (err) {
      console.error(err);
    } finally {
      setIsGeneratingReport(false);
    }
  };

  const handleUpdateHostlists = async () => {
    setIsUpdatingHostlists(true);
    try {
      const result = await backendService.updateHostlistsNow();
      addToast({
        id: Date.now(),
        type: 'success',
        title: 'Списки обновлены',
        message: result || 'Списки обхода успешно обновлены.',
      });
    } catch {
      addToast({ id: Date.now(), type: 'error', title: 'Ошибка обновления', message: 'Не удалось обновить хостлисты.' });
    } finally {
      setIsUpdatingHostlists(false);
    }
  };

  // Opening Diagnostics launches ONE async StartDoctor('quick') inside DiagnosticsModal
  const handleOpenDiagnostics = useCallback(() => {
    setDoctorResult(null);
    setDiagResults([]);
    setIsDiagOpen(true);
  }, []);

  const handleClearCache = async () => {
    try {
      const runningCheck = await backendService.checkDiscordRunning();
      const isRunning = Array.isArray(runningCheck) ? Boolean(runningCheck[0]) : Boolean(runningCheck);
      const procs = Array.isArray(runningCheck) && Array.isArray(runningCheck[1]) ? (runningCheck[1] as string[]) : [];
      if (isRunning) {
        setDiscordRunningProcs(procs);
        setIsDiscordCleanModalOpen(true);
        return;
      }
      await executeDiscordClean(false);
    } catch (err) {
      console.error('Check discord running error:', err);
      await executeDiscordClean(false);
    }
  };

  const executeDiscordClean = async (closeIfRunning: boolean) => {
    setIsDiscordCleanModalOpen(false);
    try {
      const res = await backendService.clearDiscordCache(closeIfRunning);
      if (res && res.message) {
        const title =
          res.status === 'SUCCESS'
            ? 'Кэш очищен'
            : res.status === 'PARTIAL'
            ? 'Очищено частично'
            : 'Кэш Discord';
        windowService.showNotification(title, res.message);
      }
    } catch (err) {
      console.error('Clear discord cache error:', err);
      windowService.showNotification('Ошибка очистки', 'Не удалось очистить кэш Discord: ' + String(err));
    }
  };

  const handleKillWinws2 = async () => {
    try {
      await backendService.killWinws2();
      engineActions.setStatus('Stopped');
      windowService.showNotification('Успех', 'Движок обхода остановлен.');
    } catch {
      windowService.showNotification('Ошибка', 'Не удалось остановить движок.');
    }
  };

  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map((toast) => setTimeout(() => removeToast(toast.id), 5000));
    return () => timers.forEach((t) => clearTimeout(t));
  }, [toasts, removeToast]);

  const isManagedVNextActive = managedVNextState.active;
  const isConnected = engineState.status === 'Running' || isManagedVNextActive;
  const isConnecting = engineState.status === 'Starting';
  const disableMain = isConnecting || engineState.isScanning || isAutoTuneVNextRunning;

  const statusLedState =
    livePingData.status === 'error'
      ? 'error'
      : isConnecting || engineState.isScanning
      ? 'connecting'
      : isConnected
      ? 'connected'
      : 'disconnected';

  return (
    <div
      className="flex flex-col h-screen w-screen relative app-drag select-none"
      style={{ background: 'var(--ui-panel)', color: 'var(--ui-text)' }}
    >
      {/* TOAST NOTIFICATIONS */}
      <div className="fixed top-4 right-4 z-[10000] flex flex-col gap-2 pointer-events-none app-no-drag">
        {toasts.map((toast) => (
          <div
            key={toast.id}
            className="pointer-events-auto p-3.5 min-w-[260px] max-w-[320px] rounded-xl border bg-[var(--ui-panel)] text-[var(--ui-text)] border-[var(--ui-border-strong)] shadow-lg"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1">
                <div className="font-semibold text-sm mb-0.5">{toast.title}</div>
                <div className="text-xs text-[var(--ui-text-muted)] leading-snug">{toast.message}</div>
              </div>
              <button
                onClick={() => removeToast(toast.id)}
                className="text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] text-sm"
              >
                <UIX className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* PLATFORM AWARE TITLEBAR */}
      <PlatformTitlebar platform={platform} appVersion={appVersion} privilegeError={privilegeError} />

      {/* MAIN CONTAINER */}
      <div className="flex-1 flex flex-col min-h-0 app-no-drag overflow-y-auto p-4 space-y-4">
        {/* SEGMENTED NAVIGATION */}
        <AppNavigation
          activeTab={activeTab}
          onTabChange={(tab) => {
            setActiveTab(tab);
            if (tab === 'lists') {
              hostlistActions.handleOpenHostlistEditor();
            }
          }}
        />

        {/* VIEWS */}
        {activeTab === 'main' && (
          <MainControlView
            statusLedState={statusLedState}
            isConnected={isConnected}
            isConnecting={isConnecting}
            disableMain={disableMain}
            toggleConnection={engineActions.toggleConnection}
            selectedProfile={engineState.selectedProfile}
            setSelectedProfile={engineActions.setSelectedProfile}
            sortedProfiles={engineState.sortedProfiles}
            selectedEngine={engineState.selectedEngine}
            handleToggleFavorite={engineActions.handleToggleFavorite}
            favoriteProfiles={engineState.favoriteProfiles}
            handleAutoTune={engineActions.handleAutoTune}
            openAutoTuneVNext={() => setIsAutoTuneVNextOpen(true)}
            vNextRunning={isAutoTuneVNextRunning}
            isScanning={engineState.isScanning}
            scanProgress={engineState.scanProgress}
            autotuneProgress={engineState.autotuneProgress}
            handleCancelAutoTune={engineActions.cancelAutoTune}
            livePingData={livePingData}
            pingHistory={pingHistory}
            managedState={managedVNextState}
            revertManaged={() => backendService.revertAutoTuneVNext().then(setManagedVNextState)}
          />
        )}

        {activeTab === 'profiles' && (
          <ProfilesView
            sortedProfiles={engineState.sortedProfiles}
            selectedProfile={engineState.selectedProfile}
            setSelectedProfile={engineActions.setSelectedProfile}
            favoriteProfiles={engineState.favoriteProfiles}
            isConnected={isConnected}
            openLuaEditor={luaActions.openLuaEditor}
            openStrategyLab={() => setIsStrategyLabOpen(true)}
          />
        )}

        {activeTab === 'lists' && (
          <BypassListsView
            selectedList={hostlistState.selectedList}
            hostlists={hostlistState.hostlists}
            handleSelectHostlist={hostlistActions.handleSelectHostlist}
            hostlistContent={hostlistState.hostlistContent}
            setHostlistContent={hostlistActions.setHostlistContent}
            handleSaveHostlist={hostlistActions.handleSaveHostlist}
            isSavingHostlist={hostlistState.isSavingHostlist}
          />
        )}

        {activeTab === 'settings' && (
          <SettingsView
            settings={settings}
            setSettings={updateSettings}
            theme={theme}
            setThemeState={setThemeState}
            startupProfiles={engineState.sortedProfiles}
            handleRunDiagnostics={handleOpenDiagnostics}
            handleVerifyAssets={handleVerifyAssets}
            isVerifyingAssets={isVerifyingAssets}
            handleDiagnosticReport={handleDiagnosticReport}
            isGeneratingReport={isGeneratingReport}
            handleUpdateHostlists={handleUpdateHostlists}
            isUpdatingHostlists={isUpdatingHostlists}
            handleClearCache={handleClearCache}
            handleKillWinws2={handleKillWinws2}
          />
        )}
      </div>

      {/* LOG JOURNAL DRAWER */}
      {settings.showLogs && (
        <LogJournalDrawer
          logs={logJournal.displayLogs}
          isExpanded={logJournal.isExpanded}
          onToggle={logJournal.toggleExpanded}
          onExportLogs={logJournal.exportLogs}
        />
      )}

      {/* MODALS HOST */}
      <ModalHost
        conflictWarning={conflictWarning}
        onIgnoreConflicts={() => setConflictWarning([])}
        onKillConflicts={async () => {
          await backendService.killConflicts();
          setConflictWarning([]);
        }}
        privilegeError={privilegeError}
        platform={platform}
        onClosePrivilegeModal={() => setPrivilegeError('')}
        isDiagOpen={isDiagOpen}
        isDiagRunning={isDiagRunning}
        diagResults={diagResults}
        onCloseDiagnosticsModal={() => setIsDiagOpen(false)}
        doctorResult={doctorResult}
        isDiscordCleanModalOpen={isDiscordCleanModalOpen}
        discordRunningProcs={discordRunningProcs}
        onCancelDiscordClean={() => setIsDiscordCleanModalOpen(false)}
        onConfirmDiscordClean={() => executeDiscordClean(true)}
        isStrategyLabOpen={isStrategyLabOpen}
        onCloseStrategyLab={() => setIsStrategyLabOpen(false)}
        isAutoTuneVNextOpen={isAutoTuneVNextOpen}
        onCloseAutoTuneVNext={() => setIsAutoTuneVNextOpen(false)}
        onAutoTuneVNextRunningChange={setIsAutoTuneVNextRunning}
        onSaveDiscoveredProfileSuccess={(name) => {
          engineActions.setSelectedProfile(name);
          addToast({
            id: Date.now(),
            type: 'success',
            title: 'Профиль сохранён',
            message: `Стратегия сохранена как «${name}»`,
          });
        }}
        isLuaOpen={luaState.isLuaOpen}
        onCloseLuaModal={() => luaActions.setIsLuaOpen(false)}
        luaTab={luaState.luaTab}
        setLuaTab={luaActions.setLuaTab}
        luaIsAuto={luaState.luaIsAuto}
        setLuaIsAuto={luaActions.setLuaIsAuto}
        luaFakeBlob={luaState.luaFakeBlob}
        setLuaFakeBlob={luaActions.setLuaFakeBlob}
        luaPos={luaState.luaPos}
        setLuaPos={luaActions.setLuaPos}
        luaFool={luaState.luaFool}
        setLuaFool={luaActions.setLuaFool}
        luaTtl={luaState.luaTtl}
        setLuaTtl={luaActions.setLuaTtl}
        luaCode={luaState.luaCode}
        setLuaCode={luaActions.setLuaCode}
        onSaveLua={luaActions.saveLuaStrategy}
      />
    </div>
  );
}
