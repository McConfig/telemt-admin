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
    environment: 'jsdom',
    server: { deps: { inline: [/@gravity-ui\//] } },
    setupFiles: ['./src/test/setup.ts'],
    exclude: ['node_modules/**', 'dist/**', 'e2e/**'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json-summary', 'lcov'],
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/**/*.test.{ts,tsx}', 'src/main.tsx', 'src/test/**'],
      thresholds: { statements: 60, lines: 60, functions: 45, branches: 40 },
    },
  },
});
