import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// [BACKEND] 개발 중에는 /api, /ws 요청을 Spring 서버(8080)로 프록시합니다. (CORS 설정이 필요 없음)
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
});
