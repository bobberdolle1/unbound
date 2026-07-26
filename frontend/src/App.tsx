import { useState, useEffect, useRef } from 'react';

import { cn } from './lib/cn';
import { formatLog } from './lib/format';
import { generateLuaCode, parseLuaCode } from './lib/lua';
import { SketchySpinner, SketchyX, SketchyStar, SketchyGear, SketchyTerminal, SketchyCheck } from './components/icons';
import { DoodleSelect } from './components/DoodleSelect';
import { DoodleCheckbox } from './components/DoodleCheckbox';
import { PingChart } from './components/PingChart';

import { GetEngineNames, GetProfiles, StartEngine, StopEngine, GetLogs, AutoTune, CancelAutoTune, GetSettings, SaveSettings, GetLivePing, ShowNotification, EnableAutoStart, DisableAutoStart, IsAutoStartEnabled, CheckConflicts, KillConflicts, CheckPrivileges, RunDiagnostics, ClearDiscordCache, KillWinws2, GetAppVersion, HideWindowToTray, GetBypassLists, ReadBypassList, SaveBypassList, ExportLogs, SaveCustomScript, LoadCustomScript } from '../wailsjs/go/main/App';
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
  
  const [theme, setThemeState] = useState<string>(localStorage.getItem('unbound-theme') || 'light');
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
    secureDns: boolean
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
    secureDns: false
  });
  const [livePingData, setLivePingData] = useState<{active: boolean, latency: number, status: string}>({active: false, latency: 0, status: 'stopped'});
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
  }, []);

  useEffect(() => {
    // Sync theme to body
    const classes = Array.from(document.body.classList).filter(c => c.startsWith('theme-'));
    classes.forEach(c => document.body.classList.remove(c));
    document.body.classList.add(`theme-${theme}`);
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
          setLivePingData({ active: data?.active || false, latency: lat, status: stat });
          if (stat === 'ok') {
            setPingHistory(prev => [...prev.slice(-14), lat]);
          }
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
          setPrivilegeError('Требуются права администратора. Перезапустите приложение от имени администратора.');
        }
      } catch (err) {
        setPrivilegeError('Требуются права администратора. Перезапустите приложение от имени администратора.');
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
        secureDns: s.secureDns || false
      });
    }).catch(() => {});
  }, []);

  useEffect(() => {
    if (selectedEngine) {
      console.log('[DEBUG] selectedEngine changed:', selectedEngine);
      GetProfiles(selectedEngine).then((p: string[]) => {
        console.log('[DEBUG] Loaded profiles:', p);
        setProfiles(p || []);
        if (p && p.length > 0 && !selectedProfile) {
          console.log('[DEBUG] Setting selectedProfile to:', p[0]);
          setSelectedProfile(p[0]);
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
    EventsOn('engine_log', (msg: string) => {
      setLogs(prev => [...prev, msg]);
    });
    EventsOn('autotune_complete', (data: {success: boolean, profile: string}) => {
      setScanSuccess(data.success);
      if (data.success && data.profile) {
        setSelectedProfile(data.profile);
        setScanProgress(`✅ Готово! Профиль: ${data.profile}`);
      } else {
        setScanProgress('❌ Рабочий профиль не найден. Проверьте права администратора и соединение.');
      }
      setTimeout(() => {
        setScanSuccess(null);
        setScanProgress('');
      }, 8000);
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
    console.log('[DEBUG] toggleConnection called, status:', status);
    try {
      if (status === 'Running' || status === 'Starting') {
        console.log('[DEBUG] Stopping engine...');
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
        console.log('[DEBUG] Starting engine:', eToStart, pToStart);
        await StartEngine(eToStart || "", pToStart || "");
      }
    } catch (err) {
      console.error('[ERROR] toggleConnection failed:', err);
      setLogs(prev => [...prev, `Ошибка: ${err}`]);
    }
  };

  const handleAutoTune = async () => {
    console.log('[DEBUG] handleAutoTune called');
    setIsScanning(true);
    setScanLogs([]);
    setScanSuccess(null);
    setScanProgress('🔍 Сканирую профили...');
    if (settings.showLogs) setIsLogExpanded(true);
    try {
      console.log('[DEBUG] Calling AutoTune...');
      const bestProfile = await AutoTune();
      console.log('[DEBUG] AutoTune result:', bestProfile);
      if (bestProfile && bestProfile !== "Failed") {
        setSelectedProfile(bestProfile);
        setScanProgress(`✅ Найдено: ${bestProfile}`);
        setScanSuccess(true);
      } else {
        setScanProgress('❌ Рабочий профиль не найден. Проверьте права администратора и интернет-соединение.');
        setScanSuccess(false);
      }
    } catch (err) {
      console.error('[ERROR] AutoTune failed:', err);
      setScanProgress('❌ Ошибка сканирования. Проверьте права администратора.');
      setScanSuccess(false);
    } finally {
      setIsScanning(false);
      setTimeout(() => {
        setScanSuccess(null);
        setScanProgress('');
      }, 8000);
    }
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
        secureDns: loadedSettings.secureDns || false
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
    toasts.forEach(toast => {
      const timer = setTimeout(() => removeToast(toast.id), 5000);
      return () => clearTimeout(timer);
    });
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
                <h3 className="text-2xl font-marker text-red-900 mb-2">НУЖНЫ ПРАВА АДМИНИСТРАТОРА!</h3>
                <p className="text-base font-bold text-red-800 leading-snug mb-3">
                  {privilegeError}
                </p>
                <p className="text-sm text-red-700 leading-snug">
                  WinDivert не может перехватывать трафик без прав администратора. Нажмите правой кнопкой на unbound.exe и выберите «Запуск от имени администратора».
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
      <div className="flex-none h-[40px] flex items-center justify-between px-5 z-10 border-b-2 border-red-300/60 bg-[#fdfdfc]">
        <div className="flex items-center gap-2 app-no-drag">
          <span className="font-marker text-xl text-gray-800 tracking-wider">UNBOUND!</span>
        </div>

        <div className="flex items-center gap-4 text-gray-500 app-no-drag">
          <button onClick={WindowMinimise} className="hover:text-black font-marker text-xl leading-none" title="Свернуть">
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
          <p className="text-lg font-bold text-gray-500 mt-3 underline decoration-gray-300 decoration-wavy">
            {isScanning && scanProgress ? scanProgress : isConnected ? 'Трафик обходит DPI!' : 'Готово к запуску'}
          </p>
        </div>

        {/* Выбор профиля */}
        <div className="flex flex-col gap-2 mb-10 relative z-40">
          <div className="flex justify-between items-end px-2">
            <span className="text-lg font-bold text-gray-700">Профиль:</span>
            {isConnected && (
              <span className={cn(
                "font-marker text-lg px-2 transform rotate-2",
                livePingData.status === 'ok' ? "text-green-600" : livePingData.status === 'blocked' ? "text-red-600" : "text-blue-500"
              )}>
                {livePingData.status === 'ok' ? `Пинг: ${livePingData.latency}мс` : livePingData.status === 'blocked' ? 'Заблокировано!' : '?'}
              </span>
            )}
          </div>
          
          <DoodleSelect 
            value={selectedProfile}
            options={profiles}
            onChange={(val) => setSelectedProfile(val)}
            disabled={isConnected || disableMain || !selectedEngine}
            up={false}
          />
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
                isScanning ? "!bg-red-300 !border-2 !shadow-[2px_2px_0_#222]" : 
                scanSuccess === true ? "!bg-green-300 !border-2 !shadow-[2px_2px_0_#222]" :
                scanSuccess === false ? "!bg-red-300 !border-2 !shadow-[2px_2px_0_#222]" :
                "!bg-yellow-300 !border-2 !shadow-[2px_2px_0_#222]",
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

      {/* 3. ЖУРНАЛ ЛОГОВ */}
      {settings.showLogs && (
        <div 
          className={cn(
            "flex-none w-full bg-[#f8f9fa] border-t-4 border-gray-800 transition-all duration-300 flex flex-col z-20 app-no-drag shadow-[0_-10px_20px_rgba(0,0,0,0.05)]",
            isLogExpanded ? "h-[220px]" : "h-14"
          )}
        >
          <div 
            className="flex items-center justify-between px-6 h-14 cursor-pointer hover:bg-gray-100 transition-colors"
            onClick={() => setIsLogExpanded(!isLogExpanded)}
          >
            <div className="flex items-center gap-3 text-gray-700 font-bold text-lg">
              <SketchyTerminal className="w-6 h-6" />
              <span>{isScanning ? 'Лог сканера' : 'Журнал'}</span>
              {isLogExpanded && displayLogs.length > 0 && (
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleExportLogs();
                  }}
                  className="ml-3 px-3 py-1 bg-white hover:bg-gray-100 border-2 border-gray-800 text-gray-800 rounded font-marker text-xs transition-transform duration-100 hover:scale-105 active:scale-95 shadow-[1px_1px_0_#222]"
                >
                  Экспорт
                </button>
              )}
            </div>
            <div className="font-marker text-xl text-gray-500">
              {isLogExpanded ? '\\/' : '^'}
            </div>
          </div>

          <div 
            ref={logsContainerRef}
            className={cn(
            "flex-1 overflow-y-auto px-6 py-2 font-mono text-sm leading-relaxed transition-opacity duration-300 bg-[#f8f9fa] text-blue-800 select-text",
            isLogExpanded ? "opacity-100 block" : "opacity-0 hidden"
          )}>
            {displayLogs.length === 0 ? (
              <div className="text-gray-400 h-full flex items-center justify-center font-hand text-lg font-bold">Пока пусто...</div>
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
                        "break-words pl-2 border-l-2 border-blue-200",
                        isError ? "text-red-600 font-bold" : 
                        isSuccess ? "text-green-700 font-bold" : 
                        "text-blue-800 font-medium"
                      )}
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
          className="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/40 backdrop-blur-sm p-4 app-no-drag animate-in fade-in duration-200"
          onClick={() => setIsSettingsOpen(false)}
        >
          <div 
            className="w-full max-w-[340px] bg-[#fdfdfc] sketch-box flex flex-col max-h-[85vh] p-1 animate-in zoom-in-95 slide-in-from-bottom-4 duration-300"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Шапка модалки */}
            <div className="flex items-center justify-between px-4 py-3 border-b-2 border-gray-200 mb-2">
              <div className="flex items-center gap-2">
                <SketchyGear className="w-6 h-6 text-gray-800" />
                <h2 className="text-xl font-marker text-gray-800">Настройки</h2>
                {appVersion && (
                  <span className="text-xs font-bold text-gray-400 self-end pb-1">v{appVersion}</span>
                )}
              </div>
              <button onClick={() => setIsSettingsOpen(false)} className="text-gray-500 hover:text-black font-marker text-xl transition-colors duration-150 hover:scale-110">
                X
              </button>
            </div>

            {/* Содержимое настроек */}
            <div className="px-4 py-2 overflow-y-visible space-y-4 flex-1">
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

              <DoodleCheckbox
                id="enableTCPTimestamps"
                label="TCP Timestamps"
                desc="Улучшить совместимость с некоторыми провайдерами"
                checked={settings.enableTCPTimestamps}
                onChange={() => setSettings({...settings, enableTCPTimestamps: !settings.enableTCPTimestamps})}
              />

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

              <div className="flex flex-col gap-2 p-3 bg-white border-2 border-gray-800 rounded-xl relative z-50 shadow-[2px_2px_0_#222]">
                <div>
                  <span className="text-lg font-bold text-gray-900 block leading-none">Тема интерфейса</span>
                  <span className="text-xs text-gray-600 block mt-1">Выберите стиль (v2.5.0)</span>
                </div>
                <DoodleSelect
                  value={theme === 'light' ? 'Standard White' : 
                         theme === 'dark' ? 'Modern Dark' :
                         theme === 'doodle' ? 'Doodle Jump' :
                         theme === 'liquid-glass' ? 'Liquid Glass' :
                         theme === 'win95' ? 'Windows 95' :
                         theme === 'ghost' ? 'Ghost in the Shell' :
                         theme === 'skeuomorphic' ? 'iOS 6 Classic' :
                         theme === 'winxp' ? 'Windows XP' :
                         theme === 'macos26' ? 'macOS Spatial' :
                         theme === 'win8' ? 'Windows 8 Metro' :
                         theme === 'ios26' ? 'iOS 26 Hologram' :
                         theme === 'gravity' ? 'Interstellar Gravity' : theme}
                  options={[
                    'Standard White', 
                    'Modern Dark', 
                    'Doodle Jump', 
                    'Liquid Glass', 
                    'Windows 95', 
                    'Ghost in the Shell',
                    'iOS 6 Classic',
                    'Windows XP',
                    'macOS Spatial',
                    'Windows 8 Metro',
                    'iOS 26 Hologram',
                    'Interstellar Gravity'
                  ]}
                  onChange={(val) => {
                    const themeMap: Record<string, string> = {
                      'Standard White': 'light',
                      'Modern Dark': 'dark',
                      'Doodle Jump': 'doodle',
                      'Liquid Glass': 'liquid-glass',
                      'Windows 95': 'win95',
                      'Ghost in the Shell': 'ghost',
                      'iOS 6 Classic': 'skeuomorphic',
                      'Windows XP': 'winxp',
                      'macOS Spatial': 'macos26',
                      'Windows 8 Metro': 'win8',
                      'iOS 26 Hologram': 'ios26',
                      'Interstellar Gravity': 'gravity'
                    };
                    setThemeState(themeMap[val]);
                  }}
                  up={true}
                />
              </div>

              <div className="flex flex-col gap-2 p-3 bg-white border-2 border-gray-800 rounded-xl relative z-50 shadow-[2px_2px_0_#222]">
                <div>
                  <span className="text-lg font-bold text-gray-900 block leading-none">Профиль при запуске</span>
                  <span className="text-xs text-gray-600 block mt-1">Какой профиль загружать при старте?</span>
                </div>
                <DoodleSelect
                  value={settings.startupProfileMode}
                  options={["Последний использованный", "Автоподбор", ...profiles]}
                  onChange={(val) => setSettings({...settings, startupProfileMode: val})}
                  up={true}
                />
              </div>
            </div>

            {/* Подвал настроек */}
            <div className="px-4 py-2 space-y-2 mb-2 relative z-[60]">
              <button 
                onClick={handleRunDiagnostics}
                className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-blue-50 hover:bg-blue-100 text-blue-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
              >
                <SketchyTerminal className="w-4 h-4" />
                Диагностика
              </button>
              <button 
                onClick={handleOpenHostlistEditor}
                className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-green-50 hover:bg-green-100 text-green-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
              >
                <SketchyTerminal className="w-4 h-4 text-green-700" />
                Редактор списков обхода
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
                className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-red-50 hover:bg-red-100 text-red-800 font-bold text-sm transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
              >
                <SketchyX className="w-4 h-4" />
                Завершить winws2.exe
              </button>
            </div>

            <div className="flex gap-4 px-4 py-4 mt-2 border-t-2 border-gray-200 relative z-[60]">
              <button
                onClick={() => setIsSettingsOpen(false)}
                className="flex-1 py-3 text-xl font-marker text-gray-600 hover:text-gray-900 hover:bg-gray-100 border-2 border-gray-800 rounded-xl shadow-[2px_2px_0_#222] transition-all duration-150 active:translate-y-1 active:shadow-none bg-white hover:scale-[1.02]"
              >
                Отмена
              </button>
              <button
                onClick={handleSaveSettings}
                className="flex-1 py-3 text-xl font-marker doodle-btn transition-all duration-150 hover:scale-[1.02] active:scale-[0.98]"
              >
                Сохранить!
              </button>
            </div>
            
            <div className="px-4 py-2 flex justify-center opacity-40">
              <span className="font-marker text-sm">v2.5.0 — Aura Design Edition</span>
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
                <span className="text-xs font-bold text-gray-600 uppercase mb-1">Домены (по одному на строке):</span>
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
