import React, { useState, useEffect, useRef } from 'react';
import { GetEngineNames, GetProfiles, StartEngine, StopEngine, GetLogs, TestProfile, AutoSelectProfile, HideToTray } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { motion, AnimatePresence } from 'framer-motion';
import { Play, Square, Zap, Settings, Terminal, TrendingUp, Minimize2, Activity } from 'lucide-react';

export default function App() {
  const [engines, setEngines] = useState<string[]>([]);
  const [selectedEngine, setSelectedEngine] = useState<string>('');
  const [profiles, setProfiles] = useState<string[]>([]);
  const [selectedProfile, setSelectedProfile] = useState<string>('');
  const [status, setStatus] = useState<string>('Stopped');
  const [logs, setLogs] = useState<string[]>([]);
  const [testing, setTesting] = useState(false);
  const [autoSelecting, setAutoSelecting] = useState(false);
  const [testResult, setTestResult] = useState<string>('');
  const [showTestModal, setShowTestModal] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    GetEngineNames().then(engines => {
      setEngines(engines);
      if (engines.length > 0) setSelectedEngine(engines[0]);
    });
    
    EventsOn('status_changed', (newStatus: string) => {
      setStatus(newStatus);
    });
    
    const interval = setInterval(() => {
      GetLogs().then(setLogs);
    }, 500);
    
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    if (selectedEngine) {
      GetProfiles(selectedEngine).then(p => {
        setProfiles(p);
        if (p.length > 0) setSelectedProfile(p[0]);
      });
    }
  }, [selectedEngine]);

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  const handleStart = async () => {
    try {
      await StartEngine(selectedEngine, selectedProfile);
    } catch (err: any) {
      alert('Error: ' + err);
    }
  };

  const handleStop = async () => {
    await StopEngine();
  };

  const handleTest = async () => {
    setTesting(true);
    setTestResult('');
    setShowTestModal(true);
    try {
      const result = await TestProfile(selectedEngine, selectedProfile);
      setTestResult(result);
    } catch (err: any) {
      setTestResult('Error: ' + err);
    }
    setTesting(false);
  };

  const handleAutoSelect = async () => {
    setAutoSelecting(true);
    setTestResult('Testing all profiles...');
    setShowTestModal(true);
    try {
      const best = await AutoSelectProfile(selectedEngine);
      setSelectedProfile(best);
      setTestResult(`✓ Best profile selected: ${best}`);
    } catch (err: any) {
      setTestResult('Error: ' + err);
    }
    setAutoSelecting(false);
  };

  const isRunning = status === 'Running';
  const isStarting = status === 'Starting';

  return (
    <div className="h-screen bg-gradient-to-br from-gray-900 via-black to-gray-900 flex flex-col select-none">
      <div className="bg-black/80 backdrop-blur-xl border-b border-gray-800 px-4 py-2 flex items-center justify-between" style={{ WebkitAppRegion: 'drag' } as any}>
        <div className="flex items-center gap-2">
          <Zap className="w-5 h-5 text-blue-500" />
          <span className="text-sm font-bold bg-gradient-to-r from-blue-400 to-purple-500 bg-clip-text text-transparent">UNBOUND</span>
        </div>
        <div className="flex items-center gap-1" style={{ WebkitAppRegion: 'no-drag' } as any}>
          <button onClick={() => HideToTray()} className="p-1.5 hover:bg-gray-800 rounded transition-colors">
            <Minimize2 className="w-4 h-4 text-gray-400" />
          </button>
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        <div className="w-80 bg-black/40 backdrop-blur-xl border-r border-gray-800 flex flex-col">
          <div className="p-4 border-b border-gray-800">
            <div className="flex items-center justify-between mb-3">
              <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">Status</span>
              <div className="flex items-center gap-2">
                {isRunning && <Activity className="w-4 h-4 text-green-400 animate-pulse" />}
                <span className={`text-sm font-semibold ${isRunning ? 'text-green-400' : isStarting ? 'text-yellow-400' : 'text-gray-500'}`}>{status}</span>
              </div>
            </div>
          </div>

          <div className="flex-1 overflow-auto p-4 space-y-4">
            <div>
              <label className="block text-xs font-medium text-gray-400 mb-2 uppercase tracking-wider">Engine</label>
              <select value={selectedEngine} onChange={(e) => setSelectedEngine(e.target.value)} disabled={isRunning} className="w-full bg-gray-800/70 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all">
                {engines.map(e => <option key={e} value={e}>{e}</option>)}
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-400 mb-2 uppercase tracking-wider">Profile</label>
              <select value={selectedProfile} onChange={(e) => setSelectedProfile(e.target.value)} disabled={isRunning || !selectedEngine} className="w-full bg-gray-800/70 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all">
                {profiles.map(p => <option key={p} value={p}>{p}</option>)}
              </select>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <button onClick={handleAutoSelect} disabled={!selectedEngine || isRunning || autoSelecting} className="bg-purple-600/90 hover:bg-purple-600 disabled:bg-gray-700/50 disabled:opacity-50 disabled:cursor-not-allowed text-white px-3 py-2 rounded-lg text-xs font-medium transition-all flex items-center justify-center gap-1.5">
                <TrendingUp className="w-3.5 h-3.5" />
                {autoSelecting ? 'Testing...' : 'Auto'}
              </button>
              <button onClick={handleTest} disabled={!selectedEngine || !selectedProfile || testing} className="bg-blue-600/90 hover:bg-blue-600 disabled:bg-gray-700/50 disabled:opacity-50 disabled:cursor-not-allowed text-white px-3 py-2 rounded-lg text-xs font-medium transition-all flex items-center justify-center gap-1.5">
                <Settings className="w-3.5 h-3.5" />
                {testing ? 'Testing...' : 'Test'}
              </button>
            </div>
          </div>

          <div className="p-4 border-t border-gray-800">
            {!isRunning ? (
              <button onClick={handleStart} disabled={!selectedEngine || !selectedProfile || isStarting} className="w-full bg-gradient-to-r from-green-600 to-green-500 hover:from-green-500 hover:to-green-400 disabled:from-gray-700 disabled:to-gray-700 disabled:opacity-50 disabled:cursor-not-allowed text-white px-4 py-3 rounded-lg font-semibold transition-all flex items-center justify-center gap-2 shadow-lg">
                <Play className="w-5 h-5" />
                {isStarting ? 'Starting...' : 'Start Engine'}
              </button>
            ) : (
              <button onClick={handleStop} className="w-full bg-gradient-to-r from-red-600 to-red-500 hover:from-red-500 hover:to-red-400 text-white px-4 py-3 rounded-lg font-semibold transition-all flex items-center justify-center gap-2 shadow-lg">
                <Square className="w-5 h-5" />
                Stop Engine
              </button>
            )}
          </div>
        </div>

        <div className="flex-1 flex flex-col bg-black/20">
          <div className="bg-black/40 backdrop-blur-xl border-b border-gray-800 px-4 py-2.5 flex items-center gap-2">
            <Terminal className="w-4 h-4 text-gray-400" />
            <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">Engine Logs</span>
            <div className="flex-1" />
            <span className="text-xs text-gray-600">{logs.length} lines</span>
          </div>
          <div className="flex-1 overflow-auto p-4">
            <div className="space-y-0.5 font-mono text-xs">
              {logs.length === 0 ? (
                <div className="text-gray-600 text-center py-8">No logs yet. Start an engine to see output.</div>
              ) : (
                logs.map((log, i) => {
                  const isError = log.includes('Error') || log.includes('STDERR') || log.includes('failed');
                  const isSuccess = log.includes('ACTIVE') || log.includes('SUCCESS') || log.includes('✓');
                  const isWarning = log.includes('Warning') || log.includes('warning');
                  
                  return (
                    <div key={i} className={`px-2 py-0.5 rounded ${isError ? 'text-red-400 bg-red-950/20' : isSuccess ? 'text-green-400 bg-green-950/20' : isWarning ? 'text-yellow-400 bg-yellow-950/20' : 'text-gray-400 hover:bg-gray-800/30'}`}>
                      {log}
                    </div>
                  );
                })
              )}
              <div ref={logsEndRef} />
            </div>
          </div>
        </div>
      </div>

      <AnimatePresence>
        {showTestModal && (
          <motion.div initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowTestModal(false)}>
            <motion.div initial={{ scale: 0.9, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.9, opacity: 0 }} onClick={(e) => e.stopPropagation()} className="bg-gray-900 border border-gray-800 rounded-xl p-6 max-w-2xl w-full max-h-[80vh] overflow-auto shadow-2xl">
              <h3 className="text-lg font-bold text-white mb-4">Test Results</h3>
              {testing || autoSelecting ? (
                <div className="flex flex-col items-center justify-center py-12">
                  <div className="w-12 h-12 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4" />
                  <p className="text-gray-400">Testing profile performance...</p>
                </div>
              ) : (
                <pre className="bg-black/50 border border-gray-800 rounded-lg p-4 text-sm text-gray-300 whitespace-pre-wrap font-mono">{testResult || 'No results yet'}</pre>
              )}
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
