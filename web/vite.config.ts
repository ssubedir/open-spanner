import { tanstackRouter } from '@tanstack/router-plugin/vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'
import { defineConfig } from 'vite'

const apiProxyTarget = process.env.OPEN_SPANNER_API_PROXY_URL || 'http://127.0.0.1:18083'

// https://vite.dev/config/
export default defineConfig({
  base: '/',
  build: {
    emptyOutDir: false,
    outDir: '../internal/ui/static',
  },
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
    }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/v1': apiProxyTarget,
    },
  },
})
