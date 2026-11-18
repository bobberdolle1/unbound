import { WindowMinimise, WindowToggleMaximise, EventsOn } from '../../wailsjs/runtime/runtime';
import { HideWindowToTray, QuitApp, ShowNotification } from '../../wailsjs/go/main/App';

export interface AutotuneProgressData {
  msg?: string;
  percent?: number;
  progress?: number;
  step?: number;
  total?: number;
  profile?: string;
  okCount?: number;
  totalTargets?: number;
}

export interface AutoTuneVNextProgressData {
  stage: string;
  message: string;
  percent: number;
}

type WailsRuntimeWindow = {
  runtime?: {
    EventsOnMultiple?: (...args: unknown[]) => () => void;
    WindowMinimise?: () => void;
    WindowToggleMaximise?: () => void;
  };
  go?: {
    main?: {
      App?: Record<string, (...args: unknown[]) => Promise<unknown>>;
    };
  };
};

function getRuntimeWindow(): WailsRuntimeWindow | undefined {
  return typeof window !== 'undefined' ? (window as unknown as WailsRuntimeWindow) : undefined;
}

function safeEventsOn<T>(eventName: string, callback: (data: T) => void): () => void {
  const win = getRuntimeWindow();
  if (typeof win?.runtime?.EventsOnMultiple === 'function') {
    const unsub = EventsOn(eventName, callback);
    return typeof unsub === 'function' ? unsub : () => {};
  }
  return () => {};
}

export const windowService = {
  minimise: () => {
    const win = getRuntimeWindow();
    if (typeof win?.runtime?.WindowMinimise === 'function') {
      WindowMinimise();
    }
  },
  toggleMaximise: () => {
    const win = getRuntimeWindow();
    if (typeof win?.runtime?.WindowToggleMaximise === 'function') {
      WindowToggleMaximise();
    }
  },
  hideToTray: () => {
    const app = getRuntimeWindow()?.go?.main?.App;
    return typeof app?.HideWindowToTray === 'function' ? HideWindowToTray() : Promise.resolve();
  },
  quit: () => {
    const app = getRuntimeWindow()?.go?.main?.App;
    return typeof app?.QuitApp === 'function' ? QuitApp() : Promise.resolve();
  },
  showNotification: (title: string, message: string) => {
    const app = getRuntimeWindow()?.go?.main?.App;
    return typeof app?.ShowNotification === 'function' ? ShowNotification(title, message) : Promise.resolve();
  },
};

export const eventBus = {
  onStatusChanged: (callback: (status: string) => void) => safeEventsOn('status_changed', callback),
  onEnginesChanged: (callback: (engines: string[]) => void) => safeEventsOn('engines_changed', callback),
  onPrivilegeError: (callback: (msg: string) => void) => safeEventsOn('privilege_error', callback),
  onEngineError: (callback: (msg: string) => void) => safeEventsOn('engine_error', callback),
  onProfileChanged: (callback: (profile: string) => void) => safeEventsOn('profile_changed', callback),
  onProfileError: (callback: (msg: string) => void) => safeEventsOn('profile_error', callback),
  onNotification: (callback: (data: { type?: string; title?: string; message?: string }) => void) =>
    safeEventsOn('notification', callback),
  onAutotuneStart: (callback: (running: boolean) => void) => safeEventsOn('autotune_start', callback),
  onAutotuneProgress: (callback: (data: AutotuneProgressData) => void) => safeEventsOn('autotune_progress', callback),
  onAutotuneLog: (callback: (msg: string) => void) => safeEventsOn('autotune_log', callback),
  onEngineLog: (callback: (msg: string) => void) => safeEventsOn('engine_log', callback),
  onAutotuneComplete: (callback: (data: { success: boolean; profile?: string; error?: string }) => void) =>
    safeEventsOn('autotune_complete', callback),
  onOpenAutoTuneVNext: (callback: () => void) => safeEventsOn('open_autotune_vnext', callback),
  onAutoTuneVNextProgress: (callback: (data: AutoTuneVNextProgressData) => void) =>
    safeEventsOn('autotune_vnext_progress', callback),
  onDoctorStart: (callback: (data: { runId: string; mode: string; total: number }) => void) =>
    safeEventsOn('doctor_start', callback),
  onDoctorProgress: (
    callback: (data: {
      runId: string;
      completed: number;
      total: number;
      percent: number;
      running: string[];
      lastCompleted: string;
      elapsedMs: number;
    }) => void
  ) => safeEventsOn('doctor_progress', callback),
  onDoctorComplete: (callback: (data: { runId: string; result: unknown }) => void) =>
    safeEventsOn('doctor_complete', callback),
  onDoctorCancelled: (callback: (runId: string) => void) => safeEventsOn('doctor_cancelled', callback),
  onDoctorError: (callback: (data: { runId: string; error: string }) => void) => safeEventsOn('doctor_error', callback),
  onStrategyLabProgress: (callback: (data: unknown) => void) => safeEventsOn('strategy_lab_progress', callback),
};
