import { mkdirSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const OUT_DIR = 'dist'

/**
 * main.go embeds frontend/dist with //go:embed, which fails the whole Go build
 * if the directory does not exist. The directory itself is therefore tracked
 * via a .gitkeep placeholder — but `emptyOutDir` wipes it on every build, which
 * would leave `git status` permanently dirty. Put it back after each build.
 */
function keepEmbedDir(): Plugin {
  return {
    name: 'unbound-keep-embed-dir',
    apply: 'build',
    closeBundle() {
      const dir = resolve(__dirname, OUT_DIR)
      mkdirSync(dir, { recursive: true })
      writeFileSync(resolve(dir, '.gitkeep'), '')
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), keepEmbedDir()],
  build: {
    outDir: OUT_DIR,
    emptyOutDir: true,
    // The desktop shell always ships the matching build, so source maps cost
    // bundle size for no debugging benefit in production.
    sourcemap: false,
  },
})
