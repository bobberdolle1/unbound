import { useState, useEffect, useRef } from 'react';
import { Power, Terminal, Shield, ShieldAlert, Minimize2, ChevronUp, ChevronDown, Radar, Code, Settings } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// @ts-ignore
import { GetEngineNames, GetProfiles, StartEngine, StopEngine, GetLogs, HideToTray, AutoTune, SaveCustomScript, LoadCustomScript, GetCurrentPing, GetSettings, SaveSettings, GetLivePing, CheckForUpdates, AddSubscription, GetXrayNodes, GenerateXrayConfig } from '../wailsjs/go/main/App';
// @ts-ignore
import { EventsOn } from '../wailsjs/runtime/runtime';

const formatLog = (log: string) => {
  let formatted = log;
  if (formatted.includes('[STDOUT]')) formatted = formatted.replace('[STDOUT]', '').trim();
  if (formatted.includes('[STDERR]')) formatted = formatted.replace('[STDERR]', '').trim();
  
  if (formatted.includes('--filter-tcp')) return "Configuring network packet filters...";
  if (formatted.includes('--lua=')) return "Loading DPI bypass logic...";
  if (formatted.includes('Command:')) return "Starting engine core...";
  if (formatted.toLowerCase().includes('windivert error') || formatted.toLowerCase().includes('binding failure')) return "Driver error. Ensure no conflicting bypass tools are active.";
  
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
  const [scanCancelled, setScanCancelled] = useState<boolean>(false);
  const [isEditorOpen, setIsEditorOpen] = useState<boolean>(false);
  const [scriptContent, setScriptContent] = useState<string>('');
  const [isSettingsOpen, setIsSettingsOpen] = useState<boolean>(false);
  const [settings, setSettings] = useState<{autoStart: boolean, startMinimized: boolean, defaultProfile: string, startupProfileMode: string, gameFilter: boolean, autoUpdateEnabled: boolean}>({
    autoStart: false,
    startMinimized: false,
    defaultProfile: 'Unbound Ultimate (God Mode)',
    startupProfileMode: 'Last Used',
    gameFilter: true,
    autoUpdateEnabled: true
  });
  const [pingData, setPingData] = useState<{active: boolean, latency: number, status: string, certValid?: boolean}>({active: false, latency: 0, status: 'stopped'});
  const [livePingData, setLivePingData] = useState<{active: boolean, latency: number, status: string}>({active: false, latency: 0, status: 'stopped'});
  const [isCheckingPing, setIsCheckingPing] = useState<boolean>(false);
  const [updateNotification, setUpdateNotification] = useState<{show: boolean, version: string, url: string, changelog: string}>({show: false, version: '', url: '', changelog: ''});
  const [xraySubscriptionLink, setXraySubscriptionLink] = useState<string>('');
  const [xrayNodes, setXrayNodes] = useState<any[]>([]);
  const [selectedXrayNode, setSelectedXrayNode] = useState<string>('');
  const [isFetchingSubscription, setIsFetchingSubscription] = useState<boolean>(false);
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    GetEngineNames().then((engines: string[]) => {
      setEngines(engines || []);
      if (engines && engines.length > 0) setSelectedEngine(engines[0]);
    });
    
    GetSettings().then((loadedSettings: any) => {
      if (loadedSettings?.autoUpdateEnabled !== false) {
        CheckForUpdates("2.0.1").then((updateInfo: any) => {
          if (updateInfo?.available) {
            setUpdateNotification({
              show: true,
              version: updateInfo.version || '',
              url: updateInfo.downloadUrl || '',
              changelog: updateInfo.changelog || ''
            });
          }
        }).catch(() => {});
      }
    }).catch(() => {});
    
    EventsOn('status_changed', (newStatus: string) => {
      setStatus(newStatus);
    });

    EventsOn('autotune_log', (msg: string) => {
      setScanLogs(prev => [...prev, msg]);
      setIsLogExpanded(true);
    });
    
    const interval = setInterval(() => {
      if (!isScanning) {
        GetLogs().then((l: string[]) => setLogs(l || []));
      }
    }, 500);

    const pingInterval = setInterval(() => {
      if (status === 'Running' && !isCheckingPing) {
        GetCurrentPing().then((data: any) => {
          setPingData({
            active: data?.active || false,
            latency: data?.latency || 0,
            status: data?.status || 'stopped',
            certValid: data?.certValid
          });
        }).catch(() => {
          setPingData({active: false, latency: 0, status: 'error'});
        });
      } else {
        setPingData({active: false, latency: 0, status: 'stopped'});
      }
    }, 5000);

    const livePingInterval = setInterval(() => {
      if (status === 'Running') {
        GetLivePing().then((data: any) => {
          setLivePingData({
            active: data?.active || false,
            latency: data?.latency || 0,
            status: data?.status || 'stopped'
          });
        }).catch(() => {
          setLivePingData({active: false, latency: 0, status: 'error'});
        });
      } else {
        setLivePingData({active: false, latency: 0, status: 'stopped'});
      }
    }, 5000);
    
    return () => {
      clearInterval(interval);
      clearInterval(pingInterval);
      clearInterval(livePingInterval);
    };
  }, [isScanning, status, isCheckingPing]);

  useEffect(() => {
    if (selectedEngine) {
      GetProfiles(selectedEngine).then((p: string[]) => {
        setProfiles(p || []);
        if (p && p.length > 0 && !selectedProfile) setSelectedProfile(p[0]);
      });
    }
  }, [selectedEngine]);

  useEffect(() => {
    if (isLogExpanded) {
      logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, scanLogs, isLogExpanded, isScanning]);

  const toggleConnection = async () => {
    if (status === 'Running' || status === 'Starting') {
      try {
        await StopEngine();
      } catch (err: any) {
        console.error('Error stopping:', err);
      }
    } else {
      try {
        await StartEngine(selectedEngine, selectedProfile);
      } catch (err: any) {
        console.error('Error starting:', err);
      }
    }
  };

  const handleAutoTune = async () => {
    setIsScanning(true);
    setScanCancelled(false);
    setScanLogs([]);
    setIsLogExpanded(true);
    try {
      const bestProfile = await AutoTune();
      if (bestProfile && bestProfile !== "Failed") {
        setSelectedProfile(bestProfile);
      }
    } catch (err) {
      console.error('AutoTune error:', err);
    } finally {
      setIsScanning(false);
    }
  };

  const handleCancelScan = () => {
    setScanCancelled(true);
    setIsScanning(false);
    setScanLogs(prev => [...prev, "Auto-Tune cancelled by user"]);
  };

  const handleOpenEditor = async () => {
    setIsEditorOpen(true);
    
    if (selectedProfile.includes('Xray') || selectedProfile.includes('VLESS')) {
      try {
        const nodes = await GetXrayNodes();
        setXrayNodes(nodes || []);
      } catch (err) {
        console.error('Failed to load Xray nodes:', err);
      }
    } else {
      try {
        const content = await LoadCustomScript();
        setScriptContent(content);
      } catch (err) {
        console.error('Failed to load custom script:', err);
      }
    }
  };

  const handleOpenSettings = async () => {
    setIsSettingsOpen(true);
    try {
      const loadedSettings = await GetSettings();
      setSettings({
        autoStart: loadedSettings.autoStart || false,
        startMinimized: loadedSettings.startMinimized || false,
        defaultProfile: loadedSettings.defaultProfile || 'Unbound Ultimate (God Mode)',
        startupProfileMode: loadedSettings.startupProfileMode || 'Last Used',
        gameFilter: loadedSettings.gameFilter !== undefined ? loadedSettings.gameFilter : true,
        autoUpdateEnabled: loadedSettings.autoUpdateEnabled !== undefined ? loadedSettings.autoUpdateEnabled : true
      });
    } catch (err) {
      console.error('Failed to load settings:', err);
    }
  };

  const handleSaveSettings = async () => {
    try {
      await SaveSettings(settings);
      setIsSettingsOpen(false);
    } catch (err) {
      console.error('Failed to save settings:', err);
    }
  };

  const handleSaveScript = async () => {
    try {
      await SaveCustomScript(scriptContent);
      setIsEditorOpen(false);
      setSelectedProfile('Custom Profile');
    } catch (err) {
      console.error('Failed to save custom script:', err);
    }
  };

  const handleManualPingCheck = async () => {
    setIsCheckingPing(true);
    try {
      const data = await GetCurrentPing();
      setPingData({
        active: data?.active || false,
        latency: data?.latency || 0,
        status: data?.status || 'stopped',
        certValid: data?.certValid
      });
    } catch (err) {
      setPingData({active: false, latency: 0, status: 'error'});
    } finally {
      setIsCheckingPing(false);
    }
  };

  const isConnected = status === 'Running';
  const isConnecting = status === 'Starting';
  const disableMain = isConnecting || isScanning;

  const displayLogs = isScanning ? scanLogs : logs;

  return (
    <div className="h-screen w-screen bg-zinc-950 flex flex-col font-sans text-zinc-100 overflow-hidden relative selection:bg-cyan-500/30">
      
      {/* Background ambient glow - subtle */}
      <div className={cn(
        "absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full blur-[120px] opacity-0 pointer-events-none transition-all duration-1000",
        isConnected ? "opacity-5 bg-emerald-500" : isScanning ? "opacity-3 bg-amber-500" : "opacity-0"
      )} />

      {/* Header */}
      <div 
        className="h-16 flex items-center justify-between px-6 border-b border-white/5 bg-zinc-950/40 backdrop-blur-xl z-10 shrink-0"
        style={{ WebkitAppRegion: 'drag' } as any}
      >
        <div className="flex items-center gap-3">
          {/* Minimalist shield with lightning bolt - no background */}
          <div className="w-8 h-8 flex items-center justify-center relative">
            <svg viewBox="0 0 24 24" className="w-8 h-8 text-cyan-400" fill="currentColor">
              {/* Shield outline */}
              <path d="M12 2L4 5v6c0 5.5 3.8 10.7 8 12 4.2-1.3 8-6.5 8-12V5l-8-3z" fill="none" stroke="currentColor" strokeWidth="1.5"/>
              {/* Lightning bolt */}
              <path d="M13 3l-3 7h3l-1 8 5-9h-3l2-6z" fill="currentColor"/>
            </svg>
          </div>
          <div>
            <h1 className="text-sm font-bold tracking-widest text-white/90">UNBOUND</h1>
            <p className="text-[9px] text-zinc-500 font-bold tracking-widest uppercase">CLEARFLOW ENGINE</p>
          </div>
        </div>

        <div className="flex items-center gap-4" style={{ WebkitAppRegion: 'no-drag' } as any}>
          {/* Live Ping Indicator */}
          {pingData.active && pingData.status === 'ok' && pingData.latency > 0 && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-bold tracking-widest border bg-cyan-500/10 text-cyan-400 border-cyan-500/30 shadow-[0_0_10px_rgba(6,182,212,0.2)]">
              <div className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse shadow-[0_0_8px_rgba(6,182,212,0.8)]" />
              {pingData.latency}ms
            </div>
          )}

          {/* Manual Ping Check Button */}
          <button 
            onClick={handleManualPingCheck}
            disabled={isCheckingPing || status !== 'Running'}
            className={cn(
              "p-1.5 rounded-lg transition-colors",
              isCheckingPing ? "text-cyan-400 animate-pulse" : "text-zinc-400 hover:text-cyan-400 hover:bg-white/10",
              status !== 'Running' && "opacity-30 cursor-not-allowed"
            )}
            title="Check Connection Ping"
          >
            <Radar className={cn("w-4 h-4", isCheckingPing && "animate-spin")} />
          </button>

          {/* Status Badge */}
          <div className={cn(
            "flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-bold tracking-widest border transition-all duration-500 shadow-sm",
            isConnected 
              ? "bg-emerald-500/10 text-emerald-400 border-emerald-500/30 shadow-[0_0_10px_rgba(16,185,129,0.2)]" 
              : isScanning || isConnecting
                ? "bg-amber-500/10 text-amber-400 border-amber-500/30 shadow-[0_0_10px_rgba(245,158,11,0.2)] animate-pulse"
                : "bg-red-500/10 text-red-400 border-red-500/30 shadow-[0_0_10px_rgba(239,68,68,0.2)]"
          )}>
            {isConnected ? <Shield className="w-3.5 h-3.5" /> : isScanning ? <Radar className="w-3.5 h-3.5 animate-spin" /> : <ShieldAlert className="w-3.5 h-3.5" />}
            {isScanning ? 'SCANNING' : status.toUpperCase()}
          </div>

          <button 
            onClick={handleOpenEditor} 
            className="p-1.5 rounded-lg hover:bg-white/10 transition-colors text-zinc-400 hover:text-cyan-400"
            title="Advanced Lua Editor"
          >
            <Code className="w-4 h-4" />
          </button>

          <button 
            onClick={handleOpenSettings} 
            className="p-1.5 rounded-lg hover:bg-white/10 transition-colors text-zinc-400 hover:text-cyan-400"
            title="Settings"
          >
            <Settings className="w-4 h-4" />
          </button>

          <button onClick={() => HideToTray()} className="p-1.5 rounded-lg hover:bg-white/10 transition-colors text-zinc-400 hover:text-white">
            <Minimize2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Main Hero Section */}
      <div className="flex-1 flex flex-col items-center justify-center gap-10 relative min-h-0 z-10 p-4">
        
        {/* Massive Button Container */}
        <div className="relative flex items-center justify-center shrink-0">
          
          {/* Ambient glow around button - more subtle */}
          <div className={cn(
            "absolute w-[200px] h-[200px] rounded-full blur-[60px] opacity-0 pointer-events-none transition-all duration-700",
            isConnected ? "opacity-10 bg-emerald-500" : "opacity-0"
          )} />
          
          {/* Scanning Ring */}
          {isScanning && (
            <div className="absolute w-[180px] h-[180px] rounded-full border-2 border-transparent border-t-amber-500/80 border-r-amber-500/20 animate-spin z-0" style={{ animationDuration: '1.5s' }} />
          )}

          <button
            onClick={toggleConnection}
            disabled={disableMain}
            style={{ WebkitAppRegion: 'no-drag' } as any}
            className={cn(
              "relative group rounded-full w-40 h-40 flex flex-col items-center justify-center shrink-0 transition-all duration-700 delay-75 z-10 p-4",
              isConnected 
                ? "bg-zinc-950 glow-on border border-emerald-500/50" 
                : "bg-zinc-900/80 backdrop-blur-md border border-white/5 shadow-[0_0_40px_rgba(0,0,0,0.5)]",
              disableMain && !isConnected ? "opacity-50 cursor-not-allowed scale-95" : "hover:border-white/10 hover:scale-105 active:scale-95"
            )}
          >
            {/* Inner background gradient */}
            <div className={cn(
              "absolute inset-3 rounded-full transition-all duration-700 z-0",
              isConnected ? "bg-gradient-to-b from-emerald-900/20 to-cyan-900/20" : "bg-gradient-to-b from-white/5 to-transparent"
            )} />

            <Power className={cn(
              "w-12 h-12 mb-2 z-10 transition-all duration-700 shrink-0",
              isConnected ? "text-emerald-400 drop-shadow-[0_0_15px_rgba(16,185,129,0.8)]" : "text-zinc-500 group-hover:text-zinc-300"
            )} />
            
            <span className={cn(
              "text-[10px] font-black tracking-[0.1em] z-10 transition-all duration-700 mt-1 text-center w-full break-words px-1 leading-tight",
              isConnected ? "text-transparent bg-clip-text bg-gradient-to-r from-emerald-400 to-cyan-400" : "text-zinc-500 group-hover:text-zinc-300"
            )}>
              {isConnected ? 'CONNECTED' : isConnecting ? 'CONNECTING' : 'TAP TO CONNECT'}
            </span>
          </button>
        </div>

        {/* Profile Card - improved layout */}
        <div 
          className="flex flex-row items-center justify-between w-full max-w-[500px] bg-zinc-900/60 border border-white/10 rounded-2xl p-5 backdrop-blur-xl shadow-2xl transition-all duration-500 gap-4"
          style={{ WebkitAppRegion: 'no-drag' } as any}
        >
          <div className="flex flex-col flex-1 min-w-0 group">
            <div className="flex items-center gap-2 mb-2">
              <Shield className="w-3.5 h-3.5 text-cyan-500 transition-transform duration-300 group-hover:scale-110 group-hover:rotate-12" />
              <span className="text-[10px] text-zinc-400 font-bold tracking-widest uppercase transition-colors duration-300 group-hover:text-cyan-400">
                Active Strategy
              </span>
            </div>
            <div className="relative w-full">
              <select 
                value={selectedProfile} 
                onChange={(e) => setSelectedProfile(e.target.value)} 
                disabled={isConnected || disableMain || !selectedEngine}
                className="w-full bg-zinc-800/50 border border-white/10 rounded-lg px-3 py-2 text-zinc-100 text-sm font-semibold tracking-wide outline-none cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed hover:border-cyan-500/50 focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/20 transition-all duration-300 appearance-none pr-8"
              >
                {profiles.map(p => <option key={p} value={p} className="bg-zinc-900 text-sm py-2">{p}</option>)}
              </select>
              <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-500 pointer-events-none transition-colors group-hover:text-cyan-400" />
            </div>
            
            {/* Live Connection Health Badge */}
            {isConnected && (
              <div className="flex items-center gap-2 mt-3 px-3 py-1.5 bg-zinc-800/50 rounded-lg border border-white/5">
                <span className="text-[9px] text-zinc-500 font-bold tracking-widest uppercase">Health</span>
                {livePingData.status === 'ok' && livePingData.latency > 0 ? (
                  <div className="flex items-center gap-1.5">
                    <div className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse shadow-[0_0_8px_rgba(52,211,153,0.8)]" />
                    <span className="text-xs font-bold text-emerald-400">{livePingData.latency}ms</span>
                  </div>
                ) : livePingData.status === 'blocked' ? (
                  <div className="flex items-center gap-1.5">
                    <div className="w-2 h-2 rounded-full bg-red-400 shadow-[0_0_8px_rgba(248,113,113,0.8)]" />
                    <span className="text-xs font-bold text-red-400">BLOCKED</span>
                  </div>
                ) : (
                  <div className="flex items-center gap-1.5">
                    <div className="w-2 h-2 rounded-full bg-amber-400 animate-pulse" />
                    <span className="text-xs font-bold text-amber-400">TESTING</span>
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="w-px bg-white/10 self-stretch my-1" />

          <button
            onClick={isScanning ? handleCancelScan : handleAutoTune}
            disabled={isConnected}
            className={cn(
              "flex flex-col items-center justify-center px-4 py-3 rounded-xl transition-all duration-300 min-w-[90px] shrink-0",
              isScanning 
                ? "bg-red-500/20 text-red-400 hover:bg-red-500/30 shadow-[0_0_15px_rgba(239,68,68,0.3)]" 
                : isConnected
                  ? "opacity-30 cursor-not-allowed text-zinc-500"
                  : "bg-cyan-500/10 hover:bg-cyan-500/20 text-cyan-400 hover:text-cyan-300 border border-cyan-500/30 hover:border-cyan-500/50 hover:shadow-[0_0_20px_rgba(6,182,212,0.3)]"
            )}
          >
            <Radar className={cn("w-5 h-5 mb-1.5", isScanning ? "animate-spin" : "")} />
            <span className="text-[9px] font-bold tracking-widest uppercase text-center">{isScanning ? 'Cancel' : 'Auto-Tune'}</span>
          </button>
        </div>
      </div>

      {/* Bottom Terminal Logs */}
      <div 
        className={cn(
          "bg-zinc-950/90 backdrop-blur-2xl border-t border-white/5 flex flex-col transition-all duration-500 ease-[cubic-bezier(0.4,0,0.2,1)] shrink-0 z-20 shadow-[0_-10px_40px_rgba(0,0,0,0.3)]",
          isLogExpanded ? "h-64" : "h-12"
        )}
        style={{ WebkitAppRegion: 'no-drag' } as any}
      >
        <div 
          className="flex items-center justify-between px-6 py-3 cursor-pointer hover:bg-white/5 transition-colors group h-12"
          onClick={() => setIsLogExpanded(!isLogExpanded)}
        >
          <div className="flex items-center gap-3">
            <Terminal className="w-4 h-4 text-zinc-500 group-hover:text-cyan-500 transition-colors" />
            <span className="text-[10px] font-bold text-zinc-400 group-hover:text-zinc-300 uppercase tracking-widest transition-colors">
              {isScanning ? 'Telemetry: Auto-Tune' : 'Telemetry: Engine'}
            </span>
          </div>
          <div className="flex items-center gap-4">
            <span className="text-[9px] text-zinc-500 font-mono tracking-widest bg-black/60 px-2 py-1 rounded border border-white/5">{displayLogs.length} LOGS</span>
            {isLogExpanded ? <ChevronDown className="w-4 h-4 text-zinc-500" /> : <ChevronUp className="w-4 h-4 text-zinc-500" />}
          </div>
        </div>

        <div className={cn(
          "flex-1 overflow-y-auto px-6 pb-4 font-mono text-[10px] leading-[1.6] transition-opacity duration-300",
          isLogExpanded ? "opacity-100" : "opacity-0 hidden"
        )}>
          {displayLogs.length === 0 ? (
            <div className="text-zinc-600 h-full flex items-center justify-center font-medium tracking-widest">AWAITING TELEMETRY STREAM...</div>
          ) : (
            <div className="space-y-[3px]">
              {displayLogs.map((rawLog, i) => {
                const log = formatLog(rawLog);
                const lowerLog = log.toLowerCase();
                const isError = lowerLog.includes('error') || lowerLog.includes('fail') || lowerLog.includes('unexpected') || lowerLog.includes('cannot') || lowerLog.includes('invalid') || lowerLog.includes('unknown');
                const isSuccess = lowerLog.includes('active') || lowerLog.includes('success') || lowerLog.includes('✓') || lowerLog.includes('start');
                
                return (
                  <div 
                    key={i} 
                    className={cn(
                      "pl-3 border-l py-0.5 break-words",
                      isError ? "border-red-500/50 text-red-400 bg-red-500/5 rounded-r" : 
                      isSuccess ? "border-emerald-500/50 text-emerald-400 bg-emerald-500/5 rounded-r" : 
                      "border-white/10 text-zinc-400 hover:bg-white/5 rounded-r transition-colors"
                    )}
                  >
                    <span className="text-zinc-600 mr-3 select-none">[{new Date().toLocaleTimeString([], {hour12: false, hour: '2-digit', minute: '2-digit', second:'2-digit'})}]</span>
                    <span>{log}</span>
                  </div>
                );
              })}
              <div ref={logsEndRef} />
            </div>
          )}
        </div>
      </div>

      {/* Dynamic Advanced Editor Modal */}
      {isEditorOpen && (
        <div 
          className="absolute inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md"
          onClick={() => setIsEditorOpen(false)}
        >
          <div 
            className="w-[90%] max-w-4xl h-[80%] bg-zinc-900/95 border border-white/10 rounded-2xl shadow-2xl flex flex-col overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-white/10 bg-zinc-950/50 backdrop-blur-xl">
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-cyan-500/10 flex items-center justify-center border border-cyan-500/30">
                  <Code className="w-4 h-4 text-cyan-400" />
                </div>
                <div>
                  <h2 className="text-sm font-bold tracking-widest text-white/90">
                    {selectedProfile.includes('Xray') || selectedProfile.includes('VLESS') ? 'XRAY SUBSCRIPTION MANAGER' : 
                     selectedProfile.includes('AmneziaWG') ? 'AMNEZIAWG CONFIG IMPORT' : 
                     'ADVANCED LUA EDITOR'}
                  </h2>
                  <p className="text-[9px] text-zinc-500 font-bold tracking-widest uppercase">
                    {selectedProfile.includes('Xray') || selectedProfile.includes('VLESS') ? 'VLESS/Reality Node Configuration' : 
                     selectedProfile.includes('AmneziaWG') ? 'WireGuard Configuration File' : 
                     'Custom DPI Bypass Strategy'}
                  </p>
                </div>
              </div>
            </div>

            {/* Editor Area - Dynamic Content */}
            <div className="flex-1 p-6 overflow-hidden flex flex-col gap-4">
              
              {/* Xray Subscription UI */}
              {(selectedProfile.includes('Xray') || selectedProfile.includes('VLESS')) && (
                <>
                  <div className="flex gap-3">
                    <input
                      type="text"
                      value={xraySubscriptionLink}
                      onChange={(e) => setXraySubscriptionLink(e.target.value)}
                      placeholder="Paste subscription link or vless:// URI..."
                      className="flex-1 bg-zinc-900/80 border border-white/10 rounded-lg px-4 py-2 text-zinc-100 text-sm focus:outline-none focus:border-cyan-500/50 focus:ring-2 focus:ring-cyan-500/20 transition-all"
                    />
                    <button
                      onClick={async () => {
                        setIsFetchingSubscription(true);
                        try {
                          const nodes = await AddSubscription(xraySubscriptionLink);
                          setXrayNodes(nodes || []);
                          setXraySubscriptionLink('');
                        } catch (err: any) {
                          console.error('Failed to fetch subscription:', err);
                        } finally {
                          setIsFetchingSubscription(false);
                        }
                      }}
                      disabled={!xraySubscriptionLink || isFetchingSubscription}
                      className={cn(
                        "px-6 py-2 rounded-lg text-xs font-bold tracking-widest uppercase transition-all",
                        isFetchingSubscription ? "bg-amber-500/20 text-amber-400 animate-pulse" : "bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-400 border border-cyan-500/30"
                      )}
                    >
                      {isFetchingSubscription ? 'Fetching...' : 'Fetch'}
                    </button>
                  </div>

                  <div className="flex-1 overflow-y-auto bg-zinc-900/60 border border-white/10 rounded-xl p-4">
                    {xrayNodes.length === 0 ? (
                      <div className="h-full flex items-center justify-center text-zinc-600 text-sm">
                        No nodes loaded. Paste a subscription link above.
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {xrayNodes.map((node: any) => (
                          <button
                            key={node.id}
                            onClick={() => setSelectedXrayNode(node.id)}
                            className={cn(
                              "w-full text-left px-4 py-3 rounded-lg transition-all border",
                              selectedXrayNode === node.id 
                                ? "bg-cyan-500/20 border-cyan-500/50 text-cyan-300" 
                                : "bg-zinc-800/50 border-white/5 text-zinc-300 hover:bg-zinc-800 hover:border-white/10"
                            )}
                          >
                            <div className="font-bold text-sm">{node.name}</div>
                            <div className="text-xs text-zinc-500 mt-1">{node.address}:{node.port}</div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </>
              )}

              {/* AmneziaWG Config Import */}
              {selectedProfile.includes('AmneziaWG') && (
                <div className="flex-1 flex flex-col gap-3">
                  <div className="text-sm text-zinc-400">
                    Paste your AmneziaWG .conf file content below:
                  </div>
                  <textarea
                    value={scriptContent}
                    onChange={(e) => setScriptContent(e.target.value)}
                    className="flex-1 w-full bg-zinc-900/80 border border-white/10 rounded-xl p-4 text-emerald-400 font-mono text-sm leading-relaxed resize-none focus:outline-none focus:border-cyan-500/50 focus:ring-2 focus:ring-cyan-500/20 transition-all scrollbar-thin scrollbar-thumb-zinc-700 scrollbar-track-zinc-900"
                    placeholder="[Interface]&#10;PrivateKey = ...&#10;Address = ...&#10;&#10;[Peer]&#10;PublicKey = ...&#10;Endpoint = ..."
                    spellCheck={false}
                  />
                </div>
              )}

              {/* Lua Editor (Default) */}
              {!selectedProfile.includes('Xray') && !selectedProfile.includes('VLESS') && !selectedProfile.includes('AmneziaWG') && (
                <textarea
                  value={scriptContent}
                  onChange={(e) => setScriptContent(e.target.value)}
                  className="flex-1 w-full bg-zinc-900/80 border border-white/10 rounded-xl p-4 text-emerald-400 font-mono text-sm leading-relaxed resize-none focus:outline-none focus:border-cyan-500/50 focus:ring-2 focus:ring-cyan-500/20 transition-all scrollbar-thin scrollbar-thumb-zinc-700 scrollbar-track-zinc-900"
                  placeholder="-- Enter your custom Zapret Lua bypass strategy here..."
                  spellCheck={false}
                />
              )}
            </div>

            {/* Modal Footer */}
            <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-white/10 bg-zinc-950/50 backdrop-blur-xl">
              <button
                onClick={() => setIsEditorOpen(false)}
                className="px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-zinc-400 hover:text-white text-xs font-bold tracking-widest uppercase transition-all"
              >
                Cancel
              </button>
              <button
                onClick={async () => {
                  if (selectedProfile.includes('Xray') || selectedProfile.includes('VLESS')) {
                    if (!selectedXrayNode) {
                      alert('Please select a node first');
                      return;
                    }
                    try {
                      await GenerateXrayConfig(selectedXrayNode);
                      setIsEditorOpen(false);
                    } catch (err) {
                      console.error('Failed to generate Xray config:', err);
                    }
                  } else {
                    await handleSaveScript();
                  }
                }}
                disabled={(selectedProfile.includes('Xray') || selectedProfile.includes('VLESS')) && !selectedXrayNode}
                className="px-4 py-2 rounded-lg bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-400 hover:text-cyan-300 border border-cyan-500/30 text-xs font-bold tracking-widest uppercase transition-all shadow-[0_0_15px_rgba(6,182,212,0.2)] disabled:opacity-30 disabled:cursor-not-allowed"
              >
                {(selectedProfile.includes('Xray') || selectedProfile.includes('VLESS')) ? 'Apply Selected Node' : 'Save & Apply'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Settings Modal */}
      {isSettingsOpen && (
        <div 
          className="absolute inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-md"
          onClick={() => setIsSettingsOpen(false)}
        >
          <div 
            className="w-[90%] max-w-2xl bg-zinc-900/95 border border-white/10 rounded-2xl shadow-2xl flex flex-col overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-white/10 bg-zinc-950/50 backdrop-blur-xl">
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-cyan-500/10 flex items-center justify-center border border-cyan-500/30">
                  <Settings className="w-4 h-4 text-cyan-400" />
                </div>
                <div>
                  <h2 className="text-sm font-bold tracking-widest text-white/90">SETTINGS</h2>
                  <p className="text-[9px] text-zinc-500 font-bold tracking-widest uppercase">Application Configuration</p>
                </div>
              </div>
            </div>

            {/* Settings Content */}
            <div className="p-6 space-y-6">
              
              {/* Auto Start Toggle */}
              <div className="flex items-center justify-between p-4 bg-zinc-900/60 border border-white/10 rounded-xl hover:border-cyan-500/30 transition-all">
                <div className="flex-1">
                  <h3 className="text-sm font-bold text-white/90 mb-1">Launch on System Startup</h3>
                  <p className="text-xs text-zinc-500">Automatically start Unbound when Windows boots</p>
                </div>
                <button
                  onClick={() => setSettings({...settings, autoStart: !settings.autoStart})}
                  className={cn(
                    "relative w-12 h-6 rounded-full transition-all duration-300 shrink-0",
                    settings.autoStart ? "bg-cyan-500" : "bg-zinc-700"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full transition-all duration-300 shadow-lg",
                    settings.autoStart ? "left-7" : "left-1"
                  )} />
                </button>
              </div>

              {/* Start Minimized Toggle */}
              <div className="flex items-center justify-between p-4 bg-zinc-900/60 border border-white/10 rounded-xl hover:border-cyan-500/30 transition-all">
                <div className="flex-1">
                  <h3 className="text-sm font-bold text-white/90 mb-1">Start Minimized to Tray</h3>
                  <p className="text-xs text-zinc-500">Launch directly to system tray without showing window</p>
                </div>
                <button
                  onClick={() => setSettings({...settings, startMinimized: !settings.startMinimized})}
                  className={cn(
                    "relative w-12 h-6 rounded-full transition-all duration-300 shrink-0",
                    settings.startMinimized ? "bg-cyan-500" : "bg-zinc-700"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full transition-all duration-300 shadow-lg",
                    settings.startMinimized ? "left-7" : "left-1"
                  )} />
                </button>
              </div>

              {/* Startup Profile Mode */}
              <div className="p-4 bg-zinc-900/60 border border-white/10 rounded-xl hover:border-cyan-500/30 transition-all">
                <h3 className="text-sm font-bold text-white/90 mb-3">Startup Profile</h3>
                <div className="relative">
                  <select 
                    value={settings.startupProfileMode} 
                    onChange={(e) => setSettings({...settings, startupProfileMode: e.target.value})}
                    className="w-full bg-zinc-800/50 border border-white/10 rounded-lg px-3 py-2 text-zinc-100 text-sm font-semibold tracking-wide outline-none cursor-pointer hover:border-cyan-500/50 focus:border-cyan-500 focus:ring-2 focus:ring-cyan-500/20 transition-all appearance-none pr-8"
                  >
                    <option value="Last Used" className="bg-zinc-900">Last Used Profile</option>
                    <option value="Auto-Tune" className="bg-zinc-900">Auto-Tune on Startup</option>
                    {profiles.map(p => <option key={p} value={p} className="bg-zinc-900">{p}</option>)}
                  </select>
                  <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-500 pointer-events-none" />
                </div>
                <p className="text-xs text-zinc-500 mt-2">Choose which profile to use when Unbound starts</p>
              </div>

              {/* Game Filter Toggle */}
              <div className="flex items-center justify-between p-4 bg-zinc-900/60 border border-white/10 rounded-xl hover:border-cyan-500/30 transition-all">
                <div className="flex-1">
                  <h3 className="text-sm font-bold text-white/90 mb-1">Enable Game Filter</h3>
                  <p className="text-xs text-zinc-500">Exclude Steam/Riot/Epic UDP ports to prevent game lag</p>
                </div>
                <button
                  onClick={() => setSettings({...settings, gameFilter: !settings.gameFilter})}
                  className={cn(
                    "relative w-12 h-6 rounded-full transition-all duration-300 shrink-0",
                    settings.gameFilter ? "bg-cyan-500" : "bg-zinc-700"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full transition-all duration-300 shadow-lg",
                    settings.gameFilter ? "left-7" : "left-1"
                  )} />
                </button>
              </div>

              {/* Auto-Update Toggle */}
              <div className="flex items-center justify-between p-4 bg-zinc-900/60 border border-white/10 rounded-xl hover:border-cyan-500/30 transition-all">
                <div className="flex-1">
                  <h3 className="text-sm font-bold text-white/90 mb-1">Enable Auto-Update Checks</h3>
                  <p className="text-xs text-zinc-500">Automatically check for new releases on startup</p>
                </div>
                <button
                  onClick={() => setSettings({...settings, autoUpdateEnabled: !settings.autoUpdateEnabled})}
                  className={cn(
                    "relative w-12 h-6 rounded-full transition-all duration-300 shrink-0",
                    settings.autoUpdateEnabled ? "bg-cyan-500" : "bg-zinc-700"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full transition-all duration-300 shadow-lg",
                    settings.autoUpdateEnabled ? "left-7" : "left-1"
                  )} />
                </button>
              </div>

            </div>

            {/* Modal Footer */}
            <div className="flex items-center justify-end gap-3 px-6 py-4 border-t border-white/10 bg-zinc-950/50 backdrop-blur-xl">
              <button
                onClick={() => setIsSettingsOpen(false)}
                className="px-4 py-2 rounded-lg bg-white/5 hover:bg-white/10 text-zinc-400 hover:text-white text-xs font-bold tracking-widest uppercase transition-all"
              >
                Cancel
              </button>
              <button
                onClick={handleSaveSettings}
                className="px-4 py-2 rounded-lg bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-400 hover:text-cyan-300 border border-cyan-500/30 text-xs font-bold tracking-widest uppercase transition-all shadow-[0_0_15px_rgba(6,182,212,0.2)]"
              >
                Save Settings
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Update Notification Toast */}
      {updateNotification.show && (
        <div className="absolute top-20 right-6 z-50 w-96 bg-zinc-900/95 border border-cyan-500/30 rounded-xl shadow-2xl backdrop-blur-xl overflow-hidden animate-in slide-in-from-right">
          <div className="p-4 border-b border-white/10 bg-gradient-to-r from-cyan-500/10 to-transparent">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-lg bg-cyan-500/20 flex items-center justify-center border border-cyan-500/30">
                  <Shield className="w-4 h-4 text-cyan-400" />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-white/90">Update Available</h3>
                  <p className="text-xs text-cyan-400 font-bold">{updateNotification.version}</p>
                </div>
              </div>
              <button
                onClick={() => setUpdateNotification({...updateNotification, show: false})}
                className="text-zinc-500 hover:text-white transition-colors"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>
          <div className="p-4">
            <p className="text-xs text-zinc-400 mb-3 line-clamp-3">{updateNotification.changelog || 'New version available with improvements and bug fixes.'}</p>
            <button
              onClick={() => {
                window.open(updateNotification.url, '_blank');
                setUpdateNotification({...updateNotification, show: false});
              }}
              className="w-full px-4 py-2 rounded-lg bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-400 hover:text-cyan-300 border border-cyan-500/30 text-xs font-bold tracking-widest uppercase transition-all shadow-[0_0_15px_rgba(6,182,212,0.2)]"
            >
              Download Update
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
