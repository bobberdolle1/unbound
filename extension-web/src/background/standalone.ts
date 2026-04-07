// Standalone mode - handles PAC script generation and proxy management
// This module manages proxy routing without external dependencies

import browser from 'webextension-polyfill';
import { type ProxyConfig } from '../types';
import { generatePacScript } from '../utils/proxy';

let isProxyActive = false;
let currentDomains: string[] = [];
let currentProxyConfig: ProxyConfig | null = null;

/**
 * Enable proxy with PAC script for specified domains
 */
export async function enableProxy(domains: string[], proxyConfig: ProxyConfig): Promise<void> {
  if (domains.length === 0) {
    throw new Error('At least one domain is required');
  }

  console.log('[Standalone] Enabling proxy for domains:', domains);

  const pacScript = generatePacScript(domains, proxyConfig);
  
  try {
    await browser.proxy.settings.set({
      value: {
        mode: 'pac_script',
        pacScript: {
          data: pacScript,
          mandatory: false,
        },
      },
      scope: 'regular',
    });

    isProxyActive = true;
    currentDomains = domains;
    currentProxyConfig = proxyConfig;

    console.log('[Standalone] Proxy enabled successfully');
  } catch (error) {
    console.error('[Standalone] Failed to enable proxy:', error);
    throw error;
  }
}

/**
 * Disable proxy and revert to direct connection
 */
export async function disableProxy(): Promise<void> {
  if (!isProxyActive) {
    console.log('[Standalone] Proxy not active, nothing to disable');
    return;
  }

  console.log('[Standalone] Disabling proxy');

  try {
    await browser.proxy.settings.set({
      value: {
        mode: 'direct',
      },
      scope: 'regular',
    });

    isProxyActive = false;
    currentDomains = [];
    currentProxyConfig = null;

    console.log('[Standalone] Proxy disabled');
  } catch (error) {
    console.error('[Standalone] Failed to disable proxy:', error);
    throw error;
  }
}

/**
 * Update domains in the PAC script (re-applies proxy settings)
 */
export async function updateDomains(domains: string[]): Promise<void> {
  if (!isProxyActive || !currentProxyConfig) {
    console.warn('[Standalone] Proxy not active, cannot update domains');
    return;
  }

  console.log('[Standalone] Updating domains:', domains);
  currentDomains = domains;

  await enableProxy(domains, currentProxyConfig);
}

/**
 * Update proxy configuration (re-applies PAC script)
 */
export async function updateProxyConfig(proxyConfig: ProxyConfig): Promise<void> {
  currentProxyConfig = proxyConfig;

  if (isProxyActive && currentDomains.length > 0) {
    await enableProxy(currentDomains, proxyConfig);
  }
}

/**
 * Get current proxy status
 */
export function getStatus(): {
  isActive: boolean;
  domains: string[];
  proxyConfig: ProxyConfig | null;
} {
  return {
    isActive: isProxyActive,
    domains: currentDomains,
    proxyConfig: currentProxyConfig,
  };
}

/**
 * Validate proxy configuration
 */
export function validateProxyConfig(config: ProxyConfig): string | null {
  if (!config.host || config.host.trim() === '') {
    return 'Host is required';
  }

  if (!config.port || config.port < 1 || config.port > 65535) {
    return 'Port must be between 1 and 65535';
  }

  if (!['https', 'socks5'].includes(config.protocol)) {
    return 'Protocol must be HTTPS or SOCKS5';
  }

  return null;
}

/**
 * Test proxy connectivity (optional enhancement)
 * Note: This would require making a test request through the proxy
 * For now, it's a placeholder for future implementation
 */
export async function testProxyConnectivity(config: ProxyConfig): Promise<boolean> {
  console.log('[Standalone] Testing proxy connectivity:', config);
  
  // In a real implementation, you would:
  // 1. Enable the proxy temporarily
  // 2. Make a test request to a known endpoint
  // 3. Check if the request succeeded
  // 4. Restore previous proxy state
  
  // For now, just validate the config
  const error = validateProxyConfig(config);
  if (error) {
    console.error('[Standalone] Proxy config validation failed:', error);
    return false;
  }

  // Assume valid config means connectivity is possible
  return true;
}
