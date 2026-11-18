import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, render, screen, fireEvent, waitFor } from '@testing-library/react';
import App from '../App';

// Setup Mock LocalStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] || null,
    setItem: (key: string, value: string) => {
      store[key] = value.toString();
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
})();

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
});

const eventSubscribers: Record<string, Array<(...args: unknown[]) => void>> = {};

beforeEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
  document.body.className = '';
  Object.keys(eventSubscribers).forEach((k) => delete eventSubscribers[k]);

  (window as unknown as { go: unknown }).go = {
    main: {
      App: {
        GetAppVersion: vi.fn().mockResolvedValue('0.8.0'),
        GetOSPlatform: vi.fn().mockResolvedValue('darwin'),
        CheckPrivileges: vi.fn().mockResolvedValue(true),
        GetEngineNames: vi.fn().mockResolvedValue(['zapret', 'byedpi']),
        GetProfiles: vi.fn().mockResolvedValue(['discord_preset_1', 'youtube_preset_2']),
        GetFavoriteProfiles: vi.fn().mockResolvedValue(['discord_preset_1']),
        GetBypassLists: vi.fn().mockResolvedValue(['general.txt', 'youtube.txt']),
        GetSettings: vi.fn().mockResolvedValue({
          autoStart: false,
          startMinimized: false,
          autoStartProfile: true,
          defaultProfile: 'discord_preset_1',
          startupProfileMode: 'Last Used',
          gameFilter: true,
          autoUpdateEnabled: false,
          showLogs: true,
          enableTCPTimestamps: true,
          discordCacheAutoClean: true,
          secureDns: true,
          favoriteProfiles: ['discord_preset_1'],
          autoReconnect: false,
          autoTuneTargets: ['https://www.youtube.com', 'https://discord.com'],
          diagnosticsMode: 'extended',
          autoUpdatePolicy: 'manual',
        }),
        SaveSettings: vi.fn().mockResolvedValue(null),
        GetAutoTuneVNextManagedStatus: vi
          .fn()
          .mockResolvedValue({ state: 'DIRECT', active: false, needs_revalidation: false }),
        GetLogs: vi.fn().mockResolvedValue(['[STDOUT] Engine initialised']),
        GetLivePing: vi.fn().mockResolvedValue({ active: true, latency: 18, status: 'ok', services: {} }),
        LoadPingHistory: vi.fn().mockResolvedValue([12, 15, 18]),
        SavePingHistory: vi.fn().mockResolvedValue(null),
        StartEngine: vi.fn().mockResolvedValue(null),
        StopEngine: vi.fn().mockResolvedValue(null),
        HideWindowToTray: vi.fn().mockResolvedValue(null),
        QuitApp: vi.fn().mockResolvedValue(null),
        RunDiagnostics: vi
          .fn()
          .mockResolvedValue([{ Component: 'Driver', Status: 'OK', Details: 'No errors', IsError: false }]),
        ReadBypassList: vi.fn().mockResolvedValue('example.com\ndiscord.com'),
        SaveBypassList: vi.fn().mockResolvedValue(null),
        LoadCustomScript: vi.fn().mockResolvedValue('-- Lua script'),
        SaveCustomScript: vi.fn().mockResolvedValue(null),
        CheckConflicts: vi.fn().mockResolvedValue([]),
        KillConflicts: vi.fn().mockResolvedValue(null),
        GetSystemComponentState: vi.fn().mockResolvedValue({ components: [] }),
        RunDoctor: vi.fn().mockResolvedValue({ groups: [] }),
        StartDoctor: vi.fn().mockResolvedValue({
          runId: 'doc_1',
          mode: 'quick',
          total: 16,
          startedAt: '2026-11-18T00:00:00Z',
        }),
      },
    },
  };

  (window as unknown as { runtime: unknown }).runtime = {
    WindowMinimise: vi.fn(),
    WindowToggleMaximise: vi.fn(),
    EventsOnMultiple: vi.fn().mockImplementation((eventName: string, cb: (...args: unknown[]) => void) => {
      eventSubscribers[eventName] = eventSubscribers[eventName] || [];
      eventSubscribers[eventName].push(cb);
      return () => {};
    }),
  };
});

describe('UNBOUND App Main Integration Tests', () => {
  describe('Platform-aware titlebar', () => {
    it('renders macOS traffic lights when platform is darwin', async () => {
      (window as any).go.main.App.GetOSPlatform.mockResolvedValue('darwin');
      render(<App />);

      await waitFor(() => {
        expect(screen.getByTitle('Закрыть в трей')).toBeDefined();
        expect(screen.getByTitle('Свернуть')).toBeDefined();
        expect(screen.getByText('ROOT')).toBeDefined();
      });
    });

    it('renders Windows window controls when platform is windows', async () => {
      (window as any).go.main.App.GetOSPlatform.mockResolvedValue('windows');
      render(<App />);

      await waitFor(() => {
        expect(screen.getByText('UNBOUND Refresh')).toBeDefined();
        expect(screen.getByTitle('Свернуть')).toBeDefined();
        expect(screen.getByTitle('Закрыть')).toBeDefined();
      });
    });

    it('renders Linux neutral titlebar and quits cleanly on close', async () => {
      (window as any).go.main.App.GetOSPlatform.mockResolvedValue('linux');
      render(<App />);

      await waitFor(() => {
        expect(screen.getByText('UNBOUND')).toBeDefined();
        expect(screen.getByTitle('Свернуть')).toBeDefined();
        expect(screen.getByTitle('Закрыть')).toBeDefined();
      });

      fireEvent.click(screen.getByTitle('Закрыть'));
      expect((window as any).go.main.App.QuitApp).toHaveBeenCalledTimes(1);
      expect((window as any).go.main.App.HideWindowToTray).not.toHaveBeenCalled();
    });
  });

  describe('Tab switching and Segmented Navigation', () => {
    it('switches between Main, Profiles, Bypass Lists, and Settings tabs', async () => {
      render(<App />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Подключить/i })).toBeDefined();
      });

      fireEvent.click(screen.getByRole('button', { name: /Профили & LUA/i }));
      await waitFor(() => {
        expect(screen.getByText(/Управление профилями обхода/i)).toBeDefined();
      });

      fireEvent.click(screen.getByRole('button', { name: /Списки обхода/i }));
      await waitFor(() => {
        expect(screen.getByText(/Сохранить список/i)).toBeDefined();
      });

      fireEvent.click(screen.getByRole('button', { name: /Настройки/i }));
      await waitFor(() => {
        expect(screen.getByText(/Параметры системы/i)).toBeDefined();
      });
    });
  });

  describe('Status states & Connect/Disconnect actions', () => {
    it('handles connect and disconnect action clicks', async () => {
      render(<App />);

      await waitFor(() => {
        expect(screen.getByText('Служба остановлена')).toBeDefined();
      });

      const connectBtn = screen.getByRole('button', { name: /Подключить/i });
      fireEvent.click(connectBtn);

      await waitFor(() => {
        expect((window as any).go.main.App.StartEngine).toHaveBeenCalled();
      });
    });
  });

  describe('Themes switching & Settings preservation', () => {
    it('applies selected theme class to document body', async () => {
      render(<App />);

      await waitFor(() => {
        expect(document.body.classList.contains('theme-monolith')).toBe(true);
      });
    });

    it('preserves all persisted Settings fields without overwriting unseen fields with zero values', async () => {
      render(<App />);

      await waitFor(() => {
        expect((window as any).go.main.App.GetSettings).toHaveBeenCalled();
      });

      fireEvent.click(screen.getByRole('button', { name: /Настройки/i }));
      await screen.findByText(/Параметры системы/i);

      const autoStartCheckbox = document.querySelector('[role="checkbox"]#autoStart') as HTMLElement;
      fireEvent.click(autoStartCheckbox);

      await waitFor(() => {
        expect((window as any).go.main.App.SaveSettings).toHaveBeenCalledTimes(1);
      });

      const saved = (window as any).go.main.App.SaveSettings.mock.calls[0][0];
      expect(saved.autoStart).toBe(true);
      expect(saved.autoTuneTargets).toEqual(['https://www.youtube.com', 'https://discord.com']);
      expect(saved.diagnosticsMode).toBe('extended');
      expect(saved.autoUpdatePolicy).toBe('manual');
      expect(saved.secureDns).toBe(true);
      expect(saved.gameFilter).toBe(true);
      expect(saved.autoReconnect).toBe(false);
      expect(saved.discordCacheAutoClean).toBe(true);
      expect(saved.enableTCPTimestamps).toBe(true);
      expect(saved.showLogs).toBe(true);
      expect(saved.autoUpdateEnabled).toBe(false);
      expect(saved.defaultProfile).toBe('discord_preset_1');
    });
  });

  describe('LogJournalDrawer & Backend Events', () => {
    it('renders collapsed log footer bar and toggles expansion via Ctrl+J', async () => {
      render(<App />);

      await waitFor(() => {
        expect(screen.getByText(/Журнал \(0\)/)).toBeDefined();
      });

      fireEvent.keyDown(window, { key: 'j', ctrlKey: true });
      await waitFor(() => {
        expect(screen.getByText(/Системный журнал L3\/L4 \(0\)/)).toBeDefined();
      });

      fireEvent.keyDown(window, { key: '`', code: 'Backquote', ctrlKey: true });
      await waitFor(() => {
        expect(screen.queryByText(/Системный журнал L3\/L4/)).toBeNull();
        expect(screen.getByText(/Журнал \(0\)/)).toBeDefined();
      });
    });

    it('updates UI on profile_changed and displays toast on profile_error', async () => {
      render(<App />);
      await waitFor(() => {
        expect(screen.getByText('Служба остановлена')).toBeDefined();
      });

      await act(async () => {
        eventSubscribers['profile_changed']?.forEach((cb) => cb('auto_switched_profile'));
        eventSubscribers['profile_error']?.forEach((cb) => cb('Все профили заблокированы после 3 попыток'));
      });

      expect(screen.getAllByText('auto_switched_profile').length).toBeGreaterThan(0);
      expect(screen.getByText('Сбой переключения профиля')).toBeDefined();
      expect(screen.getAllByText(/Все профили заблокированы после 3 попыток/).length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('Modals Host', () => {
    it('opens diagnostics modal with single async StartDoctor call and closes cleanly', async () => {
      render(<App />);

      fireEvent.click(screen.getByRole('button', { name: /Настройки/i }));
      const diagBtn = await screen.findByRole('button', { name: /Диагностика/i });
      fireEvent.click(diagBtn);

      await waitFor(() => {
        expect(screen.getByText(/Проверка системы/i)).toBeDefined();
        expect((window as any).go.main.App.StartDoctor).toHaveBeenCalledTimes(1);
        expect((window as any).go.main.App.StartDoctor).toHaveBeenCalledWith('quick');
      });
      expect((window as any).go.main.App.RunDoctor).not.toHaveBeenCalled();

      const closeBtn = screen.getByRole('button', { name: /^Закрыть$/ });
      fireEvent.click(closeBtn);

      await waitFor(() => {
        expect(screen.queryByText(/Проверка системы/i)).toBeNull();
      });
    });
  });
});
