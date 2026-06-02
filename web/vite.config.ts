import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 9101,
    proxy: {
      '/v1': {
        target: 'http://localhost:9100',
        changeOrigin: true,
      },
      '/auth/login': {
        target: 'http://localhost:9100',
        changeOrigin: true,
      },
      '/auth/callback': {
        target: 'http://localhost:9100',
        changeOrigin: true,
      },
      '/auth/status': {
        target: 'http://localhost:9100',
        changeOrigin: true,
      },
      '/auth/logout': {
        target: 'http://localhost:9100',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://localhost:9100',
        changeOrigin: true,
      },
    },
  },
})
