import { useState, useEffect, useCallback } from 'react';
import { backendService } from '../services/backend';
import { windowService } from '../services/window';

export function useLogJournal(status: string, isScanning: boolean) {
  const [logs, setLogs] = useState<string[]>([]);
  const [scanLogs, setScanLogs] = useState<string[]>([]);
  const [isExpanded, setIsExpanded] = useState<boolean>(false);

  useEffect(() => {
    if (!isScanning && status === 'Running') {
      const interval = setInterval(() => {
        backendService.getLogs().then((l: string[]) => setLogs(l || [])).catch(() => {});
      }, 2000);
      return () => clearInterval(interval);
    }
  }, [isScanning, status]);

  const addScanLog = useCallback((msg: string) => {
    setScanLogs((prev) => [...prev, msg]);
  }, []);

  const addEngineLog = useCallback((msg: string) => {
    setLogs((prev) => [...prev, msg]);
  }, []);

  const clearScanLogs = useCallback(() => {
    setScanLogs([]);
  }, []);

  const exportLogs = useCallback(async () => {
    const activeLogs = isScanning ? scanLogs : logs;
    try {
      const saved = await backendService.exportLogs(activeLogs.join('\n'));
      if (saved) {
        windowService.showNotification('Успех', 'Лог успешно сохранен.');
      }
    } catch (err) {
      console.error('Failed to export logs:', err);
    }
  }, [isScanning, scanLogs, logs]);

  const displayLogs = isScanning ? scanLogs : logs;

  return {
    logs,
    scanLogs,
    displayLogs,
    isExpanded,
    setIsExpanded,
    addScanLog,
    addEngineLog,
    clearScanLogs,
    exportLogs,
  };
}
