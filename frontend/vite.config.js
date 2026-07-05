import { defineConfig } from 'vite';

export default defineConfig({
  server: {
    host: true,
    port: 5173,
    allowedHosts: ['.ngrok-free.dev', '.ngrok-free.app'],
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
