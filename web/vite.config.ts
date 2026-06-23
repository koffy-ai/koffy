import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  return {
    plugins: [react()],
    server: {
      host: "0.0.0.0",
      port: 5173,
      proxy: {
        "/api": env.VITE_BILLING_API_PROXY || "http://localhost:8080",
        "/auth": env.VITE_BILLING_API_PROXY || "http://localhost:8080"
      }
    },
    build: {
      chunkSizeWarningLimit: 1000,
      rollupOptions: {
        output: {
          manualChunks: {
            react: ["react", "react-dom", "react-router-dom"],
            antd: ["antd"],
            query: ["@tanstack/react-query"]
          }
        }
      }
    }
  };
});
