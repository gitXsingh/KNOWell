import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: { host: "::", port: 5173, strictPort: true },
  preview: { host: "::", port: 5173, strictPort: true },
});
