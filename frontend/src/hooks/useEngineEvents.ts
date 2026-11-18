import { useEffect, useRef } from 'react';
import { AutotuneProgressData, eventBus, windowService } from '../services/window';
import { backendService } from '../services/backend';

export interface UseEngineEventsProps {
  onStatusChange: (newStatus: string) => void;
  onEnginesChange: (engines: string[]) => void;
  onPrivilegeError: (msg: string) => void;
  onAddToast: (toast: { id: number; type: string; title: string; message: string }) => void;
  onAutotuneStart: (running: boolean) => void;
  onAutotuneProgress: (data: AutotuneProgressData) => void;
  onAutotuneLog: (msg: string) => void;
  onEngineLog: (msg: string) => void;
  onAutotuneComplete: (data: { success: boolean; profile?: string; error?: string }) => void;
  onProfileChanged?: (profile: string) => void;
  onProfileError?: (msg: string) => void;
}

export function useEngineEvents(props: UseEngineEventsProps) {
  const handlersRef = useRef<UseEngineEventsProps>(props);
  handlersRef.current = props;

  useEffect(() => {
    const unsubs: Array<(() => void) | undefined> = [
      eventBus.onStatusChanged((newStatus: string) => {
        handlersRef.current.onStatusChange(newStatus);
      }),

      eventBus.onEnginesChanged((e: string[]) => {
        if (e && e.length > 0) {
          handlersRef.current.onEnginesChange(e);
          backendService.getProfiles(e[0]).catch(() => {});
        }
      }),

      eventBus.onPrivilegeError((msg: string) => {
        handlersRef.current.onPrivilegeError(msg);
      }),

      eventBus.onEngineError((msg: string) => {
        windowService.showNotification('Ошибка движка', msg);
        handlersRef.current.onStatusChange('Stopped');
      }),

      eventBus.onNotification((data) => {
        handlersRef.current.onAddToast({
          id: Date.now() + Math.random(),
          type: data?.type || 'info',
          title: data?.title || 'Уведомление',
          message: data?.message || '',
        });
      }),

      eventBus.onAutotuneStart((running: boolean) => {
        handlersRef.current.onAutotuneStart(running);
      }),

      eventBus.onAutotuneProgress((data) => {
        handlersRef.current.onAutotuneProgress(data);
      }),

      eventBus.onAutotuneLog((msg: string) => {
        handlersRef.current.onAutotuneLog(msg);
      }),

      eventBus.onEngineLog((msg: string) => {
        handlersRef.current.onEngineLog(msg);
      }),

      eventBus.onAutotuneComplete((data) => {
        handlersRef.current.onAutotuneComplete(data);
      }),

      typeof eventBus.onProfileChanged === 'function'
        ? eventBus.onProfileChanged((profile: string) => {
            handlersRef.current.onProfileChanged?.(profile);
          })
        : undefined,

      typeof eventBus.onProfileError === 'function'
        ? eventBus.onProfileError((msg: string) => {
            handlersRef.current.onProfileError?.(msg);
          })
        : undefined,
    ];

    return () => {
      unsubs.forEach((unsub) => {
        if (typeof unsub === 'function') unsub();
      });
    };
  }, []);
}
