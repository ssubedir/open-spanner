import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  base: '/',
  build: {
    emptyOutDir: false,
    outDir: '../internal/ui/static',
  },
  plugins: [react()],
  server: {
    proxy: {
      '/v1': 'http://127.0.0.1:18083',
    },
  },
})
