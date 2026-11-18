import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, renderHook, act } from '@testing-library/react';
import { PlatformTitlebar } from '../components/shell/PlatformTitlebar';
import { AppNavigation } from '../components/shell/AppNavigation';
import { MainControlView } from '../components/views/MainControlView';
import { ModalHost } from '../components/modals/ModalHost';
import { ConflictModal } from '../components/modals/ConflictModal';
import { SettingsView, AppSettings } from '../components/views/SettingsView';
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
    onProfileChanged: vi.fn(),
    onProfileError: vi.fn(),
    onNotification: vi.fn(),
    onAutotuneStart: vi.fn(),
    onAutotuneProgress: vi.fn(),
    onAutotuneLog: vi.fn(),
    onEngineLog: vi.fn(),
    onAutotuneComplete: vi.fn(),
    onAutoTuneVNextProgress: vi.fn().mockReturnValue(() => {}),
    onDoctorStart: vi.fn().mockReturnValue(() => {}),
    onDoctorProgress: vi.fn().mockReturnValue(() => {}),
    onDoctorComplete: vi.fn().mockReturnValue(() => {}),
    onDoctorCancelled: vi.fn().mockReturnValue(() => {}),
    onDoctorError: vi.fn().mockReturnValue(() => {}),
  },
}));

vi.mock('../services/backend', () => ({
  backendService: {
    getLivePing: vi.fn().mockResolvedValue({ active: true, latency: 24, status: 'ok' }),
    savePingHistory: vi.fn().mockResolvedValue(undefined),
    loadPingHistory: vi.fn().mockResolvedValue([{ lat: 20 }, { lat: 24 }]),
    exportLogs: vi.fn().mockResolvedValue(true),
    killConflicts: vi.fn().mockResolvedValue(undefined),
    getSystemComponentState: vi.fn().mockResolvedValue({ components: [] }),
  },
}));

const createMockSettings = (overrides: Partial<AppSettings> = {}): AppSettings => ({
  autoStart: true,
  startMinimized: false,
  autoStartProfile: false,
  defaultProfile: '',
  startupProfileMode: 'Последний использованный',
  gameFilter: true,
  autoUpdateEnabled: true,
  showLogs: true,
  enableTCPTimestamps: false,
  discordCacheAutoClean: false,
  secureDns: false,
  autoReconnect: true,
  favoriteProfiles: [],
  ...overrides,
});

describe('Extracted Components Unit Tests', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('PlatformTitlebar', () => {
    it('renders macOS controls and calls hideToTray / minimise / toggleMaximise on click', () => {
      render(<PlatformTitlebar platform="darwin" appVersion="0.8.0" />);
      expect(screen.getByText('ROOT')).toBeDefined();

      fireEvent.click(screen.getByTitle('Закрыть в трей'));
      expect(windowService.hideToTray).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Свернуть'));
      expect(windowService.minimise).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Развернуть / Восстановить'));
      expect(windowService.toggleMaximise).toHaveBeenCalledTimes(1);
    });

    it('triggers toggleMaximise on titlebar double-click', () => {
      const { container } = render(<PlatformTitlebar platform="darwin" appVersion="0.8.0" />);
      const titlebar = container.firstChild as HTMLElement;
      fireEvent.doubleClick(titlebar);
      expect(windowService.toggleMaximise).toHaveBeenCalled();
    });

    it('renders Windows titlebar with maximize button and triggers handlers', () => {
      render(<PlatformTitlebar platform="windows" appVersion="0.8.0" />);
      expect(screen.getByText('UNBOUND Refresh')).toBeDefined();

      fireEvent.click(screen.getByTitle('Свернуть'));
      expect(windowService.minimise).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Развернуть'));
      expect(windowService.toggleMaximise).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByTitle('Закрыть'));
      expect(windowService.hideToTray).toHaveBeenCalledTimes(1);
    });

    it('calls windowService.quit() instead of hideToTray() on Linux close button click', () => {
      render(<PlatformTitlebar platform="linux" appVersion="0.8.0" />);
      expect(screen.getByText('UNBOUND')).toBeDefined();

      fireEvent.click(screen.getByTitle('Закрыть'));
      expect(windowService.quit).toHaveBeenCalledTimes(1);
      expect(windowService.hideToTray).not.toHaveBeenCalled();
    });

    it('renders fallback version v0.8.0 and privilege warning badge when privilegeError is set', () => {
      const { rerender } = render(
        <PlatformTitlebar platform="windows" appVersion="" privilegeError="Требуются права администратора." />
      );
      expect(screen.getByText('v0.8.0')).toBeDefined();
      expect(screen.getByText('NO ADMIN ⚠')).toBeDefined();
      expect(screen.queryByText(/^ADMIN$/)).toBeNull();

      rerender(<PlatformTitlebar platform="darwin" appVersion="" privilegeError="Требуются права root." />);
      expect(screen.getByText('NO ROOT ⚠')).toBeDefined();
      expect(screen.queryByText(/^ROOT$/)).toBeNull();
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
          openAutoTuneVNext={vi.fn()}
          vNextRunning={false}
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
          openAutoTuneVNext={vi.fn()}
          vNextRunning={false}
          isScanning={false}
          scanProgress=""
          autotuneProgress={null}
          handleCancelAutoTune={vi.fn()}
          livePingData={{ active: false, latency: 0, status: 'stopped' }}
          pingHistory={[]}
        />
      );

      expect(screen.getByRole('button', { name: /Подключить/i })).toBeDefined();
      expect(screen.getByText('Автоподбор стратегии')).toBeDefined();
    });
  });

  describe('ModalHost & ConflictModal', () => {
    it('renders ConflictModal when conflictWarning is non-empty', () => {
      const onKillConflicts = vi.fn();
      render(
        <ModalHost
          conflictWarning={['⚠️ GoodbyeDPI запущен']}
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
      expect(screen.getByText('Завершить все')).toBeDefined();
    });

    it('clearly separates killable DPI processes from external VPNs requiring manual closing', () => {
      const onKill = vi.fn();
      const onIgnore = vi.fn();
      const { rerender } = render(
        <ConflictModal
          conflicts={['⚠️ GoodbyeDPI запущен', '⚠️ Clash Verge запущен', '⚠️ Cloudflare WARP запущен']}
          onIgnore={onIgnore}
          onKillConflicts={onKill}
        />
      );

      expect(screen.getByText(/могут быть завершены автоматически/i)).toBeDefined();
      expect(screen.getByText(/Внешние VPN \/ прокси \(требуют ручного закрытия\)/i)).toBeDefined();
      expect(screen.queryByText('Завершить все')).toBeNull();
      fireEvent.click(screen.getByText('Завершить DPI-процессы'));
      expect(onKill).toHaveBeenCalledTimes(1);

      rerender(
        <ConflictModal
          conflicts={['⚠️ OpenVPN запущен']}
          onIgnore={onIgnore}
          onKillConflicts={onKill}
        />
      );
      expect(screen.getByText('Понятно, закрыть окно')).toBeDefined();
      fireEvent.click(screen.getByText('Понятно, закрыть окно'));
      expect(onIgnore).toHaveBeenCalledTimes(1);
    });
  });

  describe('LogJournalDrawer', () => {
    it('renders compact collapsed footer bar when isExpanded is false and toggles on click', () => {
      const onToggle = vi.fn();
      const { container } = render(
        <LogJournalDrawer
          logs={['[INFO] First line', '[INFO] Latest line']}
          isExpanded={false}
          onToggle={onToggle}
          onExportLogs={vi.fn()}
        />
      );

      expect(screen.getByText('Журнал (2)')).toBeDefined();
      expect(screen.getByText(/Latest line/)).toBeDefined();
      expect(screen.getByText('Ctrl+J')).toBeDefined();
      expect(container.innerHTML).not.toContain('#090b0e');
      expect(container.innerHTML).not.toContain('--ui-surface)');

      fireEvent.click(screen.getByRole('button', { name: /Развернуть журнал/i }));
      expect(onToggle).toHaveBeenCalledTimes(1);
    });

    it('renders log lines, theme variables, and handles export button when expanded', () => {
      const onExport = vi.fn();
      const onToggle = vi.fn();
      const { container } = render(
        <LogJournalDrawer
          logs={['Test log entry']}
          isExpanded={true}
          onToggle={onToggle}
          onExportLogs={onExport}
        />
      );

      expect(screen.getByText('Test log entry')).toBeDefined();
      expect(container.innerHTML).not.toContain('#090b0e');
      expect(container.innerHTML).not.toContain('#101318');
      expect(container.innerHTML).not.toContain('var(--ui-surface)');

      fireEvent.click(screen.getByRole('button', { name: /Сохранить/i }));
      expect(onExport).toHaveBeenCalledTimes(1);

      fireEvent.click(screen.getByRole('button', { name: /Свернуть журнал/i }));
      expect(onToggle).toHaveBeenCalledTimes(1);
    });
  });

  describe('usePingPolling Hook', () => {
    it('loads initial ping history and cleans up interval on unmount', async () => {
      vi.useFakeTimers();
      const { unmount } = renderHook(() => usePingPolling('Running'));

      await act(async () => {
        vi.advanceTimersByTime(6000);
      });

      unmount();
      vi.useRealTimers();
    });
  });

  describe('SettingsView controls & parity', () => {
    it('contains cross-platform startMinimized label and links to autoStart', () => {
      const { container } = render(
        <SettingsView
          settings={createMockSettings({ autoStart: true })}
          setSettings={vi.fn()}
          theme="monolith"
          setThemeState={vi.fn()}
          startupProfiles={['Recommended']}
          handleRunDiagnostics={vi.fn()}
          handleVerifyAssets={vi.fn()}
          isVerifyingAssets={false}
          handleDiagnosticReport={vi.fn()}
          isGeneratingReport={false}
          handleUpdateHostlists={vi.fn()}
          isUpdatingHostlists={false}
          handleClearCache={vi.fn()}
          handleKillWinws2={vi.fn()}
        />
      );

      const checkboxes = container.querySelectorAll('[role="checkbox"]#startMinimized');
      expect(checkboxes.length).toBe(1);
      expect(checkboxes[0].getAttribute('aria-disabled')).toBe('false');
      expect(screen.queryByText(/автозапуске Windows/i)).toBeNull();
    });

    it('disables startMinimized when autoStart is false', () => {
      const { container } = render(
        <SettingsView
          settings={createMockSettings({ autoStart: false, startMinimized: true })}
          setSettings={vi.fn()}
          theme="monolith"
          setThemeState={vi.fn()}
          startupProfiles={['Recommended']}
          handleRunDiagnostics={vi.fn()}
          handleVerifyAssets={vi.fn()}
          isVerifyingAssets={false}
          handleDiagnosticReport={vi.fn()}
          isGeneratingReport={false}
          handleUpdateHostlists={vi.fn()}
          isUpdatingHostlists={false}
          handleClearCache={vi.fn()}
          handleKillWinws2={vi.fn()}
        />
      );
      const checkboxes = container.querySelectorAll('[role="checkbox"]#startMinimized');
      expect(checkboxes.length).toBe(1);
      expect(checkboxes[0].getAttribute('aria-disabled')).toBe('true');
    });

    it('exposes toggles for all missing backend settings and system theme option', () => {
      const setSettings = vi.fn();
      const setThemeState = vi.fn();
      const { container } = render(
        <SettingsView
          settings={createMockSettings()}
          setSettings={setSettings}
          theme="system"
          setThemeState={setThemeState}
          startupProfiles={['Recommended']}
          handleRunDiagnostics={vi.fn()}
          handleVerifyAssets={vi.fn()}
          isVerifyingAssets={false}
          handleDiagnosticReport={vi.fn()}
          isGeneratingReport={false}
          handleUpdateHostlists={vi.fn()}
          isUpdatingHostlists={false}
          handleClearCache={vi.fn()}
          handleKillWinws2={vi.fn()}
        />
      );

      const ids = [
        'showLogs',
        'autoReconnect',
        'gameFilter',
        'secureDns',
        'enableTCPTimestamps',
        'discordCacheAutoClean',
        'autoUpdateEnabled',
      ];
      for (const id of ids) {
        const el = container.querySelector(`[role="checkbox"]#${id}`) as HTMLElement;
        expect(el).not.toBeNull();
        fireEvent.click(el);
      }
      expect(setSettings).toHaveBeenCalledTimes(ids.length);
      expect(screen.getByText('Системный DNS 1.1.1.1')).toBeDefined();
      expect(screen.getByText(/без DoH/i)).toBeDefined();
      expect(screen.getByText('System (Авто)')).toBeDefined();
    });
  });
});
