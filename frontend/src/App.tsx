import { useState, useEffect, useRef } from 'react';
import { Power, Terminal, Shield, ShieldAlert, Minimize2, ChevronUp, ChevronDown, Radar, Globe, Code } from 'lucide-react';
import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// @ts-ignore
import { GetEngineNames, GetProfiles, StartEngine, StopEngine, GetLogs, HideToTray, AutoTune, SaveCustomScript, LoadCustomScript, GetCurrentPing } from '../wailsjs/go/main/App';
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
  const [isEditorOpen, setIsEditorOpen] = useState<boolean>(false);
  const [scriptContent, setScriptContent] = useState<string>('');
  const [pingData, setPingData] = useState<{active: boolean, latency: number, status: string, certValid?: boolean}>({active: false, latency: 0, status: 'stopped'});
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    GetEngineNames().then((engines: string[]) => {
      setEngines(engines || []);
      if (engines && engines.length > 0) setSelectedEngine(engines[0]);
    });
    
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
      if (status === 'Running') {
        GetCurrentPing().then((data: any) => {
          setPingData(data || {active: false, latency: 0, status: 'stopped'});
        }).catch(() => {
          setPingData({active: false, latency: 0, status: 'error'});
        });
      } else {
        setPingData({active: false, latency: 0, status: 'stopped'});
      }
    }, 5000);
    
    return () => {
      clearInterval(interval);
      clearInterval(pingInterval);
    };
  }, [isScanning, status]);

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

  const handleOpenEditor = async () => {
    setIsEditorOpen(true);
    try {
      const content = await LoadCustomScript();
      setScriptContent(content);
    } catch (err) {
      console.error('Failed to load custom script:', err);
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

  const isConnected = status === 'Running';
  const isConnecting = status === 'Starting';
  const disableMain = isConnecting || isScanning;

  const displayLogs = isScanning ? scanLogs : logs;

  return (
    <div className="h-screen w-screen bg-zinc-950 flex flex-col font-sans text-zinc-100 overflow-hidden relative selection:bg-cyan-500/30">
      
      {/* Background ambient glow based on status */}
      <div className={cn(
        "absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[400px] h-[400px] rounded-full blur-[100px] opacity-10 pointer-events-none transition-all duration-1000",
        isConnected ? "bg-emerald-500/30" : isScanning ? "bg-amber-500/15" : "bg-zinc-800/30"
      )} />

      {/* Header */}
      <div 
        className="h-16 flex items-center justify-between px-6 border-b border-white/5 bg-zinc-950/40 backdrop-blur-xl z-10 shrink-0"
        style={{ WebkitAppRegion: 'drag' } as any}
      >
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-emerald-500 to-emerald-600 flex items-center justify-center border border-emerald-400/30 shadow-lg relative overflow-hidden">
            {/* Realistic Cannabis leaf icon */}
            <svg viewBox="0 0 32 32" className="w-5 h-5 text-white" fill="currentColor">
              {/* Center main leaf */}
              <path d="M16 4 C15.5 4 15 6 15 8 C15 10 15 12 15 14 L15 20 C15 20 15.5 18 16 18 C16.5 18 17 20 17 20 L17 14 C17 12 17 10 17 8 C17 6 16.5 4 16 4 Z" opacity="0.95"/>
              
              {/* Left side leaves */}
              <path d="M15 10 C15 10 13 9.5 11 10 C9 10.5 7 11.5 6 13 C6 13 7.5 13.5 9 13.5 C10.5 13.5 12 13 13 12.5 C14 12 15 11 15 10 Z" opacity="0.9"/>
              <path d="M15 12 C15 12 12.5 12 10.5 12.5 C8.5 13 6.5 14 5.5 15.5 C5.5 15.5 7 15.5 8.5 15.5 C10 15.5 11.5 15 12.5 14.5 C13.5 14 15 13 15 12 Z" opacity="0.85"/>
              <path d="M15 14 C15 14 12 14.5 10 15.5 C8 16.5 6.5 17.5 6 19 C6 19 7.5 18.5 9 18 C10.5 17.5 12 17 13 16.5 C14 16 15 15 15 14 Z" opacity="0.8"/>
              
              {/* Right side leaves */}
              <path d="M17 10 C17 10 19 9.5 21 10 C23 10.5 25 11.5 26 13 C26 13 24.5 13.5 23 13.5 C21.5 13.5 20 13 19 12.5 C18 12 17 11 17 10 Z" opacity="0.9"/>
              <path d="M17 12 C17 12 19.5 12 21.5 12.5 C23.5 13 25.5 14 26.5 15.5 C26.5 15.5 25 15.5 23.5 15.5 C22 15.5 20.5 15 19.5 14.5 C18.5 14 17 13 17 12 Z" opacity="0.85"/>
              <path d="M17 14 C17 14 20 14.5 22 15.5 C24 16.5 25.5 17.5 26 19 C26 19 24.5 18.5 23 18 C21.5 17.5 20 17 19 16.5 C18 16 17 15 17 14 Z" opacity="0.8"/>
              
              {/* Bottom small leaves */}
              <path d="M15 16 C15 16 13.5 17 12 18.5 C10.5 20 9.5 21.5 9 23 C9 23 10 22 11.5 21 C13 20 14 19 14.5 18 C15 17 15 16 15 16 Z" opacity="0.75"/>
              <path d="M17 16 C17 16 18.5 17 20 18.5 C21.5 20 22.5 21.5 23 23 C23 23 22 22 20.5 21 C19 20 18 19 17.5 18 C17 17 17 16 17 16 Z" opacity="0.75"/>
            </svg>
            {/* Z V letters overlay */}
            <div className="absolute bottom-0 right-0 text-[6px] font-black text-cyan-400 leading-none pr-0.5 pb-0.5 drop-shadow-[0_0_2px_rgba(6,182,212,0.8)]">
              ZV
            </div>
          </div>
          <div>
            <h1 className="text-sm font-bold tracking-widest text-white/90">UNBOUND</h1>
            <p className="text-[9px] text-zinc-500 font-bold tracking-widest uppercase">CLEARFLOW ENGINE</p>
          </div>
        </div>

        <div className="flex items-center gap-4" style={{ WebkitAppRegion: 'no-drag' } as any}>
          {/* Live Ping Indicator */}
          {pingData.active && pingData.status === 'ok' && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-bold tracking-widest border bg-cyan-500/10 text-cyan-400 border-cyan-500/30 shadow-[0_0_10px_rgba(6,182,212,0.2)]">
              <div className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse shadow-[0_0_8px_rgba(6,182,212,0.8)]" />
              {pingData.latency}ms
            </div>
          )}

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

          <button onClick={() => HideToTray()} className="p-1.5 rounded-lg hover:bg-white/10 transition-colors text-zinc-400 hover:text-white">
            <Minimize2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Main Hero Section */}
      <div className="flex-1 flex flex-col items-center justify-center gap-10 relative min-h-0 z-10 p-4">
        
        {/* Massive Button Container */}
        <div className="relative flex items-center justify-center shrink-0">
          
          {/* Ambient glow around button */}
          <div className={cn(
            "absolute w-[200px] h-[200px] rounded-full blur-[60px] opacity-0 pointer-events-none transition-all duration-700",
            isConnected ? "opacity-30 bg-emerald-500/50" : "opacity-0"
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

        {/* Profile Card */}
        <div 
          className="flex flex-row items-stretch justify-between w-full max-w-[400px] bg-white/5 border border-white/10 rounded-2xl p-4 backdrop-blur-xl shadow-xl transition-all duration-500 gap-4"
          style={{ WebkitAppRegion: 'no-drag' } as any}
        >
          <div className="flex flex-col overflow-hidden flex-1 justify-center group">
            <span className="text-[10px] text-zinc-400 font-bold tracking-widest uppercase mb-1 flex items-center gap-2 transition-colors duration-300 group-hover:text-cyan-400">
              <Shield className="w-3 h-3 text-cyan-500 transition-transform duration-300 group-hover:scale-110 group-hover:rotate-12" />
              Active Strategy
            </span>
            <div className="relative w-full overflow-visible">
              <select 
                value={selectedProfile} 
                onChange={(e) => setSelectedProfile(e.target.value)} 
                disabled={isConnected || disableMain || !selectedEngine}
                className="bg-transparent border-none text-zinc-100 text-sm font-semibold tracking-wide outline-none appearance-none cursor-pointer disabled:opacity-70 w-full hover:text-cyan-100 transition-all duration-300 pr-6 hover:translate-x-1"
                style={{
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  maxWidth: '100%'
                }}
              >
                {profiles.map(p => <option key={p} value={p} className="bg-zinc-900 text-sm">{p}</option>)}
              </select>
              {/* Dropdown indicator with animation */}
              <div className="absolute right-0 top-1/2 -translate-y-1/2 pointer-events-none opacity-50 transition-all duration-300 group-hover:opacity-100 group-hover:translate-y-[-40%]">
                <ChevronDown className="w-4 h-4 group-hover:text-cyan-400 transition-colors" />
              </div>
            </div>
          </div>

          <div className="w-px bg-white/10 shrink-0 self-stretch my-1" />

          <button
            onClick={handleAutoTune}
            disabled={isConnected || isScanning}
            className={cn(
              "flex flex-col items-center justify-center p-3 rounded-xl transition-all duration-300 min-w-[80px] shrink-0",
              isScanning 
                ? "bg-amber-500/20 text-amber-400 cursor-wait shadow-[0_0_15px_rgba(245,158,11,0.2)]" 
                : isConnected
                  ? "opacity-30 cursor-not-allowed text-zinc-500"
                  : "bg-white/5 hover:bg-white/10 text-cyan-400 hover:text-cyan-300 hover:shadow-[0_0_15px_rgba(6,182,212,0.2)]"
            )}
          >
            <Radar className={cn("w-5 h-5 mb-1.5", isScanning ? "animate-spin" : "")} />
            <span className="text-[9px] font-bold tracking-widest uppercase text-center">Auto-Tune</span>
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

      {/* Advanced Lua Editor Modal */}
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
                  <h2 className="text-sm font-bold tracking-widest text-white/90">ADVANCED LUA EDITOR</h2>
                  <p className="text-[9px] text-zinc-500 font-bold tracking-widest uppercase">Custom DPI Bypass Strategy</p>
                </div>
              </div>
            </div>

            {/* Editor Area */}
            <div className="flex-1 p-6 overflow-hidden flex flex-col">
              <textarea
                value={scriptContent}
                onChange={(e) => setScriptContent(e.target.value)}
                className="flex-1 w-full bg-zinc-900/80 border border-white/10 rounded-xl p-4 text-emerald-400 font-mono text-sm leading-relaxed resize-none focus:outline-none focus:border-cyan-500/50 focus:ring-2 focus:ring-cyan-500/20 transition-all scrollbar-thin scrollbar-thumb-zinc-700 scrollbar-track-zinc-900"
                placeholder="-- Enter your custom Zapret Lua bypass strategy here..."
                spellCheck={false}
              />
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
                onClick={handleSaveScript}
                className="px-4 py-2 rounded-lg bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-400 hover:text-cyan-300 border border-cyan-500/30 text-xs font-bold tracking-widest uppercase transition-all shadow-[0_0_15px_rgba(6,182,212,0.2)]"
              >
                Save & Apply
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
