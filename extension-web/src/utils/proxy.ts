import browser from 'webextension-polyfill';
import type { ProxyConfig } from '../types';

/**
 * Generate a PAC (Proxy Auto-Configuration) script that routes
 * specific domains through a proxy while leaving other traffic direct.
 */
export function generatePacScript(domains: string[], proxy: ProxyConfig): string {
  const proxyStr = proxy.protocol === 'socks5'
    ? `SOCKS5 ${proxy.host}:${proxy.port}`
    : `PROXY ${proxy.host}:${proxy.port}`;

  // Convert domain patterns to PAC-compatible conditions
  const domainConditions = domains.map(domain => {
    if (domain.startsWith('*.')) {
      const suffix = domain.slice(2);
      return `dnsDomainIs(host, '.${suffix}')`;
    }
    return `host === '${domain}' || dnsDomainIs(host, '.${domain}')`;
  });

  const conditions = domainConditions.join(' ||\n    ');

  return `
function FindProxyForURL(url, host) {
  // Unbound Web - Dynamic PAC script
  // Route specified domains through proxy
  
  if (${conditions}) {
    return "${proxyStr}";
  }
  
  // All other traffic goes direct
  return "DIRECT";
}
  `.trim();
}

/**
 * Apply the PAC script to the browser's proxy settings.
 */
export async function enableProxyWithPac(pacScript: string): Promise<void> {
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
}

/**
 * Disable the proxy and revert to direct connection.
 */
export async function disableProxy(): Promise<void> {
  await browser.proxy.settings.set({
    value: {
      mode: 'direct',
    },
    scope: 'regular',
  });
}

/**
 * Validate domain format (basic check)
 */
export function isValidDomain(domain: string): boolean {
  const cleaned = domain.replace(/^\*\./, '');
  return /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z]{2,})+$/.test(cleaned);
}
