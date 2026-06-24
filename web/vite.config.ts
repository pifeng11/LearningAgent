import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const apiTarget = process.env.VITE_API_TARGET || "http://localhost:8080";
const wsTarget = apiTarget.replace(/^http/, "ws");

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      "/api": apiTarget,
      "/ws": {
        target: wsTarget,
        ws: true,
      },
    },
  },
});
