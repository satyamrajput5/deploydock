import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// during dev the go api runs on :8080, so just forward the api calls to it
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/auth": "http://localhost:8080",
      "/deployments": "http://localhost:8080",
    },
  },
});
