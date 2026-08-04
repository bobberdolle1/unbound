import { useEffect } from 'react';
import { eventBus, windowService } from '../services/window';
import { backendService } from '../services/backend';

interface UseEngineEventsProps {
  onStatusChange: (newStatus: string) => void;
  onEnginesChange: (engines: string[]) => void;
  onPrivilegeError: (msg: string) => void;
  onAddToast: (toast: { id: number; type: string; title: string; message: string }) => void;
  onAutotuneStart: (running: boolean) => void;
  onAutotuneProgress: (data: { msg?: string; progress?: number }) => void;
  onAutotuneLog: (msg: string) => void;
  onEngineLog: (msg: string) => void;
  onAutotuneComplete: (data: { success: boolean; profile?: string; error?: string }) => void;
}

export function useEngineEvents({
  onStatusChange,
  onEnginesChange,
  onPrivilegeError,
  onAddToast,
  onAutotuneStart,
  onAutotuneProgress,
  onAutotuneLog,
  onEngineLog,
  onAutotuneComplete,
}: UseEngineEventsProps) {
  useEffect(() => {
    const unsubs = [
      eventBus.onStatusChanged((newStatus: string) => {
        onStatusChange(newStatus);
      }),

      eventBus.onEnginesChanged((e: string[]) => {
        if (e && e.length > 0) {
          onEnginesChange(e);
          backendService.getProfiles(e[0]).catch(() => {});
        }
      }),

      eventBus.onPrivilegeError((msg: string) => {
        onPrivilegeError(msg);
      }),

      eventBus.onEngineError((msg: string) => {
        windowService.showNotification('Ошибка движка', msg);
        onStatusChange('Stopped');
      }),

      eventBus.onNotification((data) => {
        onAddToast({
          id: Date.now() + Math.random(),
          type: data?.type || 'info',
          title: data?.title || 'Уведомление',
          message: data?.message || '',
        });
      }),

      eventBus.onAutotuneStart((running: boolean) => {
        onAutotuneStart(running);
      }),

      eventBus.onAutotuneProgress((data) => {
        onAutotuneProgress(data);
      }),

      eventBus.onAutotuneLog((msg: string) => {
        onAutotuneLog(msg);
      }),

      eventBus.onEngineLog((msg: string) => {
        onEngineLog(msg);
      }),

      eventBus.onAutotuneComplete((data) => {
        onAutotuneComplete(data);
      }),
    ];

    return () => {
      unsubs.forEach((unsub) => {
        if (typeof unsub === 'function') unsub();
      });
    };
  }, [
    onStatusChange,
    onEnginesChange,
    onPrivilegeError,
    onAddToast,
    onAutotuneStart,
    onAutotuneProgress,
    onAutotuneLog,
    onEngineLog,
    onAutotuneComplete,
  ]);
}
