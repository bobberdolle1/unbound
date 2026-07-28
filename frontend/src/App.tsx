import { useState, useEffect, useRef } from 'react';

import { cn } from './lib/cn';
import { formatLog } from './lib/format';
import { generateLuaCode, parseLuaCode } from './lib/lua';
import { SketchySpinner, SketchyX, SketchyStar, SketchyGear, SketchyTerminal, SketchyCheck } from './components/icons';
import { DoodleSelect } from './components/DoodleSelect';
import { DoodleCheckbox } from './components/DoodleCheckbox';
import { PingChart } from './components/PingChart';

import { GetEngineNames, GetProfiles, StartEngine, StopEngine, GetLogs, AutoTune, CancelAutoTune, GetSettings, SaveSettings, GetLivePing, ShowNotification, EnableAutoStart, DisableAutoStart, IsAutoStartEnabled, CheckConflicts, KillConflicts, CheckPrivileges, RunDiagnostics, ClearDiscordCache, KillWinws2, QuitApp, GetAppVersion, GetOSPlatform, HideWindowToTray, GetBypassLists, ReadBypassList, SaveBypassList, ExportLogs, SaveCustomScript, LoadCustomScript, VerifyEngineAssets, GenerateDiagnosticReport, ToggleFavoriteProfile, GetFavoriteProfiles, UpdateHostlistsNow, AutoReconnectMonitor, StopAutoReconnect, SavePingHistory, LoadPingHistory } from '../wailsjs/go/main/App';
import { EventsOn, WindowMinimise } from '../wailsjs/runtime/runtime';

export default function App() {
  const [engines, setEngines] = useState<string[]>([]);
  const [selectedEngine, setSelectedEngine] = useState<string>('');
  const [profiles, setProfiles] = useState<string[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<string>('');
  const [status, setStatus] = useState<string>('Stopped');
  const [logs, setLogs] = useState<string[]>([]);
  const [scanLogs, setScanLogs] = useState<string[]>([]);
  const [isLogExpanded, setIsLogExpanded] = useState<boolean>(false);
  const [isScanning, setIsScanning] = useState<boolean>(false);
  const [scanProgress, setScanProgress] = useState<string>('');
  const [scanSuccess, setScanSuccess] = useState<boolean | null>(null);
  const [isSettingsOpen, setIsSettingsOpen] = useState<boolean>(false);
  const [isDiagOpen, setIsDiagOpen] = useState<boolean>(false);
  const [diagResults, setDiagResults] = useState<any[]>([]);
  const [isDiagRunning, setIsDiagRunning] = useState<boolean>(false);
  
  const [theme, setThemeState] = useState<string>(() => localStorage.getItem('unbound-theme') || 'doodle');
  const [settings, setSettings] = useState<{
    autoStart: boolean, 
    startMinimized: boolean, 
    defaultProfile: string, 
    startupProfileMode: string, 
    gameFilter: boolean, 
    autoUpdateEnabled: boolean, 
    showLogs: boolean,
    enableTCPTimestamps: boolean,
    discordCacheAutoClean: boolean,
    secureDns: boolean,
    autoReconnect: boolean,
    favoriteProfiles: string[]
  }>({
    autoStart: false,
    startMinimized: false,
    defaultProfile: 'Unbound Ultimate (God Mode)',
    startupProfileMode: 'Последний использованный',
    gameFilter: true,
    autoUpdateEnabled: true,
    showLogs: true,
    enableTCPTimestamps: false,
    discordCacheAutoClean: false,
    secureDns: false,
    autoReconnect: true,
    favoriteProfiles: []
  });
  const [livePingData, setLivePingData] = useState<{active: boolean, latency: number, status: string, services?: Record<string, number>}>({active: false, latency: 0, status: 'stopped'});
  const [pingHistory, setPingHistory] = useState<number[]>([]);
  const [privilegeError, setPrivilegeError] = useState<string>('');
  const [conflictWarning, setConflictWarning] = useState<string[]>([]);
  const [isUserScrolling, setIsUserScrolling] = useState<boolean>(false);
  const [toasts, setToasts] = useState<Array<{id: number, type: string, title: string, message: string}>>([]);
  const [isLuaOpen, setIsLuaOpen] = useState<boolean>(false);
  const [luaTab, setLuaTab] = useState<'builder' | 'code'>('builder');
  const [luaIsAuto, setLuaIsAuto] = useState<boolean>(true);
  const [luaFakeBlob, setLuaFakeBlob] = useState<string>('fake_default_tls');
  const [luaPos, setLuaPos] = useState<string>('1');
  const [luaFool, setLuaFool] = useState<string>('none');
  const [luaTtl, setLuaTtl] = useState<number>(0);
  const [luaCode, setLuaCode] = useState<string>('');
  const [appVersion, setAppVersion] = useState<string>('');
  const [platform, setPlatform] = useState<string>('');
  const [isVerifyingAssets, setIsVerifyingAssets] = useState<boolean>(false);
  const [autotuneProgress, setAutotuneProgress] = useState<{
    step: number;
    total: number;
    percent: number;
    profile: string;
    okCount: number;
    totalTargets: number;
    msg: string;
  } | null>(null);
  const [favoriteProfiles, setFavoriteProfiles] = useState<string[]>([]);
  const [isGeneratingReport, setIsGeneratingReport] = useState<boolean>(false);
  const [isUpdatingHostlists, setIsUpdatingHostlists] = useState<boolean>(false);
  const handleVerifyAssets = async () => {
    setIsVerifyingAssets(true);
    try {
      const res = await VerifyEngineAssets();
      const id = Date.now();
      if (res.verified) {
        setToasts(prev => [...prev, {
          id,
          type: 'success',
          title: 'Безопасность подтверждена',
          message: `Проверено ${res.totalFiles} файлов. Все SHA256 хеши совпадают!`
        }]);
      } else {
        setToasts(prev => [...prev, {
          id,
          type: 'error',
          title: 'Ошибка целостности',
          message: res.error || 'Некоторые файлы повреждены.'
        }]);
      }
    } catch (err: any) {
      const id = Date.now();
      setToasts(prev => [...prev, {
        id,
        type: 'error',
        title: 'Ошибка проверки',
        message: err?.message || 'Не удалось выполнить проверку.'
      }]);
    } finally {
      setIsVerifyingAssets(false);
    }
  };

  const handleToggleFavorite = async () => {
    try {
      const favs = await ToggleFavoriteProfile(selectedProfile);
      setFavoriteProfiles(favs || []);
    } catch (err) {
      console.error('ToggleFavorite failed:', err);
    }
  };

  const handleDiagnosticReport = async () => {
    setIsGeneratingReport(true);
    try {
      const report = await GenerateDiagnosticReport();
      await ExportLogs(report);
      setToasts(prev => [...prev, {
        id: Date.now(),
        type: 'success',
        title: 'Диагностический отчёт',
        message: 'Отчёт сохранён в файл'
      }]);
    } catch (err) {
      console.error('DiagnosticReport failed:', err);
    } finally {
      setIsGeneratingReport(false);
    }
  };

  const handleUpdateHostlists = async () => {
    setIsUpdatingHostlists(true);
    try {
      const result = await UpdateHostlistsNow();
      setToasts(prev => [...prev, {
        id: Date.now(),
        type: 'success',
        title: 'Хостлисты обновлены',
        message: result || 'Списки доменов успешно обновлены'
      }]);
    } catch (err) {
      setToasts(prev => [...prev, {
        id: Date.now(),
        type: 'error',
        title: 'Ошибка обновления',
        message: 'Не удалось обновить хостлисты'
      }]);
    } finally {
      setIsUpdatingHostlists(false);
    }
  };

  const sortedProfiles = [...profiles].sort((a, b) => {
    const aFav = favoriteProfiles.includes(a) ? -1 : 0;
    const bFav = favoriteProfiles.includes(b) ? -1 : 0;
    return aFav - bFav;
  });

  const openLuaEditor = async () => {
    try {
      const code = await LoadCustomScript();
      setLuaCode(code);
      const parsed = parseLuaCode(code);
      setLuaIsAuto(parsed.isAuto);
      if (parsed.isAuto) {
        setLuaFakeBlob(parsed.fakeBlob);
        setLuaPos(parsed.pos);
        setLuaFool(parsed.fool);
        setLuaTtl(parsed.ttl);
        setLuaTab('builder');
      } else {
        setLuaTab('code');
      }
      setIsLuaOpen(true);
    } catch (err) {
      console.error("Failed to load custom LUA script:", err);
      const id = Date.now();
      setToasts(prev => [...prev, { id, type: 'error', title: 'Ошибка', message: 'Не удалось загрузить LUA-стратегию.' }]);
    }
  };

  const saveLuaStrategy = async () => {
    try {
      const finalCode = luaIsAuto 
        ? generateLuaCode({ fakeBlob: luaFakeBlob, pos: luaPos, fool: luaFool, ttl: luaTtl })
        : luaCode;
      
      await SaveCustomScript(finalCode);
      setIsLuaOpen(false);
      
      const id = Date.now();
      setToasts(prev => [...prev, { id, type: 'success', title: 'Успех', message: 'Стратегия LUA успешно сохранена.' }]);
      
      if (selectedProfile !== 'Custom Profile') {
        setSelectedProfile('Custom Profile');
      }
    } catch (err) {
      console.error("Failed to save custom LUA script:", err);
      const id = Date.now();
      setToasts(prev => [...prev, { id, type: 'error', title: 'Ошибка', message: 'Не удалось сохранить стратегию.' }]);
    }
  };

  const logsEndRef = useRef<HTMLDivElement>(null);
  const logsContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // GetAppVersion was bound and imported but never called, so the UI showed
    // no version anywhere. Fetch it once on mount.
    GetAppVersion().then(setAppVersion).catch(() => setAppVersion(''));
    GetOSPlatform().then((plat: string) => {
      setPlatform(plat || '');
      if (plat === 'darwin' && !localStorage.getItem('unbound-theme')) {
        setThemeState('liquid-glass');
      }
    }).catch(() => {});
  }, []);

  useEffect(() => {
    let activeTheme = theme;
    if (theme === 'system') {
      const isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      activeTheme = isDark ? 'dark' : 'light';
    }
    const classes = Array.from(document.body.classList).filter(c => c.startsWith('theme-'));
    classes.forEach(c => document.body.classList.remove(c));
    document.body.classList.add(`theme-${activeTheme}`);
    localStorage.setItem('unbound-theme', theme);
  }, [theme]);
  useEffect(() => {
    EventsOn('status_changed', (newStatus: string) => setStatus(newStatus));

    const interval = setInterval(() => {
      if (!isScanning && status === 'Running') {
        GetLogs().then((l: string[]) => setLogs(l || []));
      }
    }, 2000);

    const pingInterval = setInterval(() => {
      if (status === 'Running') {
        GetLivePing().then((data: any) => {
          const lat = data?.latency || 0;
          const stat = data?.status || 'stopped';
          const serv = data?.services || {};
          setLivePingData({ active: data?.active || false, latency: lat, status: stat, services: serv });
          if (stat === 'ok') {
            setPingHistory(prev => [...prev.slice(-14), lat]);
          }
          // Persist ping history
          SavePingHistory(lat, stat).catch(() => {});
        }).catch(() => setLivePingData({active: false, latency: 0, status: 'error'}));
      } else {
        setLivePingData({active: false, latency: 0, status: 'stopped'});
        setPingHistory([]);
      }
    }, 5000);
    return () => {
      clearInterval(interval);
      clearInterval(pingInterval);
    };
  }, [isScanning, status]);

  useEffect(() => {
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission();
    }
    
    const checkAdmin = async () => {
      try {
        const hasPriv = await CheckPrivileges();
        if (!hasPriv) {
          const plat = platform || await GetOSPlatform().catch(() => '');
          const msg = plat === 'darwin' 
            ? 'Требуются права root (sudo). Запустите приложение с правами root для работы с pf.' 
            : 'Требуются права администратора. Перезапустите приложение от имени администратора.';
          setPrivilegeError(msg);
        }
      } catch (err) {
        const msg = platform === 'darwin' 
          ? 'Требуются права root (sudo). Запустите приложение с правами root для работы с pf.' 
          : 'Требуются права администратора. Перезапустите приложение от имени администратора.';
        setPrivilegeError(msg);
      }
    };
    checkAdmin();
    
    const checkConflicts = async () => {
      try {
        const conflicts = await CheckConflicts();
        if (conflicts && conflicts.length > 0) setConflictWarning(conflicts);
      } catch (err) {}
    };
    checkConflicts();
    
    GetEngineNames().then((e: string[]) => {
      setEngines(e || []);
      if (e && e.length > 0) setSelectedEngine(e[0]);
    });
    
    GetSettings().then((s: any) => {
      setSettings({
        autoStart: s.autoStart || false,
        startMinimized: s.startMinimized || false,
        defaultProfile: s.defaultProfile || 'Unbound Ultimate (God Mode)',
        startupProfileMode: s.startupProfileMode || 'Last Used',
        gameFilter: s.gameFilter !== undefined ? s.gameFilter : false,
        autoUpdateEnabled: s.autoUpdateEnabled !== undefined ? s.autoUpdateEnabled : true,
        showLogs: s.showLogs !== undefined ? s.showLogs : true,
        enableTCPTimestamps: s.enableTCPTimestamps || false,
        discordCacheAutoClean: s.discordCacheAutoClean || false,
        secureDns: s.secureDns || false,
        autoReconnect: s.autoReconnect !== undefined ? s.autoReconnect : true,
        favoriteProfiles: s.favoriteProfiles || []
      });
    });
    GetFavoriteProfiles().then((favs: string[]) => {
      setFavoriteProfiles(favs || []);
    });
    LoadPingHistory().then((records: any[]) => {
      if (records && records.length > 0) {
        const recent = records.slice(-15).map((r: any) => r.lat || 0).filter((l: number) => l > 0);
        if (recent.length > 0) setPingHistory(recent);
      }
    }).catch(() => {});
  }, []);
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsSettingsOpen(false);
        setIsDiagOpen(false);
        setIsLuaOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  useEffect(() => {
    if (selectedEngine) {
      GetProfiles(selectedEngine).then((p: string[]) => {
        setProfiles(p || []);
        if (p && p.length > 0) {
          if (!selectedProfile || !p.includes(selectedProfile)) {
            setSelectedProfile(p[0]);
          }
        } else if (!p || p.length === 0) {
          console.error('No profiles loaded from backend. Check engine registration.');
        }
      });
    }
  }, [selectedEngine]);

  useEffect(() => {
    EventsOn('privilege_error', (msg: string) => {
      setPrivilegeError(msg);
    });
    EventsOn('engine_error', (msg: string) => {
      ShowNotification("Ошибка движка", msg);
      setStatus("Stopped");
    });
    EventsOn('notification', (data: any) => {
      setToasts(prev => {
        // Prevent exact duplicates within short timeframes if wails glitch occurs
        if (prev.some(t => t.title === data.title && t.message === data.message)) return prev;
        return [...prev, {
          id: Date.now() + Math.random(),
          type: data.type || 'info',
          title: data.title,
          message: data.message
        }];
      });
      setScanLogs(prev => [...prev, `🔔 ${data.title}: ${data.message}`]);
    });
    EventsOn('autotune_start', (running: boolean) => {
      setIsScanning(running);
    });
    EventsOn('autotune_progress', (data: any) => {
      setAutotuneProgress(data);
      if (data?.msg) {
        setScanProgress(data.msg);
        setScanLogs(prev => [...prev, data.msg]);
      }
    });
    EventsOn('autotune_log', (msg: string) => {
      setScanLogs(prev => [...prev, msg]);
    });
    EventsOn('engine_log', (msg: string) => {
      setLogs(prev => [...prev, msg]);
    });
    EventsOn('autotune_complete', (data: {success: boolean, profile?: string, error?: string}) => {
      setScanSuccess(data.success);
      setIsScanning(false);
      if (data.success && data.profile) {
        setSelectedProfile(data.profile);
        setScanProgress(`✅ Готово! Профиль: ${data.profile}`);
      } else {
        setScanProgress(`❌ ${data.error || 'Рабочий профиль не найден'}`);
      }
      setTimeout(() => {
        setScanSuccess(null);
        setScanProgress('');
        setAutotuneProgress(null);
      }, 10000);
    });
    EventsOn('profile_changed', (profile: string) => {
      setSelectedProfile(profile);
      setScanLogs(prev => [...prev, `🔄 Авто-переключение на профиль: ${profile}`]);
    });
  }, []);

  useEffect(() => {
    if (isLogExpanded && settings.showLogs && !isUserScrolling) {
      logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, scanLogs, isLogExpanded, isScanning, settings.showLogs, isUserScrolling]);

  useEffect(() => {
    const container = logsContainerRef.current;
    if (!container) return;

    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = container;
      const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
      setIsUserScrolling(!isAtBottom);
    };

    container.addEventListener('scroll', handleScroll);
    return () => container.removeEventListener('scroll', handleScroll);
  }, [isLogExpanded]);

  const toggleConnection = async () => {
    try {
      if (status === 'Running' || status === 'Starting') {
        await StopEngine();
      } else {
        let eToStart = selectedEngine;
        if (!eToStart && engines.length > 0) {
          eToStart = engines[0];
          setSelectedEngine(eToStart);
        }
        let pToStart = selectedProfile;
        if (!pToStart && profiles.length > 0) {
          pToStart = profiles[0];
          setSelectedProfile(pToStart);
        }
        await StartEngine(eToStart || "", pToStart || "");
      }
    } catch (err) {
      console.error('[ERROR] toggleConnection failed:', err);
      setLogs(prev => [...prev, `Ошибка: ${err}`]);
    }
  };

  const handleAutoTune = () => {
    setScanLogs([]);
    if (settings.showLogs) setIsLogExpanded(true);
    AutoTune(); // fire-and-forget; events handle all state
  };

  const handleOpenSettings = async () => {
    setIsSettingsOpen(true);
    try {
      const loadedSettings = await GetSettings();
      const autoStartEnabled = await IsAutoStartEnabled();
      setSettings({
        autoStart: autoStartEnabled,
        startMinimized: loadedSettings.startMinimized || false,
        defaultProfile: loadedSettings.defaultProfile || 'Unbound Ultimate (God Mode)',
        startupProfileMode: loadedSettings.startupProfileMode || 'Последний использованный',
        gameFilter: loadedSettings.gameFilter !== undefined ? loadedSettings.gameFilter : false,
        autoUpdateEnabled: loadedSettings.autoUpdateEnabled !== undefined ? loadedSettings.autoUpdateEnabled : true,
        showLogs: loadedSettings.showLogs !== undefined ? loadedSettings.showLogs : true,
        enableTCPTimestamps: loadedSettings.enableTCPTimestamps || false,
        discordCacheAutoClean: loadedSettings.discordCacheAutoClean || false,
        secureDns: loadedSettings.secureDns || false,
        autoReconnect: loadedSettings.autoReconnect !== undefined ? loadedSettings.autoReconnect : true,
        favoriteProfiles: loadedSettings.favoriteProfiles || []
      });
    } catch (err) {
      console.error(err);
    }
  };

  const handleSaveSettings = async () => {
    try {
      if (settings.autoStart) {
        await EnableAutoStart();
      } else {
        await DisableAutoStart();
      }
      await SaveSettings(settings);
      setIsSettingsOpen(false);
    } catch (err) {
      console.error(err);
    }
  };

  // Hostlist Editor States & Handlers
  const [isHostlistOpen, setIsHostlistOpen] = useState<boolean>(false);
  const [hostlists, setHostlists] = useState<string[]>([]);
  const [selectedList, setSelectedList] = useState<string>('');
  const [hostlistContent, setHostlistContent] = useState<string>('');
  const [isSavingHostlist, setIsSavingHostlist] = useState<boolean>(false);

  const addHostlistPreset = (presetDomains: string[]) => {
    const existingLines = new Set(hostlistContent.split('\n').map(l => l.trim().toLowerCase()));
    const toAdd = presetDomains.filter(d => !existingLines.has(d.toLowerCase()));
    if (toAdd.length > 0) {
      const newContent = hostlistContent.trim()
        ? hostlistContent.trim() + '\n' + toAdd.join('\n')
        : toAdd.join('\n');
      setHostlistContent(newContent);
    }
  };
  const handleOpenHostlistEditor = async () => {
    try {
      const lists = await GetBypassLists();
      setHostlists(lists);
      if (lists.length > 0) {
        setSelectedList(lists[0]);
        const content = await ReadBypassList(lists[0]);
        setHostlistContent(content);
      }
      setIsHostlistOpen(true);
    } catch (err) {
      console.error("Failed to load hostlists:", err);
    }
  };

  const handleSelectHostlist = async (name: string) => {
    setSelectedList(name);
    try {
      const content = await ReadBypassList(name);
      setHostlistContent(content);
    } catch (err) {
      console.error("Failed to read hostlist:", err);
    }
  };

  const handleSaveHostlist = async () => {
    setIsSavingHostlist(true);
    try {
      await SaveBypassList(selectedList, hostlistContent);
      ShowNotification("Успех", `Список ${selectedList} успешно сохранен.`);
      setIsHostlistOpen(false);
    } catch (err) {
      console.error("Failed to save hostlist:", err);
      ShowNotification("Ошибка", `Не удалось сохранить список: ${err}`);
    } finally {
      setIsSavingHostlist(false);
    }
  };

  const handleExportLogs = async () => {
    try {
      const logsStr = logs.join('\n');
      const saved = await ExportLogs(logsStr);
      if (saved) {
        ShowNotification("Успех", "Лог успешно сохранен.");
      }
    } catch (err) {
      console.error("Failed to export logs:", err);
    }
  };

  const handleRunDiagnostics = async () => {
    setIsDiagRunning(true);
    setIsDiagOpen(true);
    try {
      const results = await RunDiagnostics();
      setDiagResults(Array.isArray(results) ? results : []);
    } catch (err) {
      console.error(err);
    } finally {
      setIsDiagRunning(false);
    }
  };

  const handleClearCache = async () => {
    try {
      await ClearDiscordCache();
      ShowNotification("Кэш очищен", "Кэш Discord успешно очищен.");
    } catch (err) {
      console.error(err);
    }
  };

  const handleKillWinws2 = async () => {
    try {
      await KillWinws2();
      setStatus('Stopped');
      ShowNotification("Успех", "Движок обхода остановлен.");
    } catch (err) {
      // Previously the error was only written to the console, so a failed
      // force-stop looked exactly like a successful one.
      console.error(err);
      ShowNotification("Ошибка", "Не удалось остановить движок. Подробности в логе.");
    }
  };

  const isConnected = status === 'Running';
  const isConnecting = status === 'Starting';
  const disableMain = isConnecting || isScanning;
  const displayLogs = isScanning ? scanLogs : logs;

  const removeToast = (id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  };

  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map(toast =>
      setTimeout(() => removeToast(toast.id), 5000)
    );
    return () => timers.forEach(t => clearTimeout(t));
  }, [toasts]);

  const auraClass = 
    isScanning ? "aura-autotune" :
    isConnected ? "aura-active" :
    isConnecting ? "aura-connecting" :
    livePingData.status === 'error' ? "aura-error" : "";

  return (
    <div className={cn("flex flex-col h-screen w-screen relative app-drag transition-all duration-500", auraClass)}>
      
      {/* УВЕДОМЛЕНИЯ-ТОСТЫ */}
      <div className="fixed top-4 right-4 z-[10000] flex flex-col gap-2 pointer-events-none app-no-drag">
        {toasts.map(toast => (
          <div
            key={toast.id}
            className={cn(
              "pointer-events-auto sketch-box p-4 min-w-[280px] max-w-[320px] animate-in slide-in-from-right-full fade-in duration-300",
              toast.type === 'error' ? "bg-red-50 border-red-800" :
              toast.type === 'warning' ? "bg-orange-50 border-orange-800" :
              toast.type === 'success' ? "bg-green-50 border-green-800" :
              "bg-blue-50 border-blue-800"
            )}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1">
                <div className={cn(
                  "font-marker text-lg mb-1",
                  toast.type === 'error' ? "text-red-900" :
                  toast.type === 'warning' ? "text-orange-900" :
                  toast.type === 'success' ? "text-green-900" :
                  "text-blue-900"
                )}>
                  {toast.title}
                </div>
                <div className="text-sm font-bold text-gray-700 leading-snug">
                  {toast.message}
                </div>
              </div>
              <button
                onClick={() => removeToast(toast.id)}
                className="flex-shrink-0 text-gray-500 hover:text-gray-900 font-marker text-xl leading-none"
              >
                ×
              </button>
            </div>
          </div>
        ))}
      </div>
      
      {/* ОВЕРЛЕЙ — КОНФЛИКТЫ */}
      {conflictWarning.length > 0 && (
        <div className="fixed inset-0 z-[9998] flex items-center justify-center bg-orange-900/90 backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-300">
          <div className="w-full max-w-md bg-orange-50 sketch-box p-6 animate-in zoom-in-95 slide-in-from-bottom-4 duration-300">
            <div className="flex items-start gap-4 mb-4">
              <div className="w-12 h-12 bg-orange-600 rounded-full flex items-center justify-center flex-shrink-0">
                <span className="text-white font-marker text-3xl">!</span>
              </div>
              <div className="flex-1">
                <h3 className="text-2xl font-marker text-orange-900 mb-2">ОБНАРУЖЕНЫ КОНФЛИКТЫ!</h3>
                <div className="text-base font-bold text-orange-800 leading-snug mb-3 space-y-1">
                  {conflictWarning.map((conflict, idx) => (
                    <div key={idx}>{conflict}</div>
                  ))}
                </div>
                <p className="text-sm text-orange-700 leading-snug">
                  Эти процессы могут помешать работе Unbound. Завершите их, чтобы избежать конфликтов.
                </p>
              </div>
            </div>
            <div className="flex gap-3">
              <button
                onClick={() => setConflictWarning([])}
                className="flex-1 py-3 text-xl font-marker text-orange-600 hover:text-orange-900 hover:bg-orange-100 border-2 border-orange-800 rounded-xl shadow-[2px_2px_0_#7c2d12] transition-all duration-150 active:translate-y-1 active:shadow-none bg-white hover:scale-[1.02]"
              >
                Игнорировать
              </button>
              <button
                onClick={async () => {
                  await KillConflicts();
                  setConflictWarning([]);
                }}
                className="flex-1 py-3 text-xl font-marker bg-orange-600 text-white hover:bg-orange-700 border-2 border-orange-900 rounded-xl shadow-[2px_2px_0_#7c2d12] transition-all duration-150 active:translate-y-1 active:shadow-none hover:scale-[1.02]"
              >
                Завершить все
              </button>
            </div>
          </div>
        </div>
      )}
      
      {/* ОВЕРЛЕЙ — ПРАВА АДМИНИСТРАТОРА */}
      {privilegeError && (
        <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-red-900/90 backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-300">
          <div className="w-full max-w-md bg-red-50 sketch-box p-6 animate-in zoom-in-95 slide-in-from-bottom-4 duration-300">
            <div className="flex items-start gap-4 mb-4">
              <div className="w-12 h-12 bg-red-600 rounded-full flex items-center justify-center flex-shrink-0">
                <span className="text-white font-marker text-3xl">!</span>
              </div>
              <div className="flex-1">
                <h3 className="text-2xl font-marker text-red-900 mb-2">
                  {platform === 'darwin' ? 'ТРЕБУЮТСЯ ПРАВА ROOT (SUDO)!' : 'НУЖНЫ ПРАВА АДМИНИСТРАТОРА!'}
                </h3>
                <p className="text-base font-bold text-red-800 leading-snug mb-3">
                  {privilegeError}
                </p>
                <p className="text-sm text-red-700 leading-snug">
                  {platform === 'darwin'
                    ? 'Управление пакетами через pf (divert socket) требует прав root. Запустите приложение с правами sudo или разрешите права.'
                    : 'WinDivert не может перехватывать трафик без прав администратора. Нажмите правой кнопкой на unbound.exe и выберите «Запуск от имени администратора».'}
                </p>
              </div>
            </div>
            <button
              onClick={() => setPrivilegeError('')}
              className="w-full py-3 text-xl font-marker bg-red-600 text-white hover:bg-red-700 border-2 border-red-900 rounded-xl shadow-[2px_2px_0_#7f1d1d] transition-all duration-150 active:translate-y-1 active:shadow-none"
            >
              Понятно!
            </button>
          </div>
        </div>
      )}
      
      {/* 1. ШАПКА */}
      <div className="flex-none h-[42px] flex items-center justify-between px-4 z-10 border-b border-[var(--ui-border)] bg-[var(--ui-panel)]">
        <div className="flex items-center gap-2.5 app-no-drag">
          <svg width="22" height="22" viewBox="0 0 512 512" fill="none" xmlns="http://www.w3.org/2000/svg">
            <rect width="512" height="512" rx="128" fill="#090D16" />
            <circle cx="256" cy="256" r="200" stroke="#1E293B" strokeWidth="6" />
            <path d="M 160 336 C 120 296 120 200 176 152 C 232 104 320 112 352 176 C 368 208 352 256 312 288 L 272 320 C 248 336 216 336 192 320 Z" stroke="#F8FAFC" strokeWidth="28" strokeLinecap="round" strokeLinejoin="round" />
            <path d="M 336 160 C 376 200 384 296 328 344 C 272 392 184 384 152 320 C 136 288 152 240 192 208" stroke="#6366F1" strokeWidth="24" strokeLinecap="round" strokeLinejoin="round" />
            <circle cx="256" cy="256" r="20" fill="#F8FAFC" />
          </svg>
          <span className="font-bold text-sm text-[var(--ui-text)] tracking-wider">UNBOUND</span>
          <span className="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">{appVersion ? `v${appVersion}` : 'v0.1.0-refresh'}</span>
        </div>

        <div className="flex items-center gap-4 text-gray-500 app-no-drag">
          <button onClick={WindowMinimise} className="hover:text-[var(--ui-text)] transition-colors font-marker text-xl leading-none" title="Свернуть">
            _
          </button>
          <button onClick={HideWindowToTray} className="hover:text-red-500 font-marker text-xl leading-none pb-1" title="Закрыть в трей">
            X
          </button>
        </div>
      </div>

      {/* 2. ОСНОВНОЙ БЛОК */}
      <div className="flex-1 flex flex-col relative w-full px-8 pt-12 pb-10 min-h-0 app-no-drag overflow-y-auto">
        
        {/* Статус */}
        <div className="flex flex-col items-center justify-center mb-12">
          <h2 className={cn(
            "text-4xl font-marker tracking-widest text-center transition-colors duration-300",
            isConnected ? "text-green-600" : isConnecting || isScanning ? "text-blue-600 animate-pulse" : "text-gray-500"
          )}>
            {isScanning ? 'ТЕСТИРУЮ...' : status === 'Running' ? 'ПОДКЛЮЧЕНО!' : status === 'Stopped' ? 'ОТКЛЮЧЕНО' : status.toUpperCase()}
          </h2>
          <p className="text-sm font-semibold opacity-75 mt-3 flex items-center justify-center gap-2" style={{ color: 'var(--ui-text-muted)' }}>
            <span className={cn(
              "w-2.5 h-2.5 rounded-full inline-block",
              isScanning ? "bg-amber-500 animate-ping" : isConnected ? "bg-emerald-500 animate-pulse" : "bg-slate-400"
            )} />
            {isScanning && scanProgress ? scanProgress : isConnected ? 'Трафик обходит DPI' : 'Готово к запуску'}
          </p>
        </div>

        {/* Выбор профиля */}
        <div className="flex flex-col gap-2 mb-10 relative z-40">
          <div className="flex justify-between items-center px-2">
            <span className="text-lg font-bold" style={{ color: 'var(--ui-text)' }}>Профиль:</span>
            {isConnected && livePingData.services && Object.keys(livePingData.services).length > 0 ? (
              <div className="flex items-center gap-1.5 flex-wrap justify-end">
                {Object.entries(livePingData.services).map(([name, ms]) => (
                  <span key={name} className="text-[11px] px-2 py-0.5 rounded-md font-mono bg-emerald-500/15 text-emerald-400 border border-emerald-500/25">
                    {name} {ms}мс
                  </span>
                ))}
              </div>
            ) : livePingData.status === 'disconnected' ? (
              <span className="text-[11px] px-2 py-0.5 rounded-md font-mono bg-slate-500/15 text-slate-400 border border-slate-500/25">
                Offline
              </span>
            ) : livePingData.status === 'blocked' ? (
              <span className="text-[11px] px-2 py-0.5 rounded-md font-mono bg-red-500/15 text-red-400 border border-red-500/25">
                Blocked
              </span>
            ) : null}
          </div>
          
          <div className="flex gap-2">
            <div className="flex-1">
              <DoodleSelect
                value={selectedProfile}
                options={sortedProfiles}
                onChange={(val) => setSelectedProfile(val)}
                disabled={isConnected || disableMain || !selectedEngine}
                up={false}
              />
            </div>
            <button
              onClick={handleToggleFavorite}
              disabled={!selectedProfile}
              className={cn(
                "px-3 py-2 rounded-xl border-2 transition-all duration-200 hover:scale-105 active:scale-95",
                favoriteProfiles.includes(selectedProfile)
                  ? "bg-amber-400/20 border-amber-400/40 text-amber-400"
                  : "bg-transparent border-[var(--ui-border)] text-[var(--ui-text-muted)] hover:border-amber-400/40"
              )}
              title={favoriteProfiles.includes(selectedProfile) ? "Убрать из избранного" : "Добавить в избранное"}
            >
              {favoriteProfiles.includes(selectedProfile) ? '★' : '☆'}
            </button>
          </div>
          {selectedProfile === 'Custom Profile' && (
            <button
              onClick={openLuaEditor}
              className="mt-2 text-sm font-bold text-blue-600 hover:text-blue-800 hover:underline flex items-center justify-center gap-1.5 app-no-drag border-2 border-dashed border-blue-400/50 rounded-xl py-2 bg-blue-50/30 transition-all hover:bg-blue-50"
            >
              <SketchyGear className="w-4 h-4" />
              Настроить стратегию обхода (LUA)
            </button>
          )}
          {isConnected && <PingChart history={pingHistory} />}
        </div>

        {/* Кнопки действий */}
        <div className="flex flex-col gap-5 relative z-30">
          <button
            onClick={toggleConnection}
            disabled={disableMain}
            className={cn(
              "w-full py-4 text-2xl font-marker tracking-widest uppercase doodle-btn flex items-center justify-center gap-3 transition-all duration-200 hover:scale-[1.02] active:scale-[0.98]",
              isConnected && !disableMain ? "doodle-btn-red" : ""
            )}
          >
            {isConnected ? 'ОТКЛЮЧИТЬ!' : 'ПОДКЛЮЧИТЬ!'}
          </button>

          <div className="grid grid-cols-2 gap-4">
            <button
              onClick={isScanning ? CancelAutoTune : handleAutoTune}
              disabled={isConnected && !isScanning}
              className={cn(
                "flex items-center justify-center gap-2 py-3 doodle-btn font-bold text-lg relative overflow-hidden transition-all duration-200 hover:scale-[1.02] active:scale-[0.98]",
                isConnected && !isScanning ? "opacity-50 cursor-not-allowed" : ""
              )}
            >
              {isScanning ? (
                <>
                  <SketchySpinner className="w-6 h-6" />
                  Сканирую...
                </>
              ) : scanSuccess === true ? (
                <>
                  <SketchyCheck className="w-6 h-6 animate-in zoom-in duration-300" />
                  Успех!
                </>
              ) : scanSuccess === false ? (
                <>
                  <SketchyX className="w-6 h-6 animate-in zoom-in duration-300" />
                  Не удалось
                </>
              ) : (
                <>
                  <SketchyStar className="w-6 h-6" />
                  Автоподбор
                </>
              )}
            </button>
            <button 
              onClick={handleOpenSettings} 
              className="flex items-center justify-center gap-2 py-3 sketch-box hover:bg-gray-100 hover:shadow-[2px_2px_0_rgba(0,0,0,0.6)] font-bold text-lg transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
            >
              <SketchyGear className="w-6 h-6" />
              Настройки
            </button>
          </div>
        </div>
      </div>

      {/* 2.5. МОДАЛЬНОЕ ОКНО АВТОПОДБОРА (LIVE PROGRESS) */}
      {isScanning && (
        <div className="absolute inset-0 z-50 bg-black/65 backdrop-blur-md flex items-center justify-center p-6 animate-in fade-in duration-200">
          <div 
            className="w-full max-w-sm rounded-2xl p-6 space-y-4 shadow-2xl relative border app-no-drag"
            style={{ background: 'var(--ui-panel)', borderColor: 'var(--ui-border)', color: 'var(--ui-text)' }}
          >
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <SketchySpinner className="w-6 h-6 text-indigo-400 animate-spin" />
                <h3 className="font-bold text-lg">Автоподбор профиля</h3>
              </div>
              <button onClick={CancelAutoTune} className="p-1.5 rounded-lg hover:bg-white/10 transition-colors">
                <SketchyX className="w-5 h-5 text-gray-400" />
              </button>
            </div>

            <div className="space-y-2">
              <div className="flex justify-between text-xs font-semibold" style={{ color: 'var(--ui-text-muted)' }}>
                <span>Прогресс оптимизации</span>
                <span>{autotuneProgress?.percent || 0}%</span>
              </div>
              <div className="w-full h-3 rounded-full bg-black/20 overflow-hidden p-0.5 border border-white/10">
                <div 
                  className="h-full rounded-full bg-gradient-to-r from-indigo-500 to-sky-400 transition-all duration-300"
                  style={{ width: `${autotuneProgress?.percent || 0}%` }}
                />
              </div>
            </div>

            <div className="p-3.5 rounded-xl bg-black/10 border border-white/5 space-y-1">
              <p className="text-sm font-semibold truncate">
                {autotuneProgress?.msg || 'Анализируем стратегии обхода...'}
              </p>
              {autotuneProgress?.profile && (
                <p className="text-xs" style={{ color: 'var(--ui-text-muted)' }}>
                  Текущий профиль: <span className="font-mono text-indigo-400">{autotuneProgress.profile}</span>
                </p>
              )}
            </div>

            <button
              onClick={CancelAutoTune}
              className="w-full py-2.5 rounded-xl border border-red-500/30 bg-red-500/10 hover:bg-red-500/20 text-red-400 font-semibold text-sm transition-all"
            >
              Отменить автоподбор
            </button>
          </div>
        </div>
      )}
      {settings.showLogs && (
        <div 
          className={cn(
            "flex-none w-full border-t-2 transition-all duration-300 flex flex-col z-20 app-no-drag shadow-[0_-10px_20px_rgba(0,0,0,0.05)]",
            isLogExpanded ? "h-[220px]" : "h-14"
          )}
          style={{ background: 'var(--ui-panel)', borderColor: 'var(--ui-border)' }}
        >
          <div 
            className="flex items-center justify-between px-6 h-14 cursor-pointer hover:bg-black/5 dark:hover:bg-white/5 transition-colors"
            onClick={() => setIsLogExpanded(!isLogExpanded)}
          >
            <div className="flex items-center gap-3 font-bold text-lg" style={{ color: 'var(--ui-text)' }}>
              <SketchyTerminal className="w-6 h-6" />
              <span>{isScanning ? 'Лог сканера' : 'Журнал'}</span>
              {isLogExpanded && displayLogs.length > 0 && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleExportLogs();
                  }}
                  className="ml-3 px-3 py-1 border-2 text-xs font-marker rounded transition-transform duration-100 hover:scale-105 active:scale-95 shadow-[1px_1px_0_#222]"
                  style={{ background: 'var(--ui-panel)', borderColor: 'var(--ui-border)', color: 'var(--ui-text)' }}
                >
                  Экспорт
                </button>
              )}
            </div>
            <div className="font-marker text-xl" style={{ color: 'var(--ui-text-muted)' }}>
              {isLogExpanded ? '\\/' : '^'}
            </div>
          </div>

          <div 
            ref={logsContainerRef}
            className={cn(
            "flex-1 overflow-y-auto px-6 py-2 font-mono text-sm leading-relaxed transition-opacity duration-300 select-text",
            isLogExpanded ? "opacity-100 block" : "opacity-0 hidden"
          )}
          style={{ background: 'var(--ui-panel)', color: 'var(--ui-text)' }}
          >
            {displayLogs.length === 0 ? (
              <div className="h-full flex items-center justify-center font-hand text-lg font-bold" style={{ color: 'var(--ui-text-muted)' }}>Пока пусто...</div>
            ) : (
              <div className="space-y-2 pb-4">
                {displayLogs.map((rawLog, i) => {
                  const log = formatLog(rawLog);
                  const lowerLog = log.toLowerCase();
                  const isError = lowerLog.includes('error') || lowerLog.includes('fail') || lowerLog.includes('ошибк');
                  const isSuccess = lowerLog.includes('active') || lowerLog.includes('success') || lowerLog.includes('✓') || lowerLog.includes('start') || lowerLog.includes('запущ');
                  
                  return (
                    <div 
                      key={i} 
                      className={cn(
                        "break-words pl-2 border-l-2",
                        isError ? "text-red-500 font-bold border-red-500" : 
                        isSuccess ? "text-emerald-500 font-bold border-emerald-500" : 
                        "font-medium border-indigo-400/50"
                      )}
                      style={{ color: !isError && !isSuccess ? 'var(--ui-text)' : undefined }}
                    >
                      <span className="opacity-50 mr-2 select-none">~</span>
                      <span>{log}</span>
                    </div>
                  );
                })}
                <div ref={logsEndRef} />
              </div>
            )}
          </div>
        </div>
      )}

      {/* МОДАЛЬНОЕ ОКНО — РЕДАКТОР LUA */}
      {isLuaOpen && (
        <div 
          className="fixed inset-0 z-[9990] flex items-center justify-center bg-gray-900/40 backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-200"
          onClick={() => setIsLuaOpen(false)}
        >
          <div 
            className="w-full max-w-[420px] bg-[#fdfdfc] sketch-box flex flex-col max-h-[85vh] p-6 animate-in zoom-in-95 slide-in-from-bottom-4 duration-300 relative"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Шапка */}
            <div className="flex justify-between items-center border-b-2 border-gray-800 pb-3 mb-4">
              <div>
                <h3 className="text-xl font-bold text-gray-950 block leading-tight">Конструктор Lua-Стратегий</h3>
                <span className="text-xs text-gray-500 font-bold block mt-0.5">Настройка пользовательского обхода DPI</span>
              </div>
              <button 
                onClick={() => setIsLuaOpen(false)}
                className="text-gray-500 hover:text-gray-950 font-marker text-2xl leading-none"
              >
                ×
              </button>
            </div>

            {/* Табы */}
            <div className="flex gap-2 border-b-2 border-gray-200 mb-4">
              <button
                onClick={() => setLuaTab('builder')}
                className={cn(
                  "px-3 py-1.5 font-bold text-sm transition-all border-b-2",
                  luaTab === 'builder' ? "border-blue-600 text-blue-600" : "border-transparent text-gray-500 hover:text-gray-900"
                )}
              >
                Конструктор
              </button>
              <button
                onClick={() => {
                  if (luaIsAuto) {
                    setLuaCode(generateLuaCode({ fakeBlob: luaFakeBlob, pos: luaPos, fool: luaFool, ttl: luaTtl }));
                  }
                  setLuaTab('code');
                }}
                className={cn(
                  "px-3 py-1.5 font-bold text-sm transition-all border-b-2",
                  luaTab === 'code' ? "border-blue-600 text-blue-600" : "border-transparent text-gray-500 hover:text-gray-900"
                )}
              >
                Код LUA
              </button>
            </div>

            {/* Контент табов */}
            <div className="flex-1 overflow-y-auto mb-4 min-h-[280px]">
              {luaTab === 'builder' ? (
                <div className="space-y-4 pr-1">
                  {/* Готовые пресеты конструктора */}
                  <div className="flex flex-col gap-1">
                    <span className="text-xs font-bold text-gray-600 uppercase">Шаблоны быстрых стратегий:</span>
                    <div className="flex flex-wrap gap-1">
                      <button
                        type="button"
                        onClick={() => {
                          setLuaIsAuto(true);
                          setLuaFakeBlob('fake_default_tls');
                          setLuaPos('1');
                          setLuaFool('none');
                          setLuaTtl(0);
                        }}
                        className="px-2 py-1 text-xs font-bold bg-blue-100 hover:bg-blue-200 text-blue-900 rounded-lg border border-blue-300 transition-all"
                      >
                        🚀 MultiSplit
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setLuaIsAuto(true);
                          setLuaFakeBlob('fake_default_quic');
                          setLuaPos('2');
                          setLuaFool('md5sig');
                          setLuaTtl(4);
                        }}
                        className="px-2 py-1 text-xs font-bold bg-red-100 hover:bg-red-200 text-red-900 rounded-lg border border-red-300 transition-all"
                      >
                        🔥 Aggressive Fake
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setLuaIsAuto(true);
                          setLuaFakeBlob('fake_default_tls');
                          setLuaPos('host');
                          setLuaFool('md5sig');
                          setLuaTtl(3);
                        }}
                        className="px-2 py-1 text-xs font-bold bg-purple-100 hover:bg-purple-200 text-purple-900 rounded-lg border border-purple-300 transition-all"
                      >
                        🛡 MD5 Fooling
                      </button>
                    </div>
                  </div>
                  {/* Фейковый пакет */}
                  <div className="flex flex-col gap-1">
                    <span className="text-sm font-bold text-gray-700">Тип фейкового пакета (Payload):</span>
                    <DoodleSelect
                      value={luaFakeBlob === 'fake_default_tls' ? 'TLS ClientHello (Дефолт)' :
                             luaFakeBlob === 'fake_default_quic' ? 'QUIC Initial (YouTube)' :
                             luaFakeBlob === 'fake_http_request' ? 'HTTP GET Request' : luaFakeBlob}
                      options={['TLS ClientHello (Дефолт)', 'QUIC Initial (YouTube)', 'HTTP GET Request']}
                      onChange={(val) => {
                        setLuaIsAuto(true);
                        if (val.includes('TLS')) setLuaFakeBlob('fake_default_tls');
                        else if (val.includes('QUIC')) setLuaFakeBlob('fake_default_quic');
                        else setLuaFakeBlob('fake_http_request');
                      }}
                      up={false}
                    />
                  </div>

                  {/* Разделение пакетов */}
                  <div className="flex flex-col gap-1">
                    <span className="text-sm font-bold text-gray-700">Разделение пакета (Desync Position):</span>
                    <DoodleSelect
                      value={luaPos === '1' ? 'Разбить на 1-м байте' :
                             luaPos === '2' ? 'Разбить на 2-м байте' :
                             luaPos === 'host' ? 'Разбить по позиции Host/SNI' : luaPos}
                      options={['Разбить на 1-м байте', 'Разбить на 2-м байте', 'Разбить по позиции Host/SNI']}
                      onChange={(val) => {
                        setLuaIsAuto(true);
                        if (val.includes('1-м')) setLuaPos('1');
                        else if (val.includes('2-м')) setLuaPos('2');
                        else setLuaPos('host');
                      }}
                      up={false}
                    />
                  </div>

                  {/* Способ обмана DPI */}
                  <div className="flex flex-col gap-1">
                    <span className="text-sm font-bold text-gray-700">Метод обмана DPI (Fooling):</span>
                    <DoodleSelect
                      value={luaFool === 'none' ? 'Без обмана (Пассивный)' :
                             luaFool === 'badsum' ? 'Неверная чексумма (Активный)' :
                             luaFool === 'md5sig' ? 'Подмена MD5 Signature (Активный)' : luaFool}
                      options={['Без обмана (Пассивный)', 'Неверная чексумма (Активный)', 'Подмена MD5 Signature (Активный)']}
                      onChange={(val) => {
                        setLuaIsAuto(true);
                        if (val.includes('Без')) setLuaFool('none');
                        else if (val.includes('чексумма')) setLuaFool('badsum');
                        else setLuaFool('md5sig');
                      }}
                      up={false}
                    />
                  </div>

                  {/* TTL Слайдер */}
                  <div className="flex flex-col gap-1">
                    <div className="flex justify-between text-sm font-bold text-gray-700">
                      <span>Ограничение TTL пакета:</span>
                      <span className="text-blue-600">{luaTtl === 0 ? 'Отключено' : `${luaTtl} хопов`}</span>
                    </div>
                    <input
                      type="range"
                      min="0"
                      max="12"
                      value={luaTtl}
                      onChange={(e) => {
                        setLuaIsAuto(true);
                        setLuaTtl(parseInt(e.target.value, 10));
                      }}
                      className="w-full h-1.5 bg-gray-200 rounded-lg appearance-none cursor-pointer accent-blue-600"
                    />
                    <span className="text-xs text-gray-500 block leading-tight mt-1">
                      Маленький TTL (например, 3-5) отбрасывает фейковый пакет на провайдере, не допуская его до сайта.
                    </span>
                  </div>

                </div>
              ) : (
                <div className="flex flex-col h-full gap-2">
                  <span className="text-xs text-gray-500 font-bold">
                    {luaIsAuto ? 'Код генерируется автоматически. Изменение кода отключит конструктор.' : 'Ручной режим: конструктор отключен.'}
                  </span>
                  <textarea
                    value={luaCode}
                    onChange={(e) => {
                      setLuaIsAuto(false);
                      setLuaCode(e.target.value);
                    }}
                    className="w-full flex-1 min-h-[220px] p-3 border-2 border-gray-800 rounded-xl font-mono text-xs bg-gray-50 focus:outline-none focus:border-blue-600"
                    placeholder="-- Введите ваш Lua код здесь"
                  />
                </div>
              )}
            </div>

            {/* Кнопки */}
            <div className="flex gap-3 pt-3 border-t-2 border-gray-800">
              <button
                onClick={() => setIsLuaOpen(false)}
                className="flex-1 py-2 text-lg font-marker text-gray-600 hover:text-gray-900 hover:bg-gray-100 border-2 border-gray-800 rounded-xl shadow-[2px_2px_0_#222] transition-all duration-150 active:translate-y-1 active:shadow-none bg-white"
              >
                Отмена
              </button>
              <button
                onClick={saveLuaStrategy}
                className="flex-1 py-2 text-lg font-marker bg-blue-600 text-white hover:bg-blue-700 border-2 border-gray-900 rounded-xl shadow-[2px_2px_0_#222] transition-all duration-150 active:translate-y-1 active:shadow-none"
              >
                Сохранить
              </button>
            </div>

          </div>
        </div>
      )}

      {/* НАСТРОЙКИ */}
      {isSettingsOpen && (
        <div 
          className="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-200"
          style={{ background: 'rgba(0,0,0,0.55)' }}
          onClick={() => setIsSettingsOpen(false)}
        >
          <div 
            className="w-full max-w-[340px] flex flex-col max-h-[85vh] rounded-2xl border-2 overflow-hidden animate-in zoom-in-95 slide-in-from-bottom-4 duration-300"
            style={{
              background: 'var(--ui-panel)',
              borderColor: 'var(--ui-border)',
              boxShadow: '0 24px 64px rgba(0,0,0,0.4)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Шапка */}
            <div className="flex items-center justify-between px-4 py-3 border-b-2" style={{ borderColor: 'var(--ui-border)' }}>
              <div className="flex items-center gap-2">
                <SketchyGear className="w-5 h-5" style={{ color: 'var(--ui-accent)' }} />
                <h2 className="text-lg font-bold" style={{ color: 'var(--ui-text)' }}>Настройки</h2>
                {appVersion && (
                  <span className="text-xs font-semibold pb-0.5" style={{ color: 'var(--ui-text-muted)' }}>v{appVersion}</span>
                )}
              </div>
              <button
                onClick={() => setIsSettingsOpen(false)}
                className="w-7 h-7 flex items-center justify-center rounded-lg transition-colors duration-150"
                style={{ color: 'var(--ui-text-muted)', background: 'transparent' }}
                onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = 'var(--ui-text)'}
                onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = 'var(--ui-text-muted)'}
              >
                ✕
              </button>
            </div>

            {/* Содержимое настроек */}
            <div className="px-4 py-2 overflow-y-auto flex-1 max-h-[62vh] space-y-4 pr-1">
              <DoodleCheckbox 
                id="autoStart" 
                label="Автозапуск"
                desc="Запускать Unbound при старте системы"
                checked={settings.autoStart} 
                onChange={() => setSettings({...settings, autoStart: !settings.autoStart})} 
              />

              <DoodleCheckbox 
                id="startMinimized" 
                label="Тихий старт"
                desc="Запускать свёрнутым в системный трей"
                checked={settings.startMinimized} 
                onChange={() => setSettings({...settings, startMinimized: !settings.startMinimized})} 
              />

              <DoodleCheckbox 
                id="showLogs" 
                label="Показать журнал"
                desc="Показать/скрыть панель логов внизу"
                checked={settings.showLogs} 
                onChange={() => setSettings({...settings, showLogs: !settings.showLogs})} 
              />

              {platform !== 'darwin' && (
                <DoodleCheckbox
                  id="enableTCPTimestamps"
                  label="TCP Timestamps"
                  desc="Улучшить совместимость с некоторыми провайдерами"
                  checked={settings.enableTCPTimestamps}
                  onChange={() => setSettings({...settings, enableTCPTimestamps: !settings.enableTCPTimestamps})}
                />
              )}

              <DoodleCheckbox
                id="discordCacheAutoClean"
                label="Очистка Discord"
                desc="Автоматически очищать кэш Discord при запуске"
                checked={settings.discordCacheAutoClean}
                onChange={() => setSettings({...settings, discordCacheAutoClean: !settings.discordCacheAutoClean})}
              />

              <DoodleCheckbox
                id="secureDns"
                label="Безопасный DNS (DoH)"
                desc="Использовать Cloudflare DNS (1.1.1.1) для защиты от DNS Spoofing"
                checked={settings.secureDns}
                onChange={() => setSettings({...settings, secureDns: !settings.secureDns})}
              />

              <DoodleCheckbox
                id="autoReconnect"
                label="Авто-реконнект"
                desc="Автоматически переключать профиль при блокировке"
                checked={settings.autoReconnect}
                onChange={() => {
                  const newVal = !settings.autoReconnect;
                  setSettings({...settings, autoReconnect: newVal});
                  if (newVal && isConnected) {
                    AutoReconnectMonitor();
                  } else {
                    StopAutoReconnect();
                  }
                }}
              />

              {/* Theme selector */}
              <div className="flex flex-col gap-2 p-3 rounded-xl border-2 relative z-40" style={{ background: 'var(--ui-panel)', borderColor: 'var(--ui-border)' }}>
                <div>
                  <span className="text-sm font-bold block leading-none" style={{ color: 'var(--ui-text)' }}>Тема интерфейса</span>
                  <span className="text-xs block mt-1" style={{ color: 'var(--ui-text-muted)' }}>Выберите стиль оформления</span>
                </div>
                <DoodleSelect
                  value={theme === 'system' ? 'Авто (как в системе)' :
                         theme === 'dark' ? 'Modern Dark' : 
                         theme === 'light' ? 'Modern Light' :
                         theme === 'liquid-glass' || theme === 'macos26' ? 'macOS Glass' :
                         'Doodle Jump'}
                  options={[
                    'Авто (как в системе)',
                    'Doodle Jump',
                    'Modern Dark',
                    'Modern Light', 
                    'macOS Glass'
                  ]}
                  onChange={(val) => {
                    const themeMap: Record<string, string> = {
                      'Авто (как в системе)': 'system',
                      'Doodle Jump': 'doodle',
                      'Modern Dark': 'dark',
                      'Modern Light': 'light',
                      'macOS Glass': 'liquid-glass'
                    };
                    setThemeState(themeMap[val] || 'system');
                  }}
                  up={true}
                />
              </div>

              {/* Startup profile selector */}
              <div className="flex flex-col gap-2 p-3 rounded-xl border-2 relative z-30" style={{ background: 'var(--ui-panel)', borderColor: 'var(--ui-border)' }}>
                <div>
                  <span className="text-sm font-bold block leading-none" style={{ color: 'var(--ui-text)' }}>Профиль при запуске</span>
                  <span className="text-xs block mt-1" style={{ color: 'var(--ui-text-muted)' }}>Какой профиль загружать при старте?</span>
                </div>
                <DoodleSelect
                  value={settings.startupProfileMode}
                  options={["Последний использованный", "Автоподбор", ...profiles]}
                  onChange={(val) => setSettings({...settings, startupProfileMode: val})}
                  up={true}
                />
              </div>

              {/* System actions */}
              <div className="space-y-2 pt-2 border-t-2" style={{ borderColor: 'var(--ui-border)' }}>
                <span className="text-xs font-bold uppercase tracking-wider block mb-2" style={{ color: 'var(--ui-text-muted)' }}>Системные действия</span>
                <button 
                  onClick={handleRunDiagnostics}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-blue-50 hover:bg-blue-100 text-blue-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyTerminal className="w-4 h-4" />
                  Диагностика системы
                </button>
                <button 
                  onClick={handleOpenHostlistEditor}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-green-50 hover:bg-green-100 text-green-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyTerminal className="w-4 h-4 text-green-700" />
                  Редактор списков обхода
                </button>
                <button 
                  onClick={handleVerifyAssets}
                  disabled={isVerifyingAssets}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-emerald-50 hover:bg-emerald-100 text-emerald-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyStar className="w-4 h-4 text-emerald-600" />
                  {isVerifyingAssets ? 'Проверка хешей...' : 'Проверить целостность файлов (SHA256)'}
                </button>
                <button 
                  onClick={handleDiagnosticReport}
                  disabled={isGeneratingReport}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-blue-50 hover:bg-blue-100 text-blue-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyTerminal className="w-4 h-4 text-blue-600" />
                  {isGeneratingReport ? 'Генерация отчёта...' : 'Экспорт диагностического отчёта'}
                </button>
                <button 
                  onClick={handleUpdateHostlists}
                  disabled={isUpdatingHostlists}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-purple-50 hover:bg-purple-100 text-purple-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyGear className="w-4 h-4 text-purple-600" />
                  {isUpdatingHostlists ? 'Обновление...' : 'Обновить хостлисты'}
                </button>
                <button 
                  onClick={handleClearCache}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-gray-50 hover:bg-gray-100 text-gray-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyX className="w-4 h-4" />
                  Очистить кэш Discord
                </button>
                <button 
                  onClick={handleKillWinws2}
                  className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-orange-50 hover:bg-orange-100 text-orange-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                >
                  <SketchyX className="w-4 h-4" />
                  {platform === 'darwin' ? 'Остановить tpws' : 'Остановить winws2.exe'}
                </button>
                <button 
                  onClick={async () => {
                    try {
                      await QuitApp();
                    } catch (err) {
                      console.error(err);
                    }
                  }}
                  className="w-full flex items-center justify-center gap-2 py-2.5 rounded-xl bg-red-600 hover:bg-red-700 text-white font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98] shadow-[2px_2px_0_#222] border-2 border-gray-900"
                >
                  <SketchyX className="w-4 h-4 text-white" />
                  Выйти из Unbound (Закрыть приложение)
                </button>
              </div>
            </div>

            {/* Footer buttons */}
            <div className="flex gap-3 px-4 py-3 mt-auto border-t-2" style={{ borderColor: 'var(--ui-border)', background: 'var(--ui-panel)' }}>
              <button
                onClick={() => setIsSettingsOpen(false)}
                className="flex-1 py-2.5 text-sm font-semibold rounded-xl border-2 transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                style={{ color: 'var(--ui-text-muted)', borderColor: 'var(--ui-border)', background: 'transparent' }}
                onMouseEnter={e => {
                  (e.currentTarget as HTMLElement).style.color = 'var(--ui-text)';
                  (e.currentTarget as HTMLElement).style.background = 'color-mix(in srgb, var(--ui-border) 60%, transparent)';
                }}
                onMouseLeave={e => {
                  (e.currentTarget as HTMLElement).style.color = 'var(--ui-text-muted)';
                  (e.currentTarget as HTMLElement).style.background = 'transparent';
                }}
              >
                Отмена
              </button>
              <button
                onClick={handleSaveSettings}
                className="flex-1 py-2.5 text-sm font-bold rounded-xl transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
                style={{ background: 'var(--ui-accent-gradient)', color: '#ffffff' }}
              >
                Сохранить
              </button>
            </div>
            
            <div className="px-4 py-2 flex justify-center">
              <span className="text-xs" style={{ color: 'var(--ui-text-muted)' }}>{appVersion ? `v${appVersion}` : 'v0.1.0-refresh'} — Unbound Refresh</span>
            </div>
          </div>
        </div>
      )}

      {/* ДИАГНОСТИКА */}
      {isDiagOpen && (
        <div 
          className="fixed inset-0 z-[100] flex items-center justify-center bg-blue-900/40 backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-200"
          onClick={() => setIsDiagOpen(false)}
        >
          <div 
            className="w-full max-w-[360px] bg-[#fdfdfc] sketch-box flex flex-col max-h-[80vh] p-1 animate-in zoom-in-95 slide-in-from-bottom-4 duration-300"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-4 py-3 border-b-2 border-gray-200 mb-2">
              <div className="flex items-center gap-2">
                <SketchyTerminal className="w-6 h-6 text-blue-600" />
                <h2 className="text-xl font-marker text-gray-800">Проверка системы</h2>
              </div>
              <button onClick={() => setIsDiagOpen(false)} className="text-gray-500 hover:text-black font-marker text-xl">X</button>
            </div>

            <div className="px-4 py-4 overflow-y-auto space-y-4 flex-1">
              {isDiagRunning ? (
                <div className="flex flex-col items-center justify-center py-12 gap-4">
                  <SketchySpinner className="w-12 h-12 text-blue-500" />
                  <span className="font-marker text-xl text-blue-600">Проверяю систему...</span>
                </div>
              ) : (
                <div className="space-y-3">
                  {diagResults.map((res, idx) => (
                    <div key={idx} className={cn(
                      "p-3 sketch-box border-2 transition-all duration-200",
                      res.IsError ? "bg-red-50 border-red-300" : "bg-green-50 border-green-300"
                    )}>
                      <div className="flex justify-between items-start mb-1">
                        <span className="font-bold text-gray-900">{res.Component}</span>
                        <span className={cn(
                          "text-xs px-2 py-0.5 rounded-full font-bold uppercase",
                          res.IsError ? "bg-red-200 text-red-700" : "bg-green-200 text-green-700"
                        )}>{res.Status}</span>
                      </div>
                      <p className="text-sm text-gray-700 leading-snug">{res.Details}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="p-4 border-t-2 border-gray-200">
              <button
                onClick={() => setIsDiagOpen(false)}
                className="w-full py-3 text-xl font-marker doodle-btn hover:scale-[1.02] active:scale-[0.98]"
              >
                Понятно!
              </button>
            </div>
          </div>
        </div>
      )}

      {/* РЕДАКТОР СПИСКОВ ОБХОДА */}
      {isHostlistOpen && (
        <div 
          className="fixed inset-0 z-[120] flex items-center justify-center bg-black/50 backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-200"
          onClick={() => setIsHostlistOpen(false)}
        >
          <div 
            className="w-full max-w-[360px] bg-[#fdfdfc] sketch-box flex flex-col max-h-[85vh] p-1 animate-in zoom-in-95 slide-in-from-bottom-4 duration-300"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-4 py-3 border-b-2 border-gray-200 mb-2">
              <div className="flex items-center gap-2">
                <SketchyTerminal className="w-6 h-6 text-green-600" />
                <h2 className="text-xl font-marker text-gray-800">Списки обхода</h2>
              </div>
              <button onClick={() => setIsHostlistOpen(false)} className="text-gray-500 hover:text-black font-marker text-xl">X</button>
            </div>

            <div className="px-4 py-2 space-y-2 flex-1 flex flex-col overflow-hidden">
              <div className="flex flex-col gap-1 relative z-50">
                <span className="text-xs font-bold text-gray-600 uppercase">Выберите файл списков:</span>
                <DoodleSelect
                  value={selectedList}
                  options={hostlists}
                  onChange={handleSelectHostlist}
                />
              </div>

              <div className="flex-1 flex flex-col min-h-[250px] mt-2 relative z-10 overflow-hidden">
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs font-bold text-gray-600 uppercase">Домены (по одному на строке):</span>
                </div>
                <div className="flex flex-wrap gap-1 mb-2">
                  <button
                    type="button"
                    onClick={() => addHostlistPreset(['youtube.com', 'googlevideo.com', 'ytimg.com', 'youtu.be', 'ggpht.com'])}
                    className="px-2 py-0.5 text-[10px] font-bold bg-red-100 hover:bg-red-200 text-red-800 rounded border border-red-300 transition-colors"
                  >
                    + YouTube
                  </button>
                  <button
                    type="button"
                    onClick={() => addHostlistPreset(['discord.com', 'discord.gg', 'discord.media', 'discordapp.com', 'discordapp.net'])}
                    className="px-2 py-0.5 text-[10px] font-bold bg-indigo-100 hover:bg-indigo-200 text-indigo-800 rounded border border-indigo-300 transition-colors"
                  >
                    + Discord
                  </button>
                  <button
                    type="button"
                    onClick={() => addHostlistPreset(['telegram.org', 't.me', 'telegram.me', 'telegra.ph'])}
                    className="px-2 py-0.5 text-[10px] font-bold bg-sky-100 hover:bg-sky-200 text-sky-800 rounded border border-sky-300 transition-colors"
                  >
                    + Telegram
                  </button>
                  <button
                    type="button"
                    onClick={() => addHostlistPreset(['twitch.tv', 'ttvnw.net', 'jtvnw.net'])}
                    className="px-2 py-0.5 text-[10px] font-bold bg-purple-100 hover:bg-purple-200 text-purple-800 rounded border border-purple-300 transition-colors"
                  >
                    + Twitch
                  </button>
                </div>
                <textarea
                  value={hostlistContent}
                  onChange={(e) => setHostlistContent(e.target.value)}
                  className="w-full flex-1 p-3 font-mono text-xs border-2 border-gray-800 rounded-xl bg-white text-gray-800 focus:outline-none resize-none overflow-y-auto select-text shadow-[inset_2px_2px_4px_rgba(0,0,0,0.05)]"
                  placeholder="domain.com&#10;sub.domain.com"
                  spellCheck={false}
                />
              </div>
            </div>

            <div className="flex gap-4 p-4 border-t-2 border-gray-200 relative z-30">
              <button
                onClick={() => setIsHostlistOpen(false)}
                className="flex-1 py-2 text-lg font-marker text-gray-600 hover:text-gray-900 hover:bg-gray-100 border-2 border-gray-800 rounded-xl shadow-[2px_2px_0_#222] transition-all duration-150 active:translate-y-1 active:shadow-none bg-white"
              >
                Отмена
              </button>
              <button
                onClick={handleSaveHostlist}
                disabled={isSavingHostlist}
                className="flex-1 py-2 text-lg font-marker doodle-btn transition-all duration-150 flex items-center justify-center gap-2"
              >
                {isSavingHostlist ? <SketchySpinner className="w-4 h-4" /> : 'Сохранить'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
