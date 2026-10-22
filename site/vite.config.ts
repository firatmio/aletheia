import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

// ssr: true compiles components for hydration; `bun run build` renders the
// page to HTML at build time (scripts/prerender.ts) and the client hydrates it.
export default defineConfig({
  plugins: [solid({ ssr: true })],
});
