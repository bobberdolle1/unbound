import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useEngineEvents } from '../hooks/useEngineEvents';
import { usePingPolling } from '../hooks/usePingPolling';
import { useLogJournal } from '../hooks/useLogJournal';
import { eventBus } from '../services/window';
import { backendService } from '../services/backend';

vi.mock('../services/window', () => {
  const unsubFns = {
    onStatusChanged: vi.fn(),
    onEnginesChanged: vi.fn(),
    onPrivilegeError: vi.fn(),
    onEngineError: vi.fn(),
    onProfileChanged: vi.fn(),
    onProfileError: vi.fn(),
    onNotification: vi.fn(),
    onAutotuneStart: vi.fn(),
    onAutotuneProgress: vi.fn(),
    onAutotuneLog: vi.fn(),
    onEngineLog: vi.fn(),
    onAutotuneComplete: vi.fn(),
  };

  const capturedHandlers: Record<string, (...args: unknown[]) => void> = {};

  return {
    windowService: {
      minimise: vi.fn(),
      toggleMaximise: vi.fn(),
      hideToTray: vi.fn(),
      quit: vi.fn(),
      showNotification: vi.fn(),
    },
    eventBus: {
      onStatusChanged: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onStatusChanged = cb;
        return unsubFns.onStatusChanged;
      }),
      onEnginesChanged: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onEnginesChanged = cb;
        return unsubFns.onEnginesChanged;
      }),
      onPrivilegeError: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onPrivilegeError = cb;
        return unsubFns.onPrivilegeError;
      }),
      onEngineError: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onEngineError = cb;
        return unsubFns.onEngineError;
      }),
      onProfileChanged: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onProfileChanged = cb;
        return unsubFns.onProfileChanged;
      }),
      onProfileError: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onProfileError = cb;
        return unsubFns.onProfileError;
      }),
      onNotification: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onNotification = cb;
        return unsubFns.onNotification;
      }),
      onAutotuneStart: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onAutotuneStart = cb;
        return unsubFns.onAutotuneStart;
      }),
      onAutotuneProgress: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onAutotuneProgress = cb;
        return unsubFns.onAutotuneProgress;
      }),
      onAutotuneLog: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onAutotuneLog = cb;
        return unsubFns.onAutotuneLog;
      }),
      onEngineLog: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onEngineLog = cb;
        return unsubFns.onEngineLog;
      }),
      onAutotuneComplete: vi.fn().mockImplementation((cb) => {
        capturedHandlers.onAutotuneComplete = cb;
        return unsubFns.onAutotuneComplete;
      }),
      unsubFns,
      capturedHandlers,
    },
  };
});

vi.mock('../services/backend', () => ({
  backendService: {
    getLivePing: vi.fn().mockResolvedValue({ active: true, latency: 30, status: 'ok' }),
    savePingHistory: vi.fn().mockResolvedValue(undefined),
    loadPingHistory: vi.fn().mockResolvedValue([{ lat: 15 }, { lat: 30 }]),
    getLogs: vi.fn().mockResolvedValue(['log line 1', 'log line 2']),
    exportLogs: vi.fn().mockResolvedValue(true),
    getProfiles: vi.fn().mockResolvedValue(['preset_1']),
  },
}));

describe('Hooks Lifecycle & Error Handling Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('useEngineEvents Lifecycle', () => {
    it('subscribes once without re-registering on re-render, handles profile events, and unsubscribes on unmount', () => {
      const onProfileChanged1 = vi.fn();
      const onProfileChanged2 = vi.fn();
      const onProfileError = vi.fn();

      const { rerender, unmount } = renderHook(
        ({ onProf }) =>
          useEngineEvents({
            onStatusChange: vi.fn(),
            onEnginesChange: vi.fn(),
            onPrivilegeError: vi.fn(),
            onAddToast: vi.fn(),
            onAutotuneStart: vi.fn(),
            onAutotuneProgress: vi.fn(),
            onAutotuneLog: vi.fn(),
            onEngineLog: vi.fn(),
            onAutotuneComplete: vi.fn(),
            onProfileChanged: onProf,
            onProfileError,
          }),
        { initialProps: { onProf: onProfileChanged1 } }
      );

      expect(eventBus.onStatusChanged).toHaveBeenCalledTimes(1);
      expect(eventBus.onProfileChanged).toHaveBeenCalledTimes(1);
      expect(eventBus.onProfileError).toHaveBeenCalledTimes(1);

      // Re-render with fresh inline handlers: must NOT unsubscribe or re-subscribe
      rerender({ onProf: onProfileChanged2 });
      expect(eventBus.onStatusChanged).toHaveBeenCalledTimes(1);
      expect(eventBus.onProfileChanged).toHaveBeenCalledTimes(1);

      const unsubs = (eventBus as unknown as { unsubFns: Record<string, ReturnType<typeof vi.fn>> }).unsubFns;
      Object.keys(unsubs).forEach((key) => {
        expect(unsubs[key]).not.toHaveBeenCalled();
      });

      // Latest handler ref is invoked
      const captured = (eventBus as unknown as { capturedHandlers: Record<string, (...args: unknown[]) => void> })
        .capturedHandlers;
      act(() => {
        captured.onProfileChanged('new_profile_v2');
        captured.onProfileError('Все профили заблокированы');
      });
      expect(onProfileChanged1).not.toHaveBeenCalled();
      expect(onProfileChanged2).toHaveBeenCalledWith('new_profile_v2');
      expect(onProfileError).toHaveBeenCalledWith('Все профили заблокированы');

      unmount();

      Object.keys(unsubs).forEach((key) => {
        expect(unsubs[key]).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('usePingPolling Interval & Lifecycle', () => {
    it('preserves loaded pingHistory when engine starts Stopped and polls only when Running', async () => {
      const { result, rerender, unmount } = renderHook(({ status }) => usePingPolling(status), {
        initialProps: { status: 'Stopped' },
      });

      // Flush loadPingHistory microtask while Stopped
      await act(async () => {
        await Promise.resolve();
      });
      expect(result.current.pingHistory).toEqual([15, 30]);

      // No live ping interval when Stopped
      act(() => {
        vi.advanceTimersByTime(10000);
      });
      expect(backendService.getLivePing).not.toHaveBeenCalled();
      expect(result.current.pingHistory).toEqual([15, 30]);

      // Change status to Running
      rerender({ status: 'Running' });

      await act(async () => {
        vi.advanceTimersByTime(5000);
      });
      expect(backendService.getLivePing).toHaveBeenCalledTimes(2);

      // Unmount hook -> interval cleared
      unmount();
      await act(async () => {
        vi.advanceTimersByTime(10000);
      });
      expect(backendService.getLivePing).toHaveBeenCalledTimes(2);
    });

    it('handles backend error gracefully without crashing', async () => {
      vi.mocked(backendService.getLivePing).mockRejectedValueOnce(new Error('Network error'));

      const { result } = renderHook(() => usePingPolling('Running'));

      await act(async () => {
        vi.advanceTimersByTime(5000);
      });

      expect(result.current.livePingData.status).toBe('error');
      expect(result.current.livePingData.active).toBe(false);
    });
  });

  describe('useLogJournal Interval & Lifecycle', () => {
    it('polls logs every 2s only when Running and not scanning, and toggles expansion', async () => {
      const { result, rerender, unmount } = renderHook(
        ({ status, isScanning }) => useLogJournal(status, isScanning),
        { initialProps: { status: 'Stopped', isScanning: false } }
      );

      expect(result.current.isExpanded).toBe(false);
      act(() => {
        result.current.toggleExpanded();
      });
      expect(result.current.isExpanded).toBe(true);

      // Stopped status -> no polling
      act(() => {
        vi.advanceTimersByTime(6000);
      });
      expect(backendService.getLogs).not.toHaveBeenCalled();

      // Change to Running
      rerender({ status: 'Running', isScanning: false });
      await act(async () => {
        vi.advanceTimersByTime(2000);
      });
      expect(backendService.getLogs).toHaveBeenCalledTimes(1);

      // Change to isScanning = true -> polling pauses
      rerender({ status: 'Running', isScanning: true });
      await act(async () => {
        vi.advanceTimersByTime(4000);
      });
      expect(backendService.getLogs).toHaveBeenCalledTimes(1);

      unmount();
    });
  });
});
