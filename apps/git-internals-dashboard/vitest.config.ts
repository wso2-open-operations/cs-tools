import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  test: {
    // DB-backed tests (ingest, incremental sync) share fixture rows keyed by
    // owner/repo name — running test files in parallel races those rows.
    fileParallelism: false,
  },
});
