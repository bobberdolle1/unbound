import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { crx, type ManifestV3 } from '@crxjs/vite-plugin';
import { resolve } from 'path';
import { chromeManifest } from './manifest.chrome';
import { firefoxManifest } from './manifest.firefox';

export default defineConfig(({ mode }) => {
  const isFirefox = mode === 'firefox';
  const manifest: ManifestV3 = isFirefox ? firefoxManifest : chromeManifest;

  return {
    plugins: [
      react(),
      crx({
        manifest,
        browser: isFirefox ? 'firefox' : 'chrome',
      }),
    ],
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
      },
    },
    build: {
      outDir: `dist/${isFirefox ? 'firefox' : 'chrome'}`,
      emptyOutDir: true,
      rollupOptions: {
        input: {
          popup: resolve(__dirname, 'popup.html'),
        },
      },
    },
  };
});
