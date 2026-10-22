import { createSignal, For, onMount, Show } from "solid-js";
import Board from "./components/Board";
import Copy from "./components/Copy";
import Mark from "./components/Mark";
import QuickStart from "./components/QuickStart";
import Story from "./components/Story";
import { footer, hero, proof, quickstart, site } from "./content";

// Dark is the default; the toggle stores the visitor's choice.
function ThemeToggle() {
  const [light, setLight] = createSignal(false);
  onMount(() => setLight(document.documentElement.dataset.theme === "light"));
  const toggle = () => {
    const next = light() ? "dark" : "light";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {}
    setLight(next === "light");
  };
  return (
    <button class="icon-btn" type="button" onClick={toggle} aria-label={light() ? "Switch to dark theme" : "Switch to light theme"}>
      <svg width="16" height="16" viewBox="0 0 24 24" aria-hidden="true">
        <circle cx="12" cy="12" r="8" fill="none" stroke="currentColor" stroke-width="2" />
        <path d="M12 4a8 8 0 0 1 0 16z" fill="currentColor" />
      </svg>
    </button>
  );
}

export default function App() {
  let cmd: HTMLElement | undefined;
  return (
    <>
      <header class="top">
        <nav class="wrap nav" aria-label="Main">
          <a class="brand" href="#top" aria-label="Aletheia home">
            <Mark size={30} drop />
            <span class="wordmark">aletheia</span>
          </a>
          <div class="navlinks">
            <a href="#board" class="hide-sm">How it works</a>
            <a href="#results" class="hide-sm">Results</a>
            <a href="#start">Quick start</a>
            <a href="/agents.md" class="hide-sm">For agents</a>
            <Show when={site.released} fallback={<span class="pill">GitHub · soon</span>}>
              <a class="pill" href={site.repo}>GitHub</a>
            </Show>
            <ThemeToggle />
          </div>
        </nav>
      </header>

      <main id="top">
        <section class="hero wrap">
          <span class="eyebrow">{hero.eyebrow}</span>
          <h1>
            {hero.title[0]}
            <br />
            {hero.title[1]}
            <em>{hero.title[2]}</em>
          </h1>
          <p class="lede">{hero.lede}</p>
          <div class="hero-actions">
            <div class="install">
              <code ref={cmd}>
                <span class="dim">$ </span>
                {site.install}
              </code>
              <Copy text={site.install} target={() => cmd} />
            </div>
            <a class="btn solid" href="#story">
              {hero.cta} ↓
            </a>
          </div>
          <Show when={!site.released}>
            <p class="soon">{hero.notReleased}</p>
          </Show>
          <p class="cue">{hero.cue}</p>
        </section>

        <div id="story">
          <Story />
        </div>

        <Board />

        <section class="wrap section" id="results">
          <span class="eyebrow">{proof.eyebrow}</span>
          <h2>{proof.title}</h2>
          <div class="results">
            <div class="figs">
              <For each={proof.figures}>
                {(f, i) => (
                  <div class="fig">
                    <span class="fig-value">
                      {f.value}
                      <a class="fn" href={`#fn-${i() + 1}`} aria-label={`Note ${i() + 1}`}>
                        {i() + 1}
                      </a>
                    </span>
                    <span class="fig-label">{f.label}</span>
                  </div>
                )}
              </For>
            </div>
            <aside class="notes">
              <ol>
                <For each={proof.notes}>{(n, i) => <li id={`fn-${i() + 1}`}>{n}</li>}</For>
              </ol>
              <p>{proof.caveat}</p>
              <Show when={site.released} fallback={<p class="dim">{proof.methodSoon}</p>}>
                <a href={site.method}>{proof.methodLink} →</a>
              </Show>
            </aside>
          </div>
        </section>

        <section class="wrap section" id="start">
          <h2>{quickstart.title}</h2>
          <p class="sub">{quickstart.sub}</p>
          <QuickStart />
        </section>
      </main>

      <footer>
        <div class="wrap foot">
          <span>{footer.left}</span>
          <span>
            {footer.agents} <a href="/agents.md">/agents.md</a> · <a href="/llms.txt">/llms.txt</a>
          </span>
          <span>{footer.right}</span>
        </div>
      </footer>
    </>
  );
}
