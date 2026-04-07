import type { ManifestV3 } from '@crxjs/vite-plugin';

export const firefoxManifest: ManifestV3 = {
  manifest_version: 3,
  name: 'Unbound Web',
  version: '1.0.0',
  description: 'Bypass DPI and censorship with intelligent routing',
  icons: {
    '16': 'icons/icon16.svg',
    '32': 'icons/icon32.svg',
    '48': 'icons/icon48.svg',
    '128': 'icons/icon128.svg',
  },
  action: {
    default_popup: 'popup.html',
    default_icon: {
      '16': 'icons/icon16.svg',
      '32': 'icons/icon32.svg',
      '48': 'icons/icon48.svg',
      '128': 'icons/icon128.svg',
    },
  },
  // Firefox MV3 supports both service_worker and scripts
  background: {
    scripts: ['src/background/index.ts'],
    type: 'module',
  },
  permissions: [
    'proxy',
    'storage',
    'nativeMessaging',
    'declarativeNetRequest',
    'tabs',
    'alarms',
  ],
  host_permissions: [
    '<all_urls>',
  ],
  // Firefox-specific: allow native messaging host
  browser_specific_settings: {
    gecko: {
      id: 'unbound-web@unbound.local',
      strict_min_version: '109.0',
    },
  },
  content_security_policy: {
    extension_pages: "script-src 'self'; object-src 'self'",
  },
};
