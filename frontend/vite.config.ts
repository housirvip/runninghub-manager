import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:3060',
      '/task': 'http://localhost:3060',
      '/openapi': 'http://localhost:3060',
      '/files': 'http://localhost:3060',
      '/uploads': 'http://localhost:3060',
    },
  },
  build: {
    // Ensure consistent chunk naming to reduce stale-cache issues
    rollupOptions: {
      output: {
        manualChunks: undefined,
      },
    },
  },
})
