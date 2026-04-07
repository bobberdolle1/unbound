// Extension state types
export type ExtensionMode = 'companion' | 'standalone';
export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error';
export type Theme = 'doodle-light' | 'modern-dark';

export interface ExtensionState {
  mode: ExtensionMode;
  status: ConnectionStatus;
  theme: Theme;
  bypassDomains: string[];
  proxyConfig?: ProxyConfig;
}

export interface ProxyConfig {
  protocol: 'https' | 'socks5';
  host: string;
  port: number;
}

// Native messaging types
export interface NativeMessage {
  command: 'start' | 'stop' | 'status' | 'update_domains';
  domains?: string[];
  proxy?: ProxyConfig;
}

export interface NativeResponse {
  status: 'success' | 'error' | 'running' | 'stopped';
  message?: string;
  version?: string;
}

// Storage keys
export const STORAGE_KEYS = {
  MODE: 'unbound_mode',
  STATUS: 'unbound_status',
  THEME: 'unbound_theme',
  BYPASS_DOMAINS: 'unbound_bypass_domains',
  PROXY_CONFIG: 'unbound_proxy_config',
} as const;
