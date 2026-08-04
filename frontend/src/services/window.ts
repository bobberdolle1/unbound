import { WindowMinimise, WindowToggleMaximise } from '../../wailsjs/runtime/runtime';
import { HideWindowToTray, QuitApp, ShowNotification } from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

export const windowService = {
  minimise: () => WindowMinimise(),
  toggleMaximise: () => WindowToggleMaximise(),
  hideToTray: () => HideWindowToTray(),
  quit: () => QuitApp(),
  showNotification: (title: string, message: string) => ShowNotification(title, message),
};

export const eventBus = {
  onStatusChanged: (callback: (status: string) => void) => EventsOn('status_changed', callback),
  onEnginesChanged: (callback: (engines: string[]) => void) => EventsOn('engines_changed', callback),
  onPrivilegeError: (callback: (msg: string) => void) => EventsOn('privilege_error', callback),
  onEngineError: (callback: (msg: string) => void) => EventsOn('engine_error', callback),
  onNotification: (callback: (data: { type?: string; title?: string; message?: string }) => void) => EventsOn('notification', callback),
  onAutotuneStart: (callback: (running: boolean) => void) => EventsOn('autotune_start', callback),
  onAutotuneProgress: (callback: (data: { msg?: string; progress?: number }) => void) => EventsOn('autotune_progress', callback),
  onAutotuneLog: (callback: (msg: string) => void) => EventsOn('autotune_log', callback),
  onEngineLog: (callback: (msg: string) => void) => EventsOn('engine_log', callback),
  onAutotuneComplete: (callback: (data: { success: boolean; profile?: string; error?: string }) => void) =>
    EventsOn('autotune_complete', callback),
};
