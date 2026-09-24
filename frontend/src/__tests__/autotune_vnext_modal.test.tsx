import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { AutoTuneVNextModal } from '../components/modals/AutoTuneVNextModal';
import { MainControlView } from '../components/views/MainControlView';
import { backendService } from '../services/backend';
import { main } from '../../wailsjs/go/models';

const config = new main.AutoTuneVNextExperimentConfig({
  targets: [
    { id: 'youtube-web', label: 'YouTube', target: 'https://www.youtube.com/generate_204' },
    { id: 'discord-api', label: 'Discord', target: 'https://discord.com/api/v10/gateway' },
    { id: 'steam-store', label: 'Steam', target: 'https://store.steampowered.com/' },
  ],
  default_control: { id: 'cloudflare-control', label: 'Cloudflare', target: 'https://cloudflare.com/cdn-cgi/trace' },
});

const result = (status: string, overrides: Record<string, unknown> = {}) => new main.AutoTuneVNextResult({
  status,
  state_restored: true,
  target: 'https://www.youtube.com/generate_204',
  planner_disposition: 'ELIGIBLE',
  catalog_status: 'READY',
  ...overrides,
});

async function openModal() {
  render(<AutoTuneVNextModal isOpen={true} onClose={vi.fn()} onRunningChange={vi.fn()} />);
  await screen.findByText('YouTube');
}

describe('AutoTune vNext experimental modal', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(backendService, 'getAutoTuneVNextExperimentConfig').mockResolvedValue(config);
    vi.spyOn(backendService, 'cancelAutoTune').mockResolvedValue(undefined);
  });

  it('is only rendered when explicitly opened', () => {
    render(<AutoTuneVNextModal isOpen={false} onClose={vi.fn()} onRunningChange={vi.fn()} />);
    expect(screen.queryByText('AutoTune vNext (эксперимент)')).toBeNull();
  });

  it('uses the product preset ID and shows the product default control', async () => {
    const run = vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result('COMPLETED_NO_ACTION_NEEDED'));
    await openModal();
    expect(screen.getByText('Защищённый контроль: https://cloudflare.com/cdn-cgi/trace')).toBeDefined();
    fireEvent.click(screen.getByText('Discord'));
    fireEvent.click(screen.getByText('Начать экспериментальную проверку'));
    await waitFor(() => expect(run).toHaveBeenCalledWith('discord-api', ''));
  });

  it('shows a running state and sends cancellation to the backend before waiting for terminal result', async () => {
    const pending = (Promise as typeof Promise & {
      withResolvers<T>(): { promise: Promise<T>; resolve: (value: T | PromiseLike<T>) => void; reject: (reason?: unknown) => void };
    }).withResolvers<main.AutoTuneVNextResult>();
    const run = vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockReturnValue(pending.promise);
    const cancel = vi.spyOn(backendService, 'cancelAutoTune').mockResolvedValue(undefined);
    await openModal();
    fireEvent.click(screen.getByText('Начать экспериментальную проверку'));
    expect(await screen.findByText('Проверяем прямое соединение...')).toBeDefined();
    fireEvent.click(screen.getByText('Отменить проверку'));
    await waitFor(() => expect(cancel).toHaveBeenCalledTimes(1));
    expect(run).toHaveBeenCalledWith('youtube-web', '');
    pending.resolve(result('CANCELLED'));
    expect(await screen.findByText('Проверка отменена.')).toBeDefined();
  });

  it.each([
    ['COMPLETED_NO_ACTION_NEEDED', 'Прямое соединение работает. Изменения не требуются.'],
    ['COMPLETED_NO_ELIGIBLE_CANDIDATES', 'Для обнаруженного типа сбоя нет подходящей стратегии.'],
    ['COMPLETED_NO_VERIFIED_CANDIDATE', 'Проверенные стратегии не восстановили соединение.'],
    ['PREFLIGHT_FAILED', 'Среда не готова к безопасной проверке.'],
    ['INCONCLUSIVE', 'Недостаточно данных для вывода.'],
    ['LIFECYCLE_FAILED', 'Ошибка во время тестирования стратегии.'],
  ])('presents %s factually', async (status, message) => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result(status));
    await openModal();
    fireEvent.click(screen.getByText('Начать экспериментальную проверку'));
    expect(await screen.findByText(message)).toBeDefined();
    expect(screen.getByText('Исходное состояние')).toBeDefined();
    expect(screen.getByText('восстановлено')).toBeDefined();
  });

  it('presents a selected strategy as verified and restored, never activated', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result('COMPLETED_SELECTED', {
      selected_strategy_id: 'prod-tls-multisplit-overlap-v1',
      selected_fingerprint: '0123456789abcdef',
      candidate_outcomes: [{ strategy_id: 'prod-tls-multisplit-overlap-v1', fingerprint: '0123456789abcdef', planner_status: 'ELIGIBLE', outcome: 'VERIFIED_FIXED' }],
    }));
    await openModal();
    fireEvent.click(screen.getByText('Начать экспериментальную проверку'));
    expect(await screen.findByText('Найдена проверенная стратегия. После тестирования исходное состояние восстановлено.')).toBeDefined();
    expect(screen.getByText('TLS Split + overlap')).toBeDefined();
    expect(screen.queryByText('Стратегия включена')).toBeNull();
    expect(screen.queryByText('Обход активирован')).toBeNull();
  });

  it('makes state restoration failure prominent and factual', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result('STATE_RESTORE_FAILED', { state_restored: false }));
    await openModal();
    fireEvent.click(screen.getByText('Начать экспериментальную проверку'));
    expect(await screen.findByText(/Критично: Не удалось подтвердить восстановление исходного состояния/)).toBeDefined();
    expect(screen.getByText('не подтверждено')).toBeDefined();
  });

  it('redacts secret target parts and does not render raw engine argv', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result('COMPLETED_NO_ACTION_NEEDED', {
      target: 'https://example.test/path?token=secret#fragment',
      candidate_outcomes: [{ strategy_id: 'prod-tls-multisplit-1-v1', fingerprint: 'abcdef0123456789', planner_status: 'ELIGIBLE', outcome: 'NOT_RUN', engine_argv: ['--wf-tcp=443'] }],
    }));
    await openModal();
    fireEvent.click(screen.getByText('Начать экспериментальную проверку'));
    expect(await screen.findByText('https://example.test/path')).toBeDefined();
    expect(screen.queryByText(/token=secret/)).toBeNull();
    expect(screen.queryByText(/--wf-tcp=443/)).toBeNull();
  });
});

describe('Legacy AutoTune remains the primary action', () => {
  it('keeps the primary button on legacy AutoTune and exposes vNext separately', () => {
    const legacy = vi.fn();
    const experimental = vi.fn();
    render(
      <MainControlView
        statusLedState="disconnected"
        isConnected={false}
        isConnecting={false}
        disableMain={false}
        toggleConnection={vi.fn()}
        selectedProfile=""
        setSelectedProfile={vi.fn()}
        sortedProfiles={[]}
        selectedEngine="zapret"
        handleToggleFavorite={vi.fn()}
        favoriteProfiles={[]}
        handleAutoTune={legacy}
        openAutoTuneVNext={experimental}
        vNextRunning={false}
        isScanning={false}
        scanProgress=""
        autotuneProgress={null}
        handleCancelAutoTune={vi.fn()}
        livePingData={{ active: false, latency: 0, status: 'disconnected' }}
        pingHistory={[]}
      />
    );
    fireEvent.click(screen.getByText('Автоподбор стратегии'));
    expect(legacy).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByText('AutoTune vNext (эксперимент)'));
    expect(experimental).toHaveBeenCalledTimes(1);
  });
});
