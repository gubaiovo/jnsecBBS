import path from "node:path"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"
import tailwindcss from "@tailwindcss/vite"

// https://vite.dev/config/
export default defineConfig({
  // base 设为 /static/：/assets/* 留给旧 Flarum 静态资源（头像与附件）继续可访问
  base: "/static/",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:18080",
      "/captcha": "http://127.0.0.1:18080",
      "/avatars": "http://127.0.0.1:18080",
      "/avatar": "http://127.0.0.1:18080",
    },
  },
  build: {
    outDir: "../internal/frontend/dist",
    emptyOutDir: true,
  },
})
