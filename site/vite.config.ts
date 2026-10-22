import { defineConfig, type Plugin } from "vite";
import solid from "vite-plugin-solid";
import { agentsMd, llmsTxt } from "./src/agents";

// /agents.md and /llms.txt are plain text for agents, generated from
// src/content.ts: served by the dev server and emitted into dist on build.
function agentFiles(): Plugin {
  const files: Record<string, { type: string; body: () => string }> = {
    "agents.md": { type: "text/plain; charset=utf-8", body: agentsMd },
    "llms.txt": { type: "text/plain; charset=utf-8", body: llmsTxt },
  };
  return {
    name: "agent-files",
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const f = files[(req.url ?? "").split("?")[0].slice(1)];
        if (!f) return next();
        res.setHeader("Content-Type", f.type);
        res.end(f.body());
      });
    },
    generateBundle() {
      if (this.environment?.config.consumer === "server") return;
      for (const [fileName, f] of Object.entries(files)) {
        this.emitFile({ type: "asset", fileName, source: f.body() });
      }
    },
  };
}

// ssr: true compiles components for hydration; `bun run build` renders the
// page to HTML at build time (scripts/prerender.ts) and the client hydrates it.
export default defineConfig({
  plugins: [solid({ ssr: true }), agentFiles()],
});
