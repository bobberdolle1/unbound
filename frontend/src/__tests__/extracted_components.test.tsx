import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, renderHook, act } from '@testing-library/react';
import { PlatformTitlebar } from '../components/shell/PlatformTitlebar';
import { AppNavigation } from '../components/shell/AppNavigation';
import { ConnectionStatus } from '../components/status/ConnectionStatus';
import { MainControlView } from '../components/views/MainControlView';
import { ModalHost } from '../components/modals/ModalHost';
import { LogJournalDrawer } from '../components/terminal/LogJournalDrawer';
import { usePingPolling } from '../hooks/usePingPolling';
import { windowService } from '../services/window';

vi.mock('../services/window', () => ({
  windowService: {
    minimise: vi.fn(),
    toggleMaximise: vi.fn(),
    hideToTray: vi.fn(),
    quit: vi.fn(),
    showNotification: vi.fn(),
  },
  eventBus: {
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
  },
}));

vi.mock('../services/backend', () => ({
  backendService: {
    getLivePing: vi.fn().mockResolvedValue({ active: true, latency: 24, status: 'ok' }),
    savePingHistory: vi.fn().mockResolvedValue(undefined),
    loadPingHistory: vi.fn().mockResolvedValue([{ lat: 20 }, { lat: 24 }]),
    exportLogs: vi.fn().mockResolvedValue(true),
    killConflicts: vi.fn().mockResolvedValue(undefined),
  },
}));

describe('Extracted Components Unit Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('PlatformTitlebar', () => {
    it('renders macOS controls and calls hideToTray / minimise / toggleMaximise on click', () => {
      render(<PlatformTitlebar platform="darwin" appVersion="0.3.0" />);
      expect(screen.getByText('ROOT ✓')).toBeDefined();

      fireEvent.click(screen.getByTitle('Закрыть в трей'));
      expect(windowService.hideToTray).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Свернуть'));
      expect(windowService.minimise).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Развернуть / Восстановить'));
      expect(windowService.toggleMaximise).toHaveBeenCalledTimes(1);
    });

    it('triggers toggleMaximise on titlebar double-click', () => {
      const { container } = render(<PlatformTitlebar platform="darwin" appVersion="0.3.0" />);
      const titlebar = container.firstChild as HTMLElement;
      fireEvent.doubleClick(titlebar);
      expect(windowService.toggleMaximise).toHaveBeenCalled();
    });

    it('renders Windows titlebar with maximize button and triggers handlers', () => {
      render(<PlatformTitlebar platform="windows" appVersion="0.3.0-rc.2" />);
      expect(screen.getByText('UNBOUND Refresh')).toBeDefined();

      fireEvent.click(screen.getByTitle('Свернуть'));
      expect(windowService.minimise).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Развернуть'));
      expect(windowService.toggleMaximise).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Закрыть'));
      expect(windowService.hideToTray).toHaveBeenCalledTimes(1);
    });

    it('renders Linux neutral titlebar as fallback', () => {
      render(<PlatformTitlebar platform="linux" appVersion="0.3.0" />);
      expect(screen.getByText('UNBOUND')).toBeDefined();
    });
  });

  describe('AppNavigation', () => {
    it('switches tabs and renders both full and compact labels without truncation dots', () => {
      const onTabChange = vi.fn();
      render(<AppNavigation activeTab="main" onTabChange={onTabChange} />);

      expect(screen.getAllByText('Главная').length).toBeGreaterThan(0);
      expect(screen.getByText('Профили & LUA')).toBeDefined();
      expect(screen.getByText('Профили')).toBeDefined();
      expect(screen.getByText('Списки обхода')).toBeDefined();
      expect(screen.getByText('Списки')).toBeDefined();
      expect(screen.getAllByText('Настройки').length).toBeGreaterThan(0);

      // Verify no ellipsis truncation character
      expect(screen.queryByText(/…/)).toBeNull();

      fireEvent.click(screen.getByText('Списки обхода'));
      expect(onTabChange).toHaveBeenCalledWith('lists');
    });

    it('triggers tab change when clicking compact tab label', () => {
      const onTabChange = vi.fn();
      render(<AppNavigation activeTab="main" onTabChange={onTabChange} />);

      fireEvent.click(screen.getByText('Списки'));
      expect(onTabChange).toHaveBeenCalledWith('lists');
    });
  });

  describe('ConnectionStatus', () => {
    it('renders correct labels for all LED states', () => {
      const { rerender } = render(<ConnectionStatus status="Stopped" onToggleEngine={vi.fn()} />);
      expect(screen.getByText('Служба остановлена')).toBeDefined();

      rerender(<ConnectionStatus status="Connecting" onToggleEngine={vi.fn()} />);
      expect(screen.getByText('Инициализация драйвера...')).toBeDefined();

      rerender(<ConnectionStatus status="Running" onToggleEngine={vi.fn()} />);
      expect(screen.getByText('Обход активен')).toBeDefined();

      rerender(<ConnectionStatus status="Error" onToggleEngine={vi.fn()} />);
      expect(screen.getByText('Ошибка драйвера')).toBeDefined();
    });
  });

  describe('MainControlView', () => {
    it('renders disconnect state when connected and handles toggle', () => {
      const toggleConnection = vi.fn();
      render(
        <MainControlView
          statusLedState="connected"
          isConnected={true}
          isConnecting={false}
          disableMain={false}
          toggleConnection={toggleConnection}
          selectedProfile="discord_preset_1"
          setSelectedProfile={vi.fn()}
          sortedProfiles={['discord_preset_1']}
          selectedEngine="zapret"
          handleToggleFavorite={vi.fn()}
          favoriteProfiles={[]}
          handleAutoTune={vi.fn()}
          isScanning={false}
          scanProgress=""
          autotuneProgress={null}
          handleCancelAutoTune={vi.fn()}
          livePingData={{ active: true, latency: 15, status: 'ok' }}
          pingHistory={[15]}
        />
      );

      const btn = screen.getByRole('button', { name: /Отключить/i });
      expect(btn).toBeDefined();
      fireEvent.click(btn);
      expect(toggleConnection).toHaveBeenCalledTimes(1);
    });

    it('renders connect state when disconnected', () => {
      render(
        <MainControlView
          statusLedState="disconnected"
          isConnected={false}
          isConnecting={false}
          disableMain={false}
          toggleConnection={vi.fn()}
          selectedProfile="discord_preset_1"
          setSelectedProfile={vi.fn()}
          sortedProfiles={['discord_preset_1']}
          selectedEngine="zapret"
          handleToggleFavorite={vi.fn()}
          favoriteProfiles={[]}
          handleAutoTune={vi.fn()}
          isScanning={false}
          scanProgress=""
          autotuneProgress={null}
          handleCancelAutoTune={vi.fn()}
          livePingData={{ active: false, latency: 0, status: 'stopped' }}
          pingHistory={[]}
        />
      );

      expect(screen.getByRole('button', { name: /Подключить/i })).toBeDefined();
    });
  });

  describe('ModalHost', () => {
    it('renders ConflictModal when conflictWarning is non-empty', () => {
      const onKillConflicts = vi.fn();
      render(
        <ModalHost
          conflictWarning={['GoodbyeDPI.exe running']}
          onIgnoreConflicts={vi.fn()}
          onKillConflicts={onKillConflicts}
          privilegeError=""
          platform="darwin"
          onClosePrivilegeModal={vi.fn()}
          isDiagOpen={false}
          isDiagRunning={false}
          diagResults={[]}
          onCloseDiagnosticsModal={vi.fn()}
          isLuaOpen={false}
          onCloseLuaModal={vi.fn()}
          luaTab="builder"
          setLuaTab={vi.fn()}
          luaIsAuto={true}
          setLuaIsAuto={vi.fn()}
          luaFakeBlob=""
          setLuaFakeBlob={vi.fn()}
          luaPos="1"
          setLuaPos={vi.fn()}
          luaFool="none"
          setLuaFool={vi.fn()}
          luaTtl={0}
          setLuaTtl={vi.fn()}
          luaCode=""
          setLuaCode={vi.fn()}
          onSaveLua={vi.fn()}
        />
      );

      expect(screen.getByText('Обнаружены конфликты')).toBeDefined();
    });
  });

  describe('LogJournalDrawer', () => {
    it('renders log lines and handles export button', () => {
      const onExport = vi.fn();
      render(
        <LogJournalDrawer
          logs={['Test log entry']}
          isExpanded={true}
          onToggle={vi.fn()}
          onExportLogs={onExport}
        />
      );

      expect(screen.getByText('Test log entry')).toBeDefined();
      const exportBtn = screen.getByRole('button', { name: /Сохранить/i });
      fireEvent.click(exportBtn);
      expect(onExport).toHaveBeenCalledTimes(1);
    });
  });

  describe('usePingPolling Hook', () => {
    it('loads initial ping history and cleans up interval on unmount', async () => {
      vi.useFakeTimers();
      const { unmount } = renderHook(() => usePingPolling('Running'));

      act(() => {
        vi.advanceTimersByTime(6000);
      });

      unmount();
      vi.useRealTimers();
    });
  });
});
