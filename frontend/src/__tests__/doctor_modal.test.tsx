import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { DiagnosticsModal } from '../components/modals/DiagnosticsModal';
import { DiscordConfirmModal } from '../components/modals/DiscordConfirmModal';
import { engine } from '../../wailsjs/go/models';
import { backendService } from '../services/backend';
describe('Doctor & Diagnostics Modal Tests', () => {
  const mockDoctorResult = new engine.DoctorResult({
    overallStatus: 'HEALTHY',
    mode: 'quick',
    timestamp: '2026-09-04T12:00:00Z',
    duration: 1200000000,
    appVersion: '0.5.0',
    engineVersion: '1.0.5',
    os: 'windows',
    arch: 'amd64',
    activeProfile: 'Recommended (hostfakesplit)',
    passCount: 5,
    failCount: 0,
    warnCount: 0,
    notVerCount: 1,
    infoCount: 1,
    groups: [
      new engine.DiagnosticGroup({
        id: 'group_youtube',
        name: 'Сервисы YouTube',
        status: 'PASS',
        summary: '1 доступно',
        probes: [
          new engine.ProbeResult({
            id: 'yt_web',
            service: 'YouTube',
            name: 'YouTube Web Frontend',
            target: 'https://www.youtube.com',
            status: 'PASS',
            latency: 45000000,
            details: 'HTTP 204 OK',
          }),
        ],
      }),
      new engine.DiagnosticGroup({
        id: 'group_discord',
        name: 'Сервисы Discord',
        status: 'NOT_VERIFIED',
        summary: '1 ручная проверка',
        probes: [
          new engine.ProbeResult({
            id: 'discord_voice_manual',
            service: 'Discord',
            name: 'Active Discord Voice Call',
            target: 'Voice server',
            status: 'NOT_VERIFIED',
            details: 'Manual check recommended',
            isManualCheck: true,
          }),
        ],
      }),
    ],
    manualItems: ['Discord voice call test'],
  });
  beforeEach(() => {
    (window as unknown as { runtime: unknown }).runtime = {
      EventsOnMultiple: vi.fn().mockReturnValue(() => {}),
    };
    vi.spyOn(backendService, 'startDoctor').mockResolvedValue({
      runId: 'test_run',
      mode: 'extended',
      total: 24,
      startedAt: '2026-09-04T12:00:00Z',
    } as unknown as engine.DoctorRunStart);
  });

  it('renders overall health banner and group names', () => {
    render(
      <DiagnosticsModal
        isOpen={true}
        isRunning={false}
        results={[]}
        doctorResult={mockDoctorResult}
        onClose={vi.fn()}
      />
    );

    expect(screen.getByText(/Проверка системы/i)).toBeDefined();
    expect(screen.getByText(/Все системы и целевые сервисы доступны/i)).toBeDefined();
    expect(screen.getByText(/Сервисы YouTube/i)).toBeDefined();
    expect(screen.getByText(/YouTube Web Frontend/i)).toBeDefined();
    expect(screen.getAllByText(/✓ PASS/i).length).toBeGreaterThan(0);
  });

  it('allows expanding probe details on click', () => {
    render(
      <DiagnosticsModal
        isOpen={true}
        isRunning={false}
        results={[]}
        doctorResult={mockDoctorResult}
        onClose={vi.fn()}
      />
    );

    const probeTitle = screen.getByText('YouTube Web Frontend');
    fireEvent.click(probeTitle);

    expect(screen.getByText(/https:\/\/www.youtube.com/i)).toBeDefined();
  });

  it('renders mode switch tabs and switches active tab', () => {
    const onRunMode = vi.fn();
    render(
      <DiagnosticsModal
        isOpen={true}
        isRunning={false}
        results={[]}
        doctorResult={mockDoctorResult}
        onClose={vi.fn()}
        onRunMode={onRunMode}
      />
    );

    const extTab = screen.getByText('Расширенная проверка');
    fireEvent.click(extTab);
    expect(onRunMode).toHaveBeenCalledWith('extended');
  });

  it('renders manual checklist items in extended mode', () => {
    render(
      <DiagnosticsModal
        isOpen={true}
        isRunning={false}
        results={[]}
        doctorResult={mockDoctorResult}
        onClose={vi.fn()}
      />
    );

    expect(screen.getByText(/Рекомендуемый чек-лист ручной приёмки/i)).toBeDefined();
    expect(screen.getByText(/Discord voice call test/i)).toBeDefined();
  });

  describe('DiscordConfirmModal', () => {
    it('renders warning and handles confirm and cancel callbacks', () => {
      const onCancel = vi.fn();
      const onConfirm = vi.fn();

      render(
        <DiscordConfirmModal
          isOpen={true}
          runningProcesses={['Discord.exe']}
          onCancel={onCancel}
          onConfirm={onConfirm}
        />
      );

      expect(screen.getByText(/Очистить кэш Discord\?/i)).toBeDefined();
      expect(screen.getByText(/Discord\.exe/i)).toBeDefined();

      const cancelBtn = screen.getByText('Отмена');
      fireEvent.click(cancelBtn);
      expect(onCancel).toHaveBeenCalledTimes(1);

      const confirmBtn = screen.getByText('Закрыть и очистить');
      fireEvent.click(confirmBtn);
      expect(onConfirm).toHaveBeenCalledTimes(1);
    });
  });
});
