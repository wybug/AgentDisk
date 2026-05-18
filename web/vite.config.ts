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
      '/auth': {
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
