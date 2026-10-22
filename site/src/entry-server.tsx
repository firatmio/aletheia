import { generateHydrationScript, renderToString } from "solid-js/web";
import App from "./App";

export function render() {
  return { html: renderToString(() => <App />), head: generateHydrationScript() };
}
