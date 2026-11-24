import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: { '/api': 'http://127.0.0.1:18090', '/healthz': 'http://127.0.0.1:18090' },
  },
  test: {
    // e2e запускает Playwright, Vitest не должен подхватывать эти файлы.
    exclude: ['node_modules/**', 'dist/**', 'e2e/**'],
  },
});
