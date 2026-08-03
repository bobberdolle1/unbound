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
    onNotification: vi.fn(),
    onAutotuneStart: vi.fn(),
    onAutotuneProgress: vi.fn(),
    onAutotuneLog: vi.fn(),
    onEngineLog: vi.fn(),
    onAutotuneComplete: vi.fn(),
  };

  return {
    windowService: {
      minimise: vi.fn(),
      hideToTray: vi.fn(),
      quit: vi.fn(),
      showNotification: vi.fn(),
    },
    eventBus: {
      onStatusChanged: vi.fn().mockImplementation(() => unsubFns.onStatusChanged),
      onEnginesChanged: vi.fn().mockImplementation(() => unsubFns.onEnginesChanged),
      onPrivilegeError: vi.fn().mockImplementation(() => unsubFns.onPrivilegeError),
      onEngineError: vi.fn().mockImplementation(() => unsubFns.onEngineError),
      onNotification: vi.fn().mockImplementation(() => unsubFns.onNotification),
      onAutotuneStart: vi.fn().mockImplementation(() => unsubFns.onAutotuneStart),
      onAutotuneProgress: vi.fn().mockImplementation(() => unsubFns.onAutotuneProgress),
      onAutotuneLog: vi.fn().mockImplementation(() => unsubFns.onAutotuneLog),
      onEngineLog: vi.fn().mockImplementation(() => unsubFns.onEngineLog),
      onAutotuneComplete: vi.fn().mockImplementation(() => unsubFns.onAutotuneComplete),
      unsubFns,
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
    it('subscribes to all eventBus channels and unsubscribes cleanly on unmount', () => {
      const callbacks = {
        onStatusChange: vi.fn(),
        onEnginesChange: vi.fn(),
        onPrivilegeError: vi.fn(),
        onAddToast: vi.fn(),
        onAutotuneStart: vi.fn(),
        onAutotuneProgress: vi.fn(),
        onAutotuneLog: vi.fn(),
        onEngineLog: vi.fn(),
        onAutotuneComplete: vi.fn(),
      };

      const { unmount } = renderHook(() => useEngineEvents(callbacks));

      // Verify all 10 event handlers were subscribed to
      expect(eventBus.onStatusChanged).toHaveBeenCalled();
      expect(eventBus.onEnginesChanged).toHaveBeenCalled();
      expect(eventBus.onPrivilegeError).toHaveBeenCalled();
      expect(eventBus.onEngineError).toHaveBeenCalled();
      expect(eventBus.onNotification).toHaveBeenCalled();
      expect(eventBus.onAutotuneStart).toHaveBeenCalled();
      expect(eventBus.onAutotuneProgress).toHaveBeenCalled();
      expect(eventBus.onAutotuneLog).toHaveBeenCalled();
      expect(eventBus.onEngineLog).toHaveBeenCalled();
      expect(eventBus.onAutotuneComplete).toHaveBeenCalled();

      // Unmount hook
      unmount();

      // Verify each unsubscribe callback was invoked
      const unsubs = (eventBus as any).unsubFns;
      Object.keys(unsubs).forEach((key) => {
        expect(unsubs[key]).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('usePingPolling Interval & Lifecycle', () => {
    it('starts interval polling when status is Running and stops when status is Stopped', async () => {
      const { rerender, unmount } = renderHook(({ status }) => usePingPolling(status), {
        initialProps: { status: 'Stopped' },
      });

      // No interval when Stopped
      act(() => {
        vi.advanceTimersByTime(10000);
      });
      expect(backendService.getLivePing).not.toHaveBeenCalled();

      // Change status to Running
      rerender({ status: 'Running' });

      // Advance by 5s interval
      await act(async () => {
        vi.advanceTimersByTime(5000);
      });
      expect(backendService.getLivePing).toHaveBeenCalledTimes(1);

      // Unmount hook -> interval cleared
      unmount();
      await act(async () => {
        vi.advanceTimersByTime(10000);
      });
      expect(backendService.getLivePing).toHaveBeenCalledTimes(1); // no extra calls
    });

    it('handles backend error gracefully without crashing', async () => {
      (backendService.getLivePing as any).mockRejectedValueOnce(new Error('Network error'));

      const { result } = renderHook(() => usePingPolling('Running'));

      await act(async () => {
        vi.advanceTimersByTime(5000);
      });

      expect(result.current.livePingData.status).toBe('error');
      expect(result.current.livePingData.active).toBe(false);
    });
  });

  describe('useLogJournal Interval & Lifecycle', () => {
    it('polls logs every 2s only when Running and not scanning', async () => {
      const { rerender, unmount } = renderHook(
        ({ status, isScanning }) => useLogJournal(status, isScanning),
        { initialProps: { status: 'Stopped', isScanning: false } }
      );

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
