import { useState, useEffect, useCallback } from 'react';

import { PlatformTitlebar } from './components/shell/PlatformTitlebar';
import { AppNavigation, TabType } from './components/shell/AppNavigation';
import { MainControlView } from './components/views/MainControlView';
import { ProfilesView } from './components/views/ProfilesView';
import { BypassListsView } from './components/views/BypassListsView';
import { SettingsView } from './components/views/SettingsView';
import { LogJournalDrawer } from './components/terminal/LogJournalDrawer';
import { ModalHost } from './components/modals/ModalHost';

import { backendService } from './services/backend';
import { windowService } from './services/window';
import { engine } from '../wailsjs/go/models';

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

  const { livePingData, pingHistory } = usePingPolling(engineState.status);
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
  const [diagResults, setDiagResults] = useState<engine.DiagnosticResult[]>([]);
  const [isDiagRunning, setIsDiagRunning] = useState<boolean>(false);

  // Operations States
  const [isVerifyingAssets, setIsVerifyingAssets] = useState<boolean>(false);
  const [isGeneratingReport, setIsGeneratingReport] = useState<boolean>(false);
  const [isUpdatingHostlists, setIsUpdatingHostlists] = useState<boolean>(false);

  // Settings State
  const [settings, setSettings] = useState<{
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
  }>({
    autoStart: false,
    startMinimized: false,
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
  });

  // Theme sync
  useEffect(() => {
    let activeTheme = theme;
    if (theme === 'system') {
      const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      activeTheme = isDark ? 'monolith' : 'paper';
    }
    const classes = Array.from(document.body.classList).filter((c) => c.startsWith('theme-'));
    classes.forEach((c) => document.body.classList.remove(c));
    document.body.classList.add(`theme-${activeTheme}`);
    localStorage.setItem('unbound-theme', theme);
  }, [theme]);

  // System Checks & Settings load
  useEffect(() => {
    backendService.checkPrivileges().then((hasPriv) => {
      if (!hasPriv) {
        const msg = platform === 'darwin' ? 'Требуются права root (sudo).' : 'Требуются права администратора.';
        setPrivilegeError(msg);
      }
    }).catch(() => {});

    backendService.checkConflicts().then((conflicts) => {
      if (conflicts && conflicts.length > 0) setConflictWarning(conflicts);
    }).catch(() => {});

    backendService.getSettings().then((s: engine.Settings) => {
      setSettings({
        autoStart: s.autoStart || false,
        startMinimized: s.startMinimized || false,
        defaultProfile: s.defaultProfile || '',
        startupProfileMode: normalizeStartupProfileMode(s.startupProfileMode),
        gameFilter: s.gameFilter !== undefined ? s.gameFilter : false,
        autoUpdateEnabled: s.autoUpdateEnabled !== undefined ? s.autoUpdateEnabled : true,
        showLogs: s.showLogs !== undefined ? s.showLogs : true,
        enableTCPTimestamps: s.enableTCPTimestamps || false,
        discordCacheAutoClean: s.discordCacheAutoClean || false,
        secureDns: s.secureDns || false,
        autoReconnect: s.autoReconnect !== undefined ? s.autoReconnect : true,
        favoriteProfiles: s.favoriteProfiles || [],
      });
    }).catch(() => {});
  }, [platform]);

  // Event Subscriptions
  useEngineEvents({
    onStatusChange: engineActions.setStatus,
    onEnginesChange: engineActions.setEngines,
    onPrivilegeError: setPrivilegeError,
    onAddToast: addToast,
    onAutotuneStart: engineActions.setIsScanning,
    onAutotuneProgress: (data) => {
      engineActions.setAutotuneProgress(data);
      if (data?.msg) {
        engineActions.setScanProgress(data.msg);
        logJournal.addScanLog(data.msg);
      }
    },
    onAutotuneLog: logJournal.addScanLog,
    onEngineLog: logJournal.addEngineLog,
    onAutotuneComplete: (data) => {
      engineActions.setIsScanning(false);
      if (data.success && data.profile) {
        engineActions.setSelectedProfile(data.profile);
        engineActions.setScanProgress(`✅ Готово! Профиль: ${data.profile}`);
      } else {
        engineActions.setScanProgress(`❌ ${data.error || 'Рабочий профиль не найден'}`);
      }
    },
  });

  // Shortcuts Hook
  useKeyboardShortcuts(
    useCallback(() => {
      setIsDiagOpen(false);
      luaActions.setIsLuaOpen(false);
    }, [luaActions])
  );

  // Operation Handlers
  const handleVerifyAssets = async () => {
    setIsVerifyingAssets(true);
    try {
      const res = await backendService.verifyEngineAssets();
      if (res.verified) {
        addToast({ id: Date.now(), type: 'success', title: 'Безопасность подтверждена', message: `Проверено ${res.totalFiles} файлов.` });
      } else {
        addToast({ id: Date.now(), type: 'error', title: 'Ошибка целостности', message: res.error || 'Файлы повреждены.' });
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      addToast({ id: Date.now(), type: 'error', title: 'Ошибка проверки', message: msg || 'Не удалось выполнить проверку.' });
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
      addToast({ id: Date.now(), type: 'success', title: 'Списки обновлены', message: result || 'Списки обхода успешно обновлены.' });
    } catch (err) {
      addToast({ id: Date.now(), type: 'error', title: 'Ошибка обновления', message: 'Не удалось обновить хостлисты.' });
    } finally {
      setIsUpdatingHostlists(false);
    }
  };

  const handleRunDiagnostics = async () => {
    setIsDiagRunning(true);
    setIsDiagOpen(true);
    try {
      const results = await backendService.runDiagnostics();
      setDiagResults(Array.isArray(results) ? results : []);
    } catch (err) {
      console.error(err);
    } finally {
      setIsDiagRunning(false);
    }
  };

  const handleClearCache = async () => {
    try {
      await backendService.clearDiscordCache();
      windowService.showNotification('Кэш очищен', 'Кэш Discord успешно очищен.');
    } catch (err) {
      console.error(err);
    }
  };

  const handleKillWinws2 = async () => {
    try {
      await backendService.killWinws2();
      engineActions.setStatus('Stopped');
      windowService.showNotification('Успех', 'Движок обхода остановлен.');
    } catch (err) {
      windowService.showNotification('Ошибка', 'Не удалось остановить движок.');
    }
  };

  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map((toast) => setTimeout(() => removeToast(toast.id), 5000));
    return () => timers.forEach((t) => clearTimeout(t));
  }, [toasts, removeToast]);

  const isConnected = engineState.status === 'Running';
  const isConnecting = engineState.status === 'Starting';
  const disableMain = isConnecting || engineState.isScanning;

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
              <button onClick={() => removeToast(toast.id)} className="text-[var(--ui-text-muted)] hover:text-[var(--ui-text)] text-sm">
                ✕
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* PLATFORM AWARE TITLEBAR */}
      <PlatformTitlebar platform={platform} appVersion={appVersion} />

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
            isScanning={engineState.isScanning}
            scanProgress={engineState.scanProgress}
            autotuneProgress={engineState.autotuneProgress}
            handleCancelAutoTune={engineActions.cancelAutoTune}
            livePingData={livePingData}
            pingHistory={pingHistory}
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
            setSettings={setSettings}
            theme={theme}
            setThemeState={setThemeState}
            handleRunDiagnostics={handleRunDiagnostics}
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
          onToggle={() => logJournal.setIsExpanded(!logJournal.isExpanded)}
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
