import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import { QRCODE_CHUNK } from './vite.qrcode.ts';

/** In dev, every page address gets the shell, as the server's gate does in production. */
function accountDevFallback(): Plugin {
  return {
    name: 'account-dev-fallback',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        const path = (req.url ?? '/').split('?')[0]!;
        const page = req.method === 'GET' && (req.headers.accept ?? '').includes('text/html')
          && !/\.[a-z0-9]+$/i.test(path) && !/^\/(api|@|src|node_modules)(\/|$)/.test(path);
        if (page) req.url = '/account.html';
        next();
      });
    },
  };
}

// The public account site: its own entry, so the bundle cannot reach game code or graphics. The SSR build only
// prerenders pages at build time and is never served.
export default defineConfig(({ isSsrBuild }) => isSsrBuild
  ? {
    plugins: [react()],
    publicDir: false,
    build: { ssr: 'src/account/entry-server.tsx', outDir: 'dist/account-ssr', emptyOutDir: true },
  }
  : {
    plugins: [react(), accountDevFallback()],
    base: '/',
    publicDir: 'public/account',
    build: {
      outDir: 'dist/account',
      emptyOutDir: true,
      rollupOptions: {
        input: 'account.html',
        output: { codeSplitting: { groups: [QRCODE_CHUNK, { name: 'shared', test: /node_modules\/(?!qrcode-generator\/)|src\/account\/(router|routes|slugs|api)\.tsx?$|src\/i18n\/(i18n|format|names)\.ts$/ }] } },
      },
    },
    server: { proxy: { '/api': { target: 'http://localhost:8080', ws: true } } },
  });
