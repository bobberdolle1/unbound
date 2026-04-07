import { useState, useEffect, useRef } from 'react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// @ts-ignore
import { GetEngineNames, GetProfiles, StartEngine, StopEngine, GetLogs, AutoTune, CancelAutoTune, GetSettings, SaveSettings, GetLivePing, ShowNotification, EnableAutoStart, DisableAutoStart, IsAutoStartEnabled, CheckConflicts, KillConflicts, CheckPrivileges, RunDiagnostics, ClearDiscordCache, EnableTCPTimestamps, KillWinws2, GetAppVersion } from '../wailsjs/go/main/App';
// @ts-ignore
import { EventsOn, WindowMinimise, Quit } from '../wailsjs/runtime/runtime';

// === SKETCHY ICONS ===
const SketchySpinner = ({ className }: { className?: string }) => (
  <svg className={cn(className, "animate-spin")} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 12a9 9 0 1 1-6.219-8.56" />
  </svg>
);

const SketchyX = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
    <path d="M18 6L6 18M6 6l12 12" />
  </svg>
);

const SketchyStar = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M13 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L11 2Z" transform="translate(0.5, 0.5) rotate(2)"/>
    <path d="M13 2L15.09 8.26L22 9.27L17 14.14L18.18 21.02L12 17.77L5.82 21.02L7 14.14L2 9.27L8.91 8.26L11 2Z" transform="translate(-0.5, -0.5) rotate(-2)" opacity="0.4"/>
  </svg>
);

const SketchyGear = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="3.5" />
    <path d="M19.5 15.5c.2.6.4 1.2.8 1.8l-1.5 2.5-3-1.5c-.6.2-1.2.4-1.8.6v3.5h-4v-3.5c-.6-.2-1.2-.4-1.8-.6l-3 1.5-1.5-2.5c.4-.6.6-1.2.8-1.8H2.5v-4h3.5c-.2-.6-.4-1.2-.6-1.8l-1.5-2.5 2.5-1.5c.6.4 1.2.6 1.8.8V2.5h4v3.5c.6-.2 1.2-.4 1.8-.6l2.5-1.5 1.5 2.5c-.2.6-.4 1.2-.6 1.8h3.5v4h-3.5z" />
  </svg>
);

const SketchyTerminal = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
    <path d="M4.5 17.5l6-6-6-6" />
    <path d="M12.5 18.5h7" />
  </svg>
);

const SketchyCheck = ({ className }: { className?: string }) => (
  <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
    <path d="M20 6.5l-11 11-5-5" />
  </svg>
);

// === DOODLE COMPONENTS ===
const DoodleSelect = ({ value, options, onChange, disabled, up }: { value: string, options: string[], onChange: (v: string) => void, disabled?: boolean, up?: boolean }) => {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className="relative w-full" ref={dropdownRef}>
      <div 
        className={cn(
          "w-full sketch-input px-4 py-3 text-gray-900 font-bold text-base flex items-center justify-between transition-all duration-200 bg-white/80",
          disabled ? "opacity-60 cursor-not-allowed" : "cursor-pointer hover:bg-white hover:shadow-[3px_3px_0_rgba(0,0,0,0.8)] hover:scale-[1.01]",
          isOpen && "bg-white z-50 relative shadow-[3px_3px_0_rgba(0,0,0,0.8)]"
        )}
        onClick={() => !disabled && setIsOpen(!isOpen)}
      >
        <span className="truncate">{value || 'Выбрать стратегию'}</span>
        <span className={cn("font-marker font-black text-xl transition-transform duration-200", isOpen && "rotate-180")}>{isOpen ? 'x' : 'v'}</span>
      </div>
      
      {isOpen && (
        <ul className={cn(
          "absolute left-0 w-full z-[100] sketch-box max-h-48 overflow-y-auto py-2 shadow-[4px_4px_0_rgba(0,0,0,0.8)] animate-in slide-in-from-top-2 fade-in duration-200",
          up ? "bottom-[calc(100%+8px)]" : "top-[calc(100%+8px)]"
        )}>
          {options.map((opt) => (
            <li 
              key={opt}
              className={cn(
                "px-4 py-2 hover:bg-yellow-100 hover:text-yellow-900 cursor-pointer truncate font-bold text-base transition-all duration-150 hover:translate-x-1",
                value === opt ? "bg-yellow-200 text-yellow-900" : "text-gray-800"
              )}
              onClick={() => {
                onChange(opt);
                setIsOpen(false);
              }}
            >
              {opt}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
};

const DoodleCheckbox = ({ checked, onChange, id, label, desc }: { checked: boolean, onChange: () => void, id: string, label: string, desc: string }) => (
  <div className="flex items-start gap-4 p-3 sketch-box cursor-pointer hover:bg-white hover:shadow-[2px_2px_0_rgba(0,0,0,0.6)] transition-all duration-150 hover:scale-[1.01]" onClick={onChange}>
    <div className={cn(
      "w-7 h-7 flex-shrink-0 sketch-input flex items-center justify-center transition-all duration-200 bg-white",
      checked ? "text-green-600 scale-110" : "text-transparent scale-100"
    )}>
      {checked && <SketchyCheck className="w-5 h-5 animate-in zoom-in duration-200" />}
    </div>
    <div className="flex flex-col pt-0.5">
      <span className="text-[17px] font-bold text-gray-900 leading-none">{label}</span>
      <span className="text-sm text-gray-600 mt-1 leading-snug">{desc}</span>
    </div>
  </div>
);

const formatLog = (log: string) => {
  let formatted = log;
  if (formatted.includes('[STDOUT]')) formatted = formatted.replace('[STDOUT]', '').trim();
  if (formatted.includes('[STDERR]')) formatted = formatted.replace('[STDERR]', '').trim();
  return formatted;
};

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
  
  const [settings, setSettings] = useState<{
    autoStart: boolean, 
    startMinimized: boolean, 
    defaultProfile: string, 
    startupProfileMode: string, 
    gameFilter: boolean, 
    autoUpdateEnabled: boolean, 
    showLogs: boolean,
    enableTCPTimestamps: boolean,
    discordCacheAutoClean: boolean
  }>({
    autoStart: false,
    startMinimized: false,
    defaultProfile: 'Unbound Ultimate (God Mode)',
    startupProfileMode: 'Last Used',
    gameFilter: true,
    autoUpdateEnabled: true,
    showLogs: true,
    enableTCPTimestamps: false,
    discordCacheAutoClean: false
  });
  const [livePingData, setLivePingData] = useState<{active: boolean, latency: number, status: string}>({active: false, latency: 0, status: 'stopped'});
  const [privilegeError, setPrivilegeError] = useState<string>('');
  const [conflictWarning, setConflictWarning] = useState<string[]>([]);
  const [isUserScrolling, setIsUserScrolling] = useState<boolean>(false);
  const [toasts, setToasts] = useState<Array<{id: number, type: string, title: string, message: string}>>([]);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const logsContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if ('Notification' in window && Notification.permission === 'default') {
      Notification.requestPermission();
    }
    
    // Проверка прав при запуске
    const checkAdmin = async () => {
      try {
        const hasPriv = await CheckPrivileges();
        if (!hasPriv) {
          setPrivilegeError('Требуются права администратора. Перезапустите приложение от имени администратора.');
        }
      } catch (err) {
        console.error('Privilege check failed:', err);
        setPrivilegeError('Требуются права администратора. Перезапустите приложение от имени администратора.');
      }
    };
    
    checkAdmin();
    
    // Проверка конфликтов при запуске
    const checkConflicts = async () => {
      try {
        const conflicts = await CheckConflicts();
        if (conflicts && conflicts.length > 0) {
          setConflictWarning(conflicts);
        }
      } catch (err) {
        console.error('Conflict check failed:', err);
      }
    };
    
    checkConflicts();
    
    GetEngineNames().then((engines: string[]) => {
      console.log('[DEBUG] Loaded engines:', engines);
      setEngines(engines || []);
      if (engines && engines.length > 0) {
        console.log('[DEBUG] Setting selectedEngine to:', engines[0]);
        setSelectedEngine(engines[0]);
      }
    });
    
    GetSettings().then((loadedSettings: any) => {
      setSettings({
        autoStart: loadedSettings.autoStart || false,
        startMinimized: loadedSettings.startMinimized || false,
        defaultProfile: loadedSettings.defaultProfile || 'Unbound Ultimate (God Mode)',
        startupProfileMode: loadedSettings.startupProfileMode || 'Last Used',
        gameFilter: loadedSettings.gameFilter !== undefined ? loadedSettings.gameFilter : false,
        autoUpdateEnabled: loadedSettings.autoUpdateEnabled !== undefined ? loadedSettings.autoUpdateEnabled : true,
        showLogs: loadedSettings.showLogs !== undefined ? loadedSettings.showLogs : true,
        enableTCPTimestamps: loadedSettings.enableTCPTimestamps || false,
        discordCacheAutoClean: loadedSettings.discordCacheAutoClean || false
      });
    }).catch(() => {});
    
    EventsOn('status_changed', (newStatus: string) => setStatus(newStatus));
    EventsOn('privilege_error', (msg: string) => {
      setPrivilegeError(msg);
    });
    EventsOn('engine_error', (msg: string) => {
      ShowNotification("Ошибка движка", msg);
      setStatus("Stopped");
    });
    EventsOn('notification', (data: any) => {
      // Внутреннее уведомление-тост
      setToasts(prev => [...prev, {
        id: Date.now(),
        type: data.type || 'info',
        title: data.title,
        message: data.message
      }]);
      
      // Системное уведомление если разрешено
      if ('Notification' in window && Notification.permission === 'granted') {
        new Notification(data.title, { body: data.message });
      } else {
        setScanLogs(prev => [...prev, `🔔 ${data.title}: ${data.message}`]);
      }
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
        if ('Notification' in window && Notification.permission === 'granted') {
          new Notification('Автоподбор завершён', {
            body: `Найден рабочий профиль: ${data.profile}`,
            icon: '/icon.png'
          });
        }
      } else {
        setScanProgress('❌ Рабочий профиль не найден. Проверьте права администратора и соединение.');
      }
      setTimeout(() => {
        setScanSuccess(null);
        setScanProgress('');
      }, 8000);
    });
    
    const interval = setInterval(() => {
      if (!isScanning && status === 'Running') {
        GetLogs().then((l: string[]) => setLogs(l || []));
      }
    }, 2000);

    const pingInterval = setInterval(() => {
      if (status === 'Running') {
        GetLivePing().then((data: any) => {
          setLivePingData({ active: data?.active || false, latency: data?.latency || 0, status: data?.status || 'stopped' });
        }).catch(() => setLivePingData({active: false, latency: 0, status: 'error'}));
      } else {
        setLivePingData({active: false, latency: 0, status: 'stopped'});
      }
    }, 5000);
    
    return () => {
      clearInterval(interval);
      clearInterval(pingInterval);
    };
  }, [isScanning, status]);

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
        console.log('[DEBUG] Starting engine:', selectedEngine, selectedProfile);
        await StartEngine(selectedEngine, selectedProfile);
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
        startupProfileMode: loadedSettings.startupProfileMode || 'Last Used',
        gameFilter: loadedSettings.gameFilter !== undefined ? loadedSettings.gameFilter : false,
        autoUpdateEnabled: loadedSettings.autoUpdateEnabled !== undefined ? loadedSettings.autoUpdateEnabled : true,
        showLogs: loadedSettings.showLogs !== undefined ? loadedSettings.showLogs : true,
        enableTCPTimestamps: false,
        discordCacheAutoClean: false
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
      ShowNotification("Успех", "Все процессы winws2 завершены.");
    } catch (err) {
      console.error(err);
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

  return (
    <div className="flex flex-col h-screen w-screen relative app-drag">
      
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
          <button onClick={Quit} className="hover:text-red-500 font-marker text-xl leading-none pb-1" title="Закрыть">
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

              <div className="flex flex-col gap-2 p-3 bg-white border-2 border-gray-800 rounded-xl relative z-50 shadow-[2px_2px_0_#222]">
                <div>
                  <span className="text-lg font-bold text-gray-900 block leading-none">Профиль при запуске</span>
                  <span className="text-xs text-gray-600 block mt-1">Какой профиль загружать при старте?</span>
                </div>
                <DoodleSelect 
                  value={settings.startupProfileMode}
                  options={["Last Used", "Auto-Tune", ...profiles]}
                  onChange={(val) => setSettings({...settings, startupProfileMode: val})}
                  up={true}
                />
              </div>
            </div>

            {/* Подвал настроек */}
            <div className="px-4 py-2 space-y-2 mb-2 relative z-[60]">
               <button 
                onClick={handleRunDiagnostics}
                className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-blue-50 hover:bg-blue-100 text-blue-800 font-bold text-sm transition-all duration-150"
              >
                <SketchyTerminal className="w-4 h-4" />
                Диагностика
              </button>
              <button 
                onClick={handleClearCache}
                className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-gray-50 hover:bg-gray-100 text-gray-800 font-bold text-sm transition-all duration-150"
              >
                <SketchyX className="w-4 h-4" />
                Очистить кэш Discord
              </button>
              <button 
                onClick={handleKillWinws2}
                className="w-full flex items-center justify-center gap-2 py-2 sketch-box bg-red-50 hover:bg-red-100 text-red-800 font-bold text-sm transition-all duration-150"
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
              <span className="font-marker text-sm">v1.0.4 — 2026 Стабильная</span>
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
    </div>
  );
}
