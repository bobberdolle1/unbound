import { describe, it, expect, vi, beforeEach } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { AutoTuneVNextModal } from '../components/modals/AutoTuneVNextModal';
import { MainControlView } from '../components/views/MainControlView';
import { backendService } from '../services/backend';
import { AutoTuneVNextProgressData, eventBus } from '../services/window';
import { main } from '../../wailsjs/go/models';

const config = new main.AutoTuneVNextExperimentConfig({
  targets: [{ id: 'youtube-web', label: 'YouTube', target: 'https://www.youtube.com/generate_204' }],
  default_control: { id: 'cloudflare-control', label: 'Cloudflare', target: 'https://cloudflare.com/cdn-cgi/trace' },
});
const status = (overrides: Record<string, unknown> = {}) =>
  new main.AutoTuneVNextManagedStatus({ state: 'DIRECT', active: false, needs_revalidation: false, ...overrides });
const result = (overrides: Record<string, unknown> = {}) =>
  new main.AutoTuneVNextResult({
    status: 'COMPLETED_SELECTED',
    state_restored: true,
    target: 'https://www.youtube.com/generate_204',
    catalog_status: 'READY',
    selected_strategy_id: 'prod-tls-multisplit-1-v1',
    selected_fingerprint: '0123456789abcdef',
    apply_available: true,
    apply_token: 'opaque-token',
    ...overrides,
  });

async function openModal(platform: string = 'windows', onRunningChange = vi.fn()) {
  render(<AutoTuneVNextModal isOpen={true} onClose={vi.fn()} onRunningChange={onRunningChange} platform={platform} />);
  await screen.findByRole('dialog');
  await waitFor(() => expect(backendService.getAutoTuneVNextExperimentConfig).toHaveBeenCalled());
}

describe('primary AutoTune vNext modal', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(backendService, 'getAutoTuneVNextExperimentConfig').mockResolvedValue(config);
    vi.spyOn(backendService, 'getAutoTuneVNextManagedStatus').mockResolvedValue(status());
    vi.spyOn(backendService, 'getAutoTuneVNextHistorySummary').mockResolvedValue({
      record_count: 3,
      verified_count: 2,
      negative_count: 1,
      updated_at: '2026-11-18T00:00:00Z',
      context_ready: true,
    });
    vi.spyOn(backendService, 'getAutoTuneVNextServicePresets').mockResolvedValue([
      {
        id: 'youtube',
        label: 'YouTube',
        description: 'Веб-интерфейс, API и медиа-узлы YouTube',
        hosts: ['www.youtube.com', 'youtubei.googleapis.com', 'i.ytimg.com'],
      },
      {
        id: 'discord',
        label: 'Discord',
        description: 'Основной домен, шлюз и CDN Discord',
        hosts: ['discord.com', 'gateway.discord.gg', 'cdn.discordapp.com'],
      },
    ]);
    vi.spyOn(backendService, 'cancelAutoTuneVNext').mockResolvedValue(true);
    vi.spyOn(backendService, 'clearAutoTuneVNextHistory').mockResolvedValue(undefined);
  });

  it('uses product-owned presets and contains no experimental wording', async () => {
    const run = vi
      .spyOn(backendService, 'runExperimentalAutoTuneVNext')
      .mockResolvedValue(result({ status: 'COMPLETED_NO_ACTION_NEEDED', apply_available: false }));
    await openModal();
    expect(screen.getByText('Автоподбор стратегии')).toBeDefined();
    expect(screen.queryByText(/эксперимент/i)).toBeNull();
    fireEvent.click(await screen.findByText('Проверить стратегии'));
    await waitFor(() => expect(run).toHaveBeenCalledWith('youtube-web', ''));
  });

  it('offers Apply only for backend-issued verified selection and never accepts strategy parameters', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(result());
    const apply = vi
      .spyOn(backendService, 'applyAutoTuneVNextSelection')
      .mockResolvedValue(
        status({
          state: 'APPLIED',
          active: true,
          strategy_id: 'prod-tls-multisplit-1-v1',
          target: 'https://www.youtube.com/generate_204',
        })
      );
    await openModal();
    fireEvent.click(await screen.findByText('Проверить стратегии'));
    fireEvent.click(await screen.findByText('Применить'));
    await waitFor(() => expect(apply).toHaveBeenCalledWith('opaque-token'));
  });

  it('does not offer Apply without a verified backend grant', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(
      result({ apply_available: false, apply_token: undefined })
    );
    await openModal();
    fireEvent.click(await screen.findByText('Проверить стратегии'));
    expect(await screen.findByText('Проверенная стратегия не получила разрешение на применение.')).toBeDefined();
    expect(screen.queryByText('Применить')).toBeNull();
  });

  it('reports readable Russian diagnosis, candidate outcomes, limitations, lifecycle errors, and history influence', async () => {
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockResolvedValue(
      result({
        status: 'COMPLETED_NO_VERIFIED_CANDIDATE',
        apply_available: false,
        diagnosis: { kind: 'TLS_HANDSHAKE_PATH_FAILURE', confidence: 'MEDIUM' },
        recommendation: { history_used: true, reason_codes: ['COMPATIBLE_RECENT_NEGATIVE'] },
        candidate_outcomes: [
          { strategy_id: 'prod-tls-multisplit-1-v1', outcome: 'STILL_FAILING' },
          { strategy_id: 'prod-tls-hostfakesplit-v1', outcome: 'REGRESSION_OBSERVED' },
        ],
        limitations: ['SAME_EDGE_REQUIRED'],
        lifecycle_errors: [{ code: 'ACTIVE_VERIFICATION_FAILED' }],
      })
    );
    await openModal();
    fireEvent.click(await screen.findByText('Проверить стратегии'));
    expect(await screen.findByText(/TLS_HANDSHAKE_PATH_FAILURE/)).toBeDefined();
    expect(screen.getByText(/Сбой TLS-рукопожатия/i)).toBeDefined();
    expect(screen.getByText(/Сбой сохраняется/i)).toBeDefined();
    expect(screen.getByText(/ухудшение контрольного соединения/i)).toBeDefined();
    expect(screen.getByText(/сменился IP-адрес CDN-узла/i)).toBeDefined();
    expect(screen.getByText(/Процесс стратегии завершился преждевременно/i)).toBeDefined();
    expect(screen.getByText(/предыдущий совместимый результат/i)).toBeDefined();
    expect(screen.getByText(/не является гарантией/i)).toBeDefined();
  });

  it('shows managed strategy, disables controls while reverting, and reverts through managed API', async () => {
    let resolveRevert!: (val: main.AutoTuneVNextManagedStatus) => void;
    const revertPromise = new Promise<main.AutoTuneVNextManagedStatus>((r) => {
      resolveRevert = r;
    });
    vi.spyOn(backendService, 'getAutoTuneVNextManagedStatus').mockResolvedValue(
      status({ state: 'VNEXT_MANAGED_ACTIVE', active: true, strategy_id: 'prod-tls-multisplit-1-v1', target: 'https://example.test/path' })
    );
    const revert = vi.spyOn(backendService, 'revertAutoTuneVNext').mockReturnValueOnce(revertPromise);
    const onRunningChange = vi.fn();
    await openModal('windows', onRunningChange);

    const revertBtn = (await screen.findByText('Отключить / Вернуть прямое подключение')) as HTMLButtonElement;
    fireEvent.click(revertBtn);

    expect(onRunningChange).toHaveBeenCalledWith(true);
    expect(revertBtn.disabled).toBe(true);

    vi.spyOn(backendService, 'getAutoTuneVNextManagedStatus').mockResolvedValue(status({ state: 'REVERTED' }));
    await act(async () => {
      resolveRevert(status({ state: 'REVERTED' }));
    });
    await waitFor(() => expect(revert).toHaveBeenCalledTimes(1));
    expect(onRunningChange).toHaveBeenLastCalledWith(false);
  });

  it('streams live progress from autotune_vnext_progress and cancels via CancelAutoTuneVNext', async () => {
    let progressCb: ((p: AutoTuneVNextProgressData) => void) | undefined;
    vi.spyOn(eventBus, 'onAutoTuneVNextProgress').mockImplementation((cb) => {
      progressCb = cb;
      return () => {};
    });

    let resolveRun!: (val: main.AutoTuneVNextResult) => void;
    vi.spyOn(backendService, 'runExperimentalAutoTuneVNext').mockReturnValue(
      new Promise<main.AutoTuneVNextResult>((r) => {
        resolveRun = r;
      })
    );

    await openModal();
    fireEvent.click(await screen.findByText('Проверить стратегии'));

    await act(async () => {
      progressCb?.({ stage: 'planning', message: 'Проверка кандидата TLS Split...', percent: 55 });
    });

    expect(screen.getByText('Отбор и проверка стратегий')).toBeDefined();
    expect(screen.getByText('Проверка кандидата TLS Split...')).toBeDefined();
    expect(screen.getByText('55%')).toBeDefined();

    fireEvent.click(screen.getByText('Отменить'));
    await waitFor(() => expect(backendService.cancelAutoTuneVNext).toHaveBeenCalledTimes(1));

    await act(async () => {
      resolveRun(result({ status: 'CANCELLED', apply_available: false }));
    });
    expect(await screen.findByText('Проверка отменена.')).toBeDefined();
  });

  it('runs multi-host ServiceGraph presets and supports reverting ServiceGraph', async () => {
    const runGraph = vi.spyOn(backendService, 'autoTuneVNextServiceGraph').mockResolvedValue({
      status: 'APPLIED',
      preset_id: 'youtube',
      hosts: ['www.youtube.com', 'youtubei.googleapis.com', 'i.ytimg.com'],
      verified_hosts: ['www.youtube.com', 'youtubei.googleapis.com'],
      applied: true,
      summary: 'Пакетная стратегия проверена и применена для YouTube.',
      errors: [],
    });
    const revertGraph = vi.spyOn(backendService, 'revertAutoTuneVNextServiceGraph').mockResolvedValue(undefined);

    await openModal();
    fireEvent.click(screen.getByText('Мульти-хост (ServiceGraph)'));
    expect(screen.getByText('www.youtube.com')).toBeDefined();

    fireEvent.click(screen.getByText('Запустить подбор ServiceGraph'));
    await waitFor(() => expect(runGraph).toHaveBeenCalledWith('youtube'));

    expect(await screen.findByText('Пакетная стратегия проверена и применена для YouTube.')).toBeDefined();
    fireEvent.click(screen.getByText('Отключить ServiceGraph'));
    await waitFor(() => expect(revertGraph).toHaveBeenCalledTimes(1));
  });

  it('displays History Ledger stats, clears history, and renders macOS tpws guidance banner', async () => {
    await openModal('darwin');
    expect(screen.getByText(/режим проксирования tpws/i)).toBeDefined();
    expect(screen.getByText(/Strategy Lab/i)).toBeDefined();
    expect(screen.getByText(/Записей: 3 · Успешных: 2 · Неудачных: 1/i)).toBeDefined();

    vi.spyOn(backendService, 'getAutoTuneVNextHistorySummary').mockResolvedValueOnce({
      record_count: 0,
      verified_count: 0,
      negative_count: 0,
      updated_at: '',
      context_ready: false,
    });
    fireEvent.click(screen.getByText('Очистить историю'));
    await waitFor(() => expect(backendService.clearAutoTuneVNextHistory).toHaveBeenCalledTimes(1));
    expect(await screen.findByText(/Записей: 0/)).toBeDefined();
  });
});

describe('AutoTune cutover & MainControlView managed states', () => {
  const baseProps = {
    statusLedState: 'disconnected' as const,
    isConnected: false,
    isConnecting: false,
    disableMain: false,
    toggleConnection: vi.fn(),
    selectedProfile: '',
    setSelectedProfile: vi.fn(),
    sortedProfiles: [],
    selectedEngine: 'zapret',
    handleToggleFavorite: vi.fn(),
    favoriteProfiles: [],
    handleAutoTune: vi.fn(),
    openAutoTuneVNext: vi.fn(),
    vNextRunning: false,
    isScanning: false,
    scanProgress: '',
    autotuneProgress: null,
    handleCancelAutoTune: vi.fn(),
    livePingData: { active: false, latency: 0, status: 'disconnected' },
    pingHistory: [],
  };

  it('routes the primary action to vNext and retains explicit Legacy AutoTune fallback', () => {
    const legacy = vi.fn();
    const vnext = vi.fn();
    render(
      <MainControlView
        {...baseProps}
        handleAutoTune={legacy}
        openAutoTuneVNext={vnext}
        managedState={{ state: 'DIRECT', active: false, needs_revalidation: false }}
        revertManaged={vi.fn()}
      />
    );
    fireEvent.click(screen.getByText('Автоподбор стратегии'));
    expect(vnext).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByText('Legacy AutoTune'));
    expect(legacy).toHaveBeenCalledTimes(1);
  });

  it('shows factual banners and actions for all managed, dormant, stale, pending, and error states', () => {
    const revert = vi.fn();
    const openVNext = vi.fn();
    const { rerender } = render(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{
          state: 'VNEXT_MANAGED_ACTIVE',
          active: true,
          needs_revalidation: false,
          strategy_id: 'prod-tls-multisplit-1-v1',
          target: 'https://example.test/',
        }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText('Управляемая стратегия активна')).toBeDefined();
    fireEvent.click(screen.getByText('Отключить управляемую стратегию'));
    expect(revert).toHaveBeenCalledTimes(1);

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'SAVED_NOT_CURRENTLY_NEEDED', active: false, needs_revalidation: true }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText(/Сохранённая стратегия сейчас не требуется/)).toBeDefined();

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'SAVED_REVALIDATION_PENDING', active: false, needs_revalidation: true, target: 'https://youtube.com/' }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText(/ожидает повторной проверки перед включением/i)).toBeDefined();
    fireEvent.click(screen.getByText('Проверить и применить'));
    expect(openVNext).toHaveBeenCalledTimes(1);

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'SAVED_STRATEGY_STALE', active: false, needs_revalidation: true }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText(/устарела или не совпадает с текущим каталогом/i)).toBeDefined();

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'NO_VERIFIED_STRATEGY', active: false, needs_revalidation: true }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText(/не подтвердила работоспособность/i)).toBeDefined();

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'PERSISTENCE_UPDATE_FAILED', active: false, needs_revalidation: false }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText(/Не удалось обновить файл состояния/i)).toBeDefined();

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'SERVICE_SCOPE_CHANGED_REVALIDATION_REQUIRED', active: true, needs_revalidation: true }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText(/Требуется повторная проверка управляемой стратегии/i)).toBeDefined();

    rerender(
      <MainControlView
        {...baseProps}
        openAutoTuneVNext={openVNext}
        managedState={{ state: 'STATE_RESTORE_FAILED', active: false, needs_revalidation: false }}
        revertManaged={revert}
      />
    );
    expect(screen.getByText('Не подтверждено восстановление исходного состояния')).toBeDefined();
    expect(screen.queryByText('Управляемая стратегия активна')).toBeNull();
    fireEvent.click(screen.getByText('Повторить восстановление'));
    expect(revert).toHaveBeenCalledTimes(2);
  });
});
