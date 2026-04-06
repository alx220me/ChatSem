import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ command }) => ({
  plugins: [react()],
  define: command === 'build' ? { 'process.env.NODE_ENV': '"production"' } : {},
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
  build: {
    lib: {
      entry: 'src/index.tsx',
      name: 'ChatSem',
      formats: ['iife'],
      fileName: () => 'widget.js',
    },
    rollupOptions: {
      // React is bundled into the widget — no external deps required on the host page
      external: [],
    },
  },
}))
