import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  base: "/web/",
  server: {
    proxy: {
      "/web/ws": {
        target: "ws://localhost:8080",
        ws: true,
      },
      "/ws": {
        target: "ws://localhost:你的后端端口",
        ws: true,
      },
    },
  },
});
