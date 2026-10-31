import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // API Gateway GraphQL endpoint
      '/graphql': {
        target: 'http://localhost:4000',
        changeOrigin: true,
      },
      // Media-service WebSocket (realtime scan/processing events)
      '/ws': {
        target: 'http://localhost:4000',
        ws: true,
        changeOrigin: true,
      },
    },
  },
});