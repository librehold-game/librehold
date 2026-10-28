import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { QRCODE_CHUNK } from './vite.qrcode.ts';

// The game, served by the server under /game/ to logged-in players only.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: '/game/',
  publicDir: false,
  build: {
    outDir: 'dist/game',
    emptyOutDir: true,
    rollupOptions: { input: 'game.html', output: { codeSplitting: { groups: [QRCODE_CHUNK] } } },
  },
  server: { proxy: { '/api': { target: 'http://localhost:8080', ws: true } } },
  test: {
    include: ['tests/**/*.test.ts', 'tests/**/*.test.tsx'],
    environment: 'node',
    setupFiles: ['tests/setupI18n.ts'],
    testTimeout: 30_000,
  },
});
