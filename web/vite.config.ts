import basicSsl from '@vitejs/plugin-basic-ssl'
import react from '@vitejs/plugin-react'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'

/**
 * Vite configuration.
 *
 * - `/api` is proxied to the Go server so the browser never deals with CORS.
 * - `host: true` exposes the dev server on the LAN so a phone can open it.
 * - Browsers only allow camera access on HTTPS or localhost. Setting
 *   `VITE_HTTPS=1` enables a self-signed certificate so the phone can use
 *   the camera over the LAN (accept the browser warning once).
 */
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const useHttps = env.VITE_HTTPS === '1' || env.VITE_HTTPS === 'true'
  const apiTarget = env.VITE_API_TARGET || 'http://localhost:8080'

  return {
    plugins: [react(), ...(useHttps ? [basicSsl()] : [])],
    server: {
      host: true,
      port: 5173,
      strictPort: false,
      proxy: {
        '/api': {
          target: apiTarget,
          changeOrigin: true,
          // Transforms can take a couple of minutes; do not let the proxy cut them off.
          timeout: 300_000,
          proxyTimeout: 300_000,
        },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: false,
      target: 'es2022',
    },
    test: {
      environment: 'node',
      include: ['src/**/*.test.ts'],
    },
  }
})
