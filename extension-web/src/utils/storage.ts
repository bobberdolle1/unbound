import browser from 'webextension-polyfill';
import { STORAGE_KEYS, type ExtensionState, type Theme } from '../types';

const defaultState: ExtensionState = {
  mode: 'companion',
  status: 'disconnected',
  theme: 'modern-dark',
  bypassDomains: [],
};

export async function getState(): Promise<ExtensionState> {
  const result = await browser.storage.local.get([
    STORAGE_KEYS.MODE,
    STORAGE_KEYS.STATUS,
    STORAGE_KEYS.THEME,
    STORAGE_KEYS.BYPASS_DOMAINS,
    STORAGE_KEYS.PROXY_CONFIG,
  ]);

  return {
    mode: result[STORAGE_KEYS.MODE] ?? defaultState.mode,
    status: result[STORAGE_KEYS.STATUS] ?? defaultState.status,
    theme: result[STORAGE_KEYS.THEME] ?? defaultState.theme,
    bypassDomains: result[STORAGE_KEYS.BYPASS_DOMAINS] ?? defaultState.bypassDomains,
    proxyConfig: result[STORAGE_KEYS.PROXY_CONFIG],
  };
}

export async function setState(partial: Partial<ExtensionState>): Promise<void> {
  const updates: Record<string, unknown> = {};
  
  if (partial.mode !== undefined) updates[STORAGE_KEYS.MODE] = partial.mode;
  if (partial.status !== undefined) updates[STORAGE_KEYS.STATUS] = partial.status;
  if (partial.theme !== undefined) updates[STORAGE_KEYS.THEME] = partial.theme;
  if (partial.bypassDomains !== undefined) updates[STORAGE_KEYS.BYPASS_DOMAINS] = partial.bypassDomains;
  if (partial.proxyConfig !== undefined) updates[STORAGE_KEYS.PROXY_CONFIG] = partial.proxyConfig;

  await browser.storage.local.set(updates);
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  root.classList.remove('dark', 'light');
  
  if (theme === 'modern-dark') {
    root.classList.add('dark');
  } else {
    root.classList.add('light');
  }
}
