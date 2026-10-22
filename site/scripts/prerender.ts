// Renders the page to HTML after `vite build` and the SSR build, so the
// first paint needs no JavaScript. The client then hydrates it.
import { rm } from "node:fs/promises";

const template = await Bun.file("dist/index.html").text();
const { render } = await import("../dist-ssr/entry-server.js");
const { html, head } = render();

if (!template.includes("<!--app-html-->")) throw new Error("dist/index.html has no <!--app-html--> marker");
await Bun.write("dist/index.html", template.replace("<!--app-head-->", head).replace("<!--app-html-->", html));
await rm("dist-ssr", { recursive: true, force: true });
console.log(`prerendered dist/index.html (${(html.length / 1024).toFixed(1)} KB of HTML)`);
