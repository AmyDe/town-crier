import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (id.includes('node_modules/react/') || id.includes('node_modules/react-dom/') || id.includes('node_modules/react-router')) {
            return 'react';
          }
          if (id.includes('node_modules/@auth0/')) {
            return 'auth';
          }
          if (id.includes('node_modules/@tanstack/')) {
            return 'query';
          }
          // leaflet intentionally omitted — only used by lazy-loaded
          // MapPage, so Vite naturally splits it into that route chunk.
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test-setup.ts',
  },
})
