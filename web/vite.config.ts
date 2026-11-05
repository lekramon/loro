import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  plugins: [react()],
  build: {
    outDir: fileURLToPath(new URL('../internal/webui/dist', import.meta.url)),
    emptyOutDir: true,
  },
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
        // Development only: forward a backend-local Origin. The Vite server
        // remains loopback-only; production uses same-origin embedded assets.
        configure(proxy) {
          proxy.on('proxyReq', (request) => {
            if (request.hasHeader('origin')) request.setHeader('origin', 'http://127.0.0.1:8080');
          });
        },
      },
    },
  },
});
