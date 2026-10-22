import { hydrate, render } from "solid-js/web";
import "@fontsource/newsreader/400.css";
import "@fontsource/newsreader/400-italic.css";
import "@fontsource/ibm-plex-sans/400.css";
import "@fontsource/ibm-plex-sans/500.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./styles.css";
import App from "./App";

const root = document.getElementById("app")!;
// The built page is prerendered and hydrated; `vite dev` serves an empty shell.
if (root.firstElementChild) hydrate(() => <App />, root);
else render(() => <App />, root);
