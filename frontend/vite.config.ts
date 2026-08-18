/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    // Pinned so the suite does not depend on a local .env file. Tests mock
    // fetch and ignore the URL, but the API client now refuses to build a
    // request without this set.
    env: {
      VITE_API_BASE_URL: "http://localhost:8080/api/v1",
    },
  },
});
