import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { PlatformTitlebar } from '../components/shell/PlatformTitlebar';
import { MainControlView } from '../components/views/MainControlView';
import { AppNavigation } from '../components/shell/AppNavigation';

describe('UI Parity & Precision Monochrome Tests', () => {
  it('renders PlatformTitlebar with U-Break logo and ADMIN badge on Windows', () => {
    const { container } = render(<PlatformTitlebar platform="windows" appVersion="0.3.0-rc.3" />);
    expect(screen.getByText('UNBOUND Refresh')).toBeDefined();
    expect(screen.getByText('ADMIN')).toBeDefined();
    // Verify SVG icon elements exist
    const svgs = container.querySelectorAll('svg');
    expect(svgs.length).toBeGreaterThan(0);
  });

  it('renders MainControlView AutoTune button without colored ⚡ emoji', () => {
    render(
      <MainControlView
        statusLedState="disconnected"
        isConnected={false}
        isConnecting={false}
        disableMain={false}
        toggleConnection={() => {}}
        selectedProfile="Recommended (hostfakesplit)"
        setSelectedProfile={() => {}}
        sortedProfiles={['Recommended (hostfakesplit)']}
        selectedEngine="zapret"
        handleToggleFavorite={() => {}}
        favoriteProfiles={[]}
        handleAutoTune={() => {}}
        isScanning={false}
        scanProgress=""
        autotuneProgress={null}
        handleCancelAutoTune={() => {}}
        livePingData={{ active: false, latency: 0, status: 'stopped' }}
        pingHistory={[]}
      />
    );
    expect(screen.getByText('Автоподбор стратегии')).toBeDefined();
    expect(screen.queryByText(/⚡/)).toBeNull();
  });

  it('renders full navigation labels for desktop viewports', () => {
    render(<AppNavigation activeTab="main" onTabChange={() => {}} />);
    expect(screen.getAllByText('Главная').length).toBeGreaterThan(0);
    expect(screen.getByText('Профили & LUA')).toBeDefined();
    expect(screen.getByText('Списки обхода')).toBeDefined();
    expect(screen.getAllByText('Настройки').length).toBeGreaterThan(0);
  });
});
