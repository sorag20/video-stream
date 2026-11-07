import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// プロキシ先は nginx と同じ環境変数名で上書きできる（docker compose ではコンテナのサービス名を指定する）
const uploadServiceUrl = process.env.UPLOAD_SERVICE_URL ?? 'http://localhost:8080'
const streamingServiceUrl = process.env.STREAMING_SERVICE_URL ?? 'http://localhost:8082'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // 本番 (nginx / Gateway API) と同じく、ブラウザからは同一オリジンで API を呼び出せるようにする
    proxy: {
      '/videos': uploadServiceUrl,
      '/stream': streamingServiceUrl,
    },
  },
})
