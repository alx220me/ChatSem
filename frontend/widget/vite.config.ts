import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
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
})
