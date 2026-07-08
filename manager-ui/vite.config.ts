/// <reference types="vitest" />
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

const DEFAULT_API_TARGET = "http://127.0.0.1:8484";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiTarget = env.VITE_API_PROXY_TARGET || DEFAULT_API_TARGET;

  return {
    plugins: [react()],
    server: {
      host: "0.0.0.0",
      port: 5174,
      proxy: {
        "/api": { target: apiTarget, changeOrigin: true },
        "/health": { target: apiTarget, changeOrigin: true },
      },
    },
    test: {
      globals: true,
      environment: "happy-dom",
      testTimeout: 20000,
      hookTimeout: 20000,
      exclude: ["tests/e2e/**", "node_modules/**"],
      css: false,
    },
    build: {
      chunkSizeWarningLimit: 1800,
    },
  };
});
