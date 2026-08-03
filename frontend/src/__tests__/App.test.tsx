import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
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
    }
  };
})();

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock
});

// Setup Wails mocks on window object
beforeEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
  document.body.className = '';

  (window as any).go = {
    main: {
      App: {
        GetAppVersion: vi.fn().mockResolvedValue('0.3.0'),
        GetOSPlatform: vi.fn().mockResolvedValue('darwin'),
        CheckPrivileges: vi.fn().mockResolvedValue(true),
        GetEngineNames: vi.fn().mockResolvedValue(['zapret', 'byedpi']),
        GetProfiles: vi.fn().mockResolvedValue(['discord_preset_1', 'youtube_preset_2']),
        GetFavoriteProfiles: vi.fn().mockResolvedValue(['discord_preset_1']),
        GetBypassLists: vi.fn().mockResolvedValue(['general.txt', 'youtube.txt']),
        GetSettings: vi.fn().mockResolvedValue({
          engineName: 'zapret',
          selectedProfile: 'discord_preset_1',
          autoStart: false,
          enableGameFilter: true,
          theme: 'monolith'
        }),
        GetLogs: vi.fn().mockResolvedValue(['[STDOUT] Engine initialised']),
        GetLivePing: vi.fn().mockResolvedValue({ active: true, latency: 18, status: 'ok', services: {} }),
        LoadPingHistory: vi.fn().mockResolvedValue([12, 15, 18]),
        SavePingHistory: vi.fn().mockResolvedValue(null),
        StartEngine: vi.fn().mockResolvedValue(null),
        StopEngine: vi.fn().mockResolvedValue(null),
        HideWindowToTray: vi.fn().mockResolvedValue(null),
        QuitApp: vi.fn().mockResolvedValue(null),
        RunDiagnostics: vi.fn().mockResolvedValue([{ Component: 'Driver', Status: 'OK', Details: 'No errors', IsError: false }]),
        ReadBypassList: vi.fn().mockResolvedValue('example.com\ndiscord.com'),
        SaveBypassList: vi.fn().mockResolvedValue(null),
        LoadCustomScript: vi.fn().mockResolvedValue('-- Lua script'),
        SaveCustomScript: vi.fn().mockResolvedValue(null),
        CheckConflicts: vi.fn().mockResolvedValue([]),
        KillConflicts: vi.fn().mockResolvedValue(null)
      }
    }
  };

  (window as any).runtime = {
    WindowMinimise: vi.fn(),
    EventsOnMultiple: vi.fn().mockReturnValue(() => {})
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
        expect(screen.getByText('ROOT ✓')).toBeDefined();
      });
    });

    it('renders Windows window controls when platform is windows', async () => {
      (window as any).go.main.App.GetOSPlatform.mockResolvedValue('windows');
      render(<App />);

      await waitFor(() => {
        expect(screen.getByText('UNBOUND Refresh')).toBeDefined();
        expect(screen.getByText('─')).toBeDefined();
        expect(screen.getByText('✕')).toBeDefined();
      });
    });

    it('renders Linux neutral titlebar when platform is linux', async () => {
      (window as any).go.main.App.GetOSPlatform.mockResolvedValue('linux');
      render(<App />);

      await waitFor(() => {
        expect(screen.getByText('UNBOUND')).toBeDefined();
        expect(screen.getByText('─')).toBeDefined();
        expect(screen.getByText('✕')).toBeDefined();
      });
    });
  });

  describe('Tab switching and Segmented Navigation', () => {
    it('switches between Main, Profiles, Bypass Lists, and Settings tabs', async () => {
      render(<App />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /Подключить/i })).toBeDefined();
      });

      // Switch to Profiles & LUA tab
      fireEvent.click(screen.getByRole('button', { name: /Профили & LUA/i }));
      await waitFor(() => {
        expect(screen.getByText(/Управление профилями обхода/i)).toBeDefined();
      });

      // Switch to Bypass Lists tab
      fireEvent.click(screen.getByRole('button', { name: /Списки обхода/i }));
      await waitFor(() => {
        expect(screen.getByText(/Сохранить список/i)).toBeDefined();
      });

      // Switch to Settings tab
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

  describe('Themes switching', () => {
    it('applies selected theme class to document body', async () => {
      render(<App />);

      await waitFor(() => {
        expect(document.body.classList.contains('theme-monolith')).toBe(true);
      });
    });
  });

  describe('Modals Host', () => {
    it('opens and closes diagnostics modal', async () => {
      render(<App />);

      fireEvent.click(screen.getByRole('button', { name: /Настройки/i }));
      
      await waitFor(() => {
        const diagBtn = screen.getByRole('button', { name: /Диагностика/i });
        fireEvent.click(diagBtn);
      });

      await waitFor(() => {
        expect(screen.getByText(/Проверка системы/i)).toBeDefined();
      });

      // Close modal
      const closeBtn = screen.getByRole('button', { name: /^Закрыть$/ });
      fireEvent.click(closeBtn);

      await waitFor(() => {
        expect(screen.queryByText(/Проверка системы/i)).toBeNull();
      });
    });
  });
});
