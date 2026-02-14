import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "/assets/",
  build: {
    outDir: "../backend/internal/httpapi/assets",
    emptyOutDir: true,
    sourcemap: false,
    rollupOptions: {
      output: {
        inlineDynamicImports: true,
        entryFileNames: "app.js",
        assetFileNames: (assetInfo) => {
          if (assetInfo.name?.endsWith(".css")) {
            return "app.css";
          }
          return "assets/[name]-[hash][extname]";
        },
      },
    },
  },
});
