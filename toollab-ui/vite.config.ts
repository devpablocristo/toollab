import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

function resolveCoreModule(relativePath: string): string {
  const candidates = [
    path.resolve(__dirname, `.deps/core/${relativePath}/src`),
    path.resolve(__dirname, `../../core/${relativePath}/src`),
  ]
  return candidates.find((candidate) => fs.existsSync(candidate)) ?? candidates[0]
}

const coreHttpPath = resolveCoreModule('http/ts')
const coreBrowserPath = resolveCoreModule('browser/ts')

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@devpablocristo/core-http': coreHttpPath,
      '@devpablocristo/core-browser': coreBrowserPath,
    },
  },
  server: {
    port: 5173,
    fs: {
      allow: [path.resolve(__dirname), coreHttpPath, coreBrowserPath],
    },
    proxy: {
      '/api': {
        target: process.env.PROXY_BACKEND || 'http://localhost:8090',
        changeOrigin: true,
      },
      '/healthz': {
        target: process.env.PROXY_BACKEND || 'http://localhost:8090',
        changeOrigin: true,
      },
    },
  },
})
