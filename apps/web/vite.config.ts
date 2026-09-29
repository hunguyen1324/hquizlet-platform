import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    allowedHosts: [".trycloudflare.com"],
    // Docker bind mounts on Windows can miss native filesystem notifications.
    watch: { usePolling: true, interval: 1000 },
  },
});
