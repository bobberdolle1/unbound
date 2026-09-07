import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { StrategyLabModal } from '../components/modals/StrategyLabModal';
import { backendService } from '../services/backend';
import { engine } from '../../wailsjs/go/models';
describe('StrategyLabModal Tests', () => {
  beforeEach(() => {
    (window as unknown as { runtime: unknown }).runtime = {
      EventsOnMultiple: vi.fn().mockReturnValue(() => {}),
    };
  });

  it('renders StrategyLabModal with target inputs and presets', () => {
    render(<StrategyLabModal isOpen={true} onClose={vi.fn()} />);

    expect(screen.getByText(/Strategy Lab — Лаборатория поиска стратегий/i)).toBeDefined();
    expect(screen.getByText(/Целевой сервис:/i)).toBeDefined();
    expect(screen.getByText('YouTube')).toBeDefined();
    expect(screen.getByText('Discord')).toBeDefined();
    expect(screen.getByText('Steam')).toBeDefined();
    expect(screen.getByText('Начать поиск')).toBeDefined();
  });

  it('switches target domain when clicking presets', () => {
    render(<StrategyLabModal isOpen={true} onClose={vi.fn()} />);

    const discordBtn = screen.getByText('Discord');
    fireEvent.click(discordBtn);

    const input = screen.getByPlaceholderText('domain.com') as HTMLInputElement;
    expect(input.value).toBe('discord.com');

    const steamBtn = screen.getByText('Steam');
    fireEvent.click(steamBtn);
    expect(input.value).toBe('store.steampowered.com');
  });

  it('handles start discovery and displays results', async () => {
    const mockReport = {
      runId: 'test_lab_run',
      targetHost: 'youtube.com',
      targetIps: ['142.250.180.206'],
      protocol: 'TLS1.3',
      baselineReachable: false,
      totalCandidates: 2,
      testedCandidates: 2,
      workingCandidates: [
        {
          candidate: {
            id: 'cand_hostfake',
            name: 'HostFakeSplit (midsld)',
            protocol: 'TLS1.3',
            aggressiveness: 1,
            source: 'blockcheck2',
            zapret2Args: ['--lua-desync=hostfakesplit'],
          },
          status: 'PASS',
          passCount: 3,
          totalAttempts: 3,
          avgLatency: 55000000,
          score: 110,
          details: '3/3 успешно, латентность: 55мс',
        },
      ],
      bestCandidate: {
        candidate: {
          id: 'cand_hostfake',
          name: 'HostFakeSplit (midsld)',
          protocol: 'TLS1.3',
          aggressiveness: 1,
          source: 'blockcheck2',
          zapret2Args: ['--lua-desync=hostfakesplit'],
        },
        status: 'PASS',
        passCount: 3,
        totalAttempts: 3,
        avgLatency: 55000000,
        score: 110,
        details: '3/3 успешно, латентность: 55мс',
      },
      serviceVerified: true,
      duration: 1500000000,
      timestamp: '2026-09-07T12:00:00Z',
    };

    const reportPayload = mockReport as unknown as engine.StrategyLabReport;
    vi.spyOn(backendService, 'runStrategyLab').mockResolvedValue(reportPayload);

    render(<StrategyLabModal isOpen={true} onClose={vi.fn()} />);

    const startBtn = screen.getByText('Начать поиск');
    fireEvent.click(startBtn);

    expect(await screen.findByText(/Рекомендуемая стратегия/i)).toBeDefined();
    expect(screen.getAllByText('HostFakeSplit (midsld)').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('LOW').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/Комплексная проверка доступности сервиса подтверждена/i)).toBeDefined();
  });
});
