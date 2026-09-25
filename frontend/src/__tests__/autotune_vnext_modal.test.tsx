import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { AutoTuneVNextModal } from '../components/modals/AutoTuneVNextModal';
import { MainControlView } from '../components/views/MainControlView';
import { backendService } from '../services/backend';
import { main } from '../../wailsjs/go/models';

const config = new main.AutoTuneVNextExperimentConfig({
  targets: [{ id: 'youtube-web', label: 'YouTube', target: 'https://www.youtube.com/generate_204' }],
  default_control: { id: 'cloudflare-control', label: 'Cloudflare', target: 'https://cloudflare.com/cdn-cgi/trace' },
});
const status = (overrides: Record<string, unknown> = {}) => new main.AutoTuneVNextManagedStatus({ state: 'DIRECT', active: false, needs_revalidation: false, ...overrides });
const result = (overrides: Record<string, unknown> = {}) => new main.AutoTuneVNextResult({ status: 'COMPLETED_SELECTED', state_restored: true, target: 'https://www.youtube.com/generate_204', catalog_status: 'READY', selected_strategy_id: 'prod-tls-multisplit-1-v1', selected_fingerprint: '0123456789abcdef', apply_available: true, apply_token: 'opaque-token', ...overrides });

async function openModal() {
  render(<AutoTuneVNextModal isOpen={true} onClose={vi.fn()} onRunningChange={vi.fn()} />);
  await screen.findByRole('dialog');
}

describe('primary AutoTune vNext modal', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(backendService, 'getAutoTuneVNextExperimentConfig').mockResolvedValue(config);
    vi.spyOn(backendService, 'getAutoTuneVNextManagedStatus').mockResolvedValue(status());
    vi.spyOn(backendService, 'cancelAutoTune').mockResolvedValue(undefined);
  });

  it('uses product-owned presets and contains no experimental wording', async () => {
    const run = vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result({ status: 'COMPLETED_NO_ACTION_NEEDED', apply_available: false }));
    await openModal();
    expect(screen.getByText('Автоподбор стратегии')).toBeDefined();
    expect(screen.queryByText(/эксперимент/i)).toBeNull();
    fireEvent.click(screen.getByText('Проверить стратегии'));
    await waitFor(() => expect(run).toHaveBeenCalledWith('youtube-web', ''));
  });

  it('offers Apply only for backend-issued verified selection and never accepts strategy parameters', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result());
    const apply = vi.spyOn(backendService, 'applyAutoTuneVNextSelection').mockResolvedValue(status({ state: 'APPLIED', active: true, strategy_id: 'prod-tls-multisplit-1-v1', target: 'https://www.youtube.com/generate_204' }));
    await openModal();
    fireEvent.click(screen.getByText('Проверить стратегии'));
    fireEvent.click(await screen.findByText('Применить'));
    await waitFor(() => expect(apply).toHaveBeenCalledWith('opaque-token'));
  });

  it('does not offer Apply without a verified backend grant', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result({ apply_available: false, apply_token: undefined }));
    await openModal();
    fireEvent.click(screen.getByText('Проверить стратегии'));
    expect(await screen.findByText('Проверенная стратегия не получила разрешение на применение.')).toBeDefined();
    expect(screen.queryByText('Применить')).toBeNull();
  });

  it('shows managed strategy and reverts through the managed backend API', async () => {
    vi.spyOn(backendService, 'getAutoTuneVNextManagedStatus').mockResolvedValue(status({ state: 'VNEXT_MANAGED_ACTIVE', active: true, strategy_id: 'prod-tls-multisplit-1-v1', target: 'https://example.test/path' }));
    const revert = vi.spyOn(backendService, 'revertAutoTuneVNext').mockResolvedValue(status({ state: 'REVERTED' }));
    await openModal();
    expect(await screen.findByText('Стратегия применена')).toBeDefined();
    fireEvent.click(screen.getByText('Отключить / Вернуть прямое подключение'));
    await waitFor(() => expect(revert).toHaveBeenCalledTimes(1));
  });
});

describe('AutoTune cutover', () => {
  it('routes the primary action to vNext and retains explicit Legacy AutoTune fallback', () => {
    const legacy = vi.fn();
    const vnext = vi.fn();
    render(<MainControlView statusLedState="disconnected" isConnected={false} isConnecting={false} disableMain={false} toggleConnection={vi.fn()} selectedProfile="" setSelectedProfile={vi.fn()} sortedProfiles={[]} selectedEngine="zapret" handleToggleFavorite={vi.fn()} favoriteProfiles={[]} handleAutoTune={legacy} openAutoTuneVNext={vnext} vNextRunning={false} isScanning={false} scanProgress="" autotuneProgress={null} handleCancelAutoTune={vi.fn()} livePingData={{ active: false, latency: 0, status: 'disconnected' }} pingHistory={[]} managedState={{ state: 'DIRECT', active: false, needs_revalidation: false }} revertManaged={vi.fn()} />);
    fireEvent.click(screen.getByText('Автоподбор стратегии'));
    expect(vnext).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByText('Legacy AutoTune'));
    expect(legacy).toHaveBeenCalledTimes(1);
  });

  it('shows factual managed and dormant states on the main view', () => {
    const revert = vi.fn();
    const { rerender } = render(<MainControlView statusLedState="disconnected" isConnected={false} isConnecting={false} disableMain={false} toggleConnection={vi.fn()} selectedProfile="" setSelectedProfile={vi.fn()} sortedProfiles={[]} selectedEngine="zapret" handleToggleFavorite={vi.fn()} favoriteProfiles={[]} handleAutoTune={vi.fn()} openAutoTuneVNext={vi.fn()} vNextRunning={false} isScanning={false} scanProgress="" autotuneProgress={null} handleCancelAutoTune={vi.fn()} livePingData={{ active: true, latency: 0, status: 'managed_active' }} pingHistory={[]} managedState={{ state: 'VNEXT_MANAGED_ACTIVE', active: true, needs_revalidation: false, strategy_id: 'prod-tls-multisplit-1-v1', target: 'https://example.test/' }} revertManaged={revert} />);
    expect(screen.getByText('Управляемая стратегия активна')).toBeDefined();
    fireEvent.click(screen.getByText('Отключить управляемую стратегию'));
    expect(revert).toHaveBeenCalledTimes(1);
    rerender(<MainControlView statusLedState="disconnected" isConnected={false} isConnecting={false} disableMain={false} toggleConnection={vi.fn()} selectedProfile="" setSelectedProfile={vi.fn()} sortedProfiles={[]} selectedEngine="zapret" handleToggleFavorite={vi.fn()} favoriteProfiles={[]} handleAutoTune={vi.fn()} openAutoTuneVNext={vi.fn()} vNextRunning={false} isScanning={false} scanProgress="" autotuneProgress={null} handleCancelAutoTune={vi.fn()} livePingData={{ active: false, latency: 0, status: 'disconnected' }} pingHistory={[]} managedState={{ state: 'SAVED_NOT_CURRENTLY_NEEDED', active: false, needs_revalidation: true }} revertManaged={revert} />);
    expect(screen.getByText(/Сохранённая стратегия сейчас не требуется/)).toBeDefined();
  });
});
