import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import Board from "./components/Board";
import Copy from "./components/Copy";
import Mark from "./components/Mark";
import QuickStart from "./components/QuickStart";
import Story from "./components/Story";
import { footer, hero, proof, quickstart, site } from "./content";

// The painting behind the hero, in gray; a small lens around the pointer
// shows it in color. Only for a mouse: touch has no hover to follow.
function HeroArt() {
  let el!: HTMLDivElement;
  onMount(() => {
    if (!window.matchMedia("(hover: hover) and (pointer: fine)").matches) return;
    let frame = 0;
    let x = 0;
    let y = 0;
    const draw = () => {
      frame = 0;
      const r = el.getBoundingClientRect();
      const inside = x >= r.left && x <= r.right && y >= r.top && y <= r.bottom;
      el.classList.toggle("lit", inside);
      if (inside) {
        el.style.setProperty("--mx", `${x - r.left}px`);
        el.style.setProperty("--my", `${y - r.top}px`);
      }
    };
    const move = (e: PointerEvent) => {
      x = e.clientX;
      y = e.clientY;
      if (!frame) frame = requestAnimationFrame(draw);
    };
    const leave = () => el.classList.remove("lit");
    window.addEventListener("pointermove", move, { passive: true });
    document.documentElement.addEventListener("pointerleave", leave);
    onCleanup(() => {
      window.removeEventListener("pointermove", move);
      document.documentElement.removeEventListener("pointerleave", leave);
      cancelAnimationFrame(frame);
    });
  });
  return (
    <div class="hero-art" ref={el} aria-hidden="true">
      <div class="gray" />
      <div class="lens" />
    </div>
  );
}

// The hero's install command, with a tab per way to install. The choice is
// remembered for the visitor; Go is the default, as in the prerendered page.
function InstallPicker() {
  let code!: HTMLElement;
  const tabs: HTMLButtonElement[] = [];
  const [pick, setPick] = createSignal(0);
  const current = () => site.installs[pick()];
  onMount(() => {
    try {
      const i = site.installs.findIndex((m) => m.id === localStorage.getItem("install"));
      if (i >= 0) setPick(i);
    } catch {}
  });
  const choose = (i: number, focus = false) => {
    setPick(i);
    if (focus) tabs[i]?.focus();
    try {
      localStorage.setItem("install", site.installs[i].id);
    } catch {}
  };
  const keys = (e: KeyboardEvent) => {
    const n = site.installs.length;
    const step = { ArrowRight: 1, ArrowLeft: n - 1 }[e.key];
    if (step) {
      e.preventDefault();
      choose((pick() + step) % n, true);
    }
  };
  return (
    <div class="installer">
      <div class="install-tabs" role="tablist" aria-label="Install with" onKeyDown={keys}>
        <For each={site.installs}>
          {(m, i) => (
            <button
              ref={(el) => (tabs[i()] = el)}
              type="button"
              role="tab"
              id={`install-${m.id}`}
              aria-selected={pick() === i()}
              aria-controls="install-cmd"
              tabindex={pick() === i() ? 0 : -1}
              classList={{ on: pick() === i() }}
              onClick={() => choose(i())}
            >
              {m.label}
            </button>
          )}
        </For>
        <span class="install-note">{current().note}</span>
      </div>
      <div class="install" id="install-cmd" role="tabpanel" aria-labelledby={`install-${current().id}`}>
        <code ref={code}>
          <span class="dim">$ </span>
          {current().cmd}
        </code>
        <Copy text={current().cmd} target={() => code} />
      </div>
    </div>
  );
}

// The GitHub mark (Octicons, MIT), in the text colour.
function GitHubMark() {
  return (
    <svg class="gh" width="15" height="15" viewBox="0 0 16 16" aria-hidden="true">
      <path
        fill="currentColor"
        d="M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z"
      />
    </svg>
  );
}

// Dark is the default; the toggle stores the visitor's choice.
function ThemeToggle() {
  const [light, setLight] = createSignal(false);
  onMount(() => setLight(document.documentElement.dataset.theme === "light"));
  const toggle = () => {
    const next = light() ? "dark" : "light";
    const root = document.documentElement;
    const apply = () => {
      root.dataset.theme = next;
      setLight(next === "light");
    };
    try {
      localStorage.setItem("theme", next);
    } catch {}
    // A cross-fade where the browser has view transitions, a short colour
    // transition elsewhere, and none for visitors who prefer less motion.
    const doc = document as Document & { startViewTransition?: (update: () => void) => unknown };
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      apply();
    } else if (doc.startViewTransition) {
      doc.startViewTransition(apply);
    } else {
      root.classList.add("theme-fade");
      apply();
      setTimeout(() => root.classList.remove("theme-fade"), 400);
    }
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
            <Show
              when={site.released}
              fallback={
                <span class="pill">
                  <GitHubMark /> GitHub <span class="pill-note">· soon</span>
                </span>
              }
            >
              <a class="pill" href={site.repo}>
                <GitHubMark /> GitHub
              </a>
            </Show>
            <ThemeToggle />
          </div>
        </nav>
      </header>

      <main id="top">
        <HeroArt />
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
            <InstallPicker />
            <a class="btn solid" href="#story">
              {hero.cta} ↓
            </a>
          </div>
          <Show when={!site.released}>
            <p class="soon">{hero.notReleased}</p>
          </Show>
          <p class="gloss">
            <span lang="grc">{hero.word}</span> <i>{hero.gloss.translit}</i> · {hero.gloss.sense}
          </p>
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
          <span>
            {footer.left} · <a href="/privacy">{footer.privacy}</a>
          </span>
          <span>
            {footer.agents} <a href="/agents.md">/agents.md</a> · <a href="/llms.txt">/llms.txt</a>
          </span>
          <span>{footer.right}</span>
        </div>
        <p class="wrap credit">
          {footer.credit.before}
          <a href={footer.credit.href}>
            <i>{footer.credit.title}</i>
          </a>
          {footer.credit.after}
        </p>
      </footer>
    </>
  );
}
