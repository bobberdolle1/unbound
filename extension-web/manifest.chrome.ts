import type { ManifestV3 } from '@crxjs/vite-plugin';

export const chromeManifest: ManifestV3 = {
  manifest_version: 3,
  name: 'Unbound Web',
  version: '2.0.0',
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
  background: {
    service_worker: 'src/background/index.ts',
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
  content_security_policy: {
    extension_pages: "script-src 'self'; object-src 'self'",
  },
};
