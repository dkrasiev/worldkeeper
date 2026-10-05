import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In dev, run `go run ./cmd/worldkeeper -no-browser` and open the Vite URL
// with ?token=<token from config.json>. API calls are proxied to the Go server.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:25599", changeOrigin: true },
    },
  },
});
