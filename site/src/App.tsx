import { createSignal, For, onMount, Show } from "solid-js";
import Copy from "./components/Copy";
import Mark from "./components/Mark";
import QuickStart from "./components/QuickStart";
import RecallDemo from "./components/RecallDemo";
import { footer, hero, how, problem, proof, quickstart, site } from "./content";

function ThemeToggle() {
  const [dark, setDark] = createSignal(false);
  const effective = () => {
    const t = document.documentElement.dataset.theme;
    return t ? t === "dark" : window.matchMedia("(prefers-color-scheme: dark)").matches;
  };
  onMount(() => setDark(effective()));
  const toggle = () => {
    const next = effective() ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("theme", next);
    } catch {}
    setDark(next === "dark");
  };
  return (
    <button class="icon-btn" type="button" onClick={toggle} aria-label={dark() ? "Switch to light theme" : "Switch to dark theme"}>
      <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
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
      <header class="wrap">
        <nav aria-label="Main">
          <a class="brand" href="#top" aria-label="Aletheia home">
            <Mark size={34} />
            <span class="wordmark">aletheia</span>
          </a>
          <div class="navlinks">
            <a href="#how" class="hide-sm">How it works</a>
            <a href="#results" class="hide-sm">Results</a>
            <a href="#start">Quick start</a>
            <Show when={site.released}>
              <a class="btn" href={site.repo}>GitHub</a>
            </Show>
            <ThemeToggle />
          </div>
        </nav>
      </header>

      <main id="top">
        <section class="wrap hero">
          <div>
            <div class="eyebrow">{hero.eyebrow}</div>
            <h1>
              {hero.title[0]}
              <em>{hero.title[1]}</em>
            </h1>
            <p class="lede">{hero.lede}</p>
            <div class="install">
              <code ref={cmd}>
                <span class="prompt">$</span> {site.install}
              </code>
              <Copy text={site.install} target={() => cmd} />
            </div>
            <Show when={!site.released}>
              <p class="soon">{hero.notReleased}</p>
            </Show>
            <div class="ctas">
              <a class="btn solid" href="#start">Set up with Claude Code</a>
              <a class="btn" href="#results">See the results</a>
            </div>
          </div>
          <RecallDemo />
        </section>

        <section class="proof" id="results" aria-label="Measured results">
          <div class="wrap">
            <For each={proof.figures}>
              {(f) => (
                <div class="fig">
                  {f.value}
                  <small>{f.label}</small>
                </div>
              )}
            </For>
            <p class="fine fine-col">
              {proof.note}{" "}
              <Show when={site.released} fallback={proof.methodSoon}>
                <a href={site.method}>{proof.methodLink}</a>
              </Show>
            </p>
          </div>
        </section>

        <section class="wrap section">
          <h2>{problem.title}</h2>
          <p class="sub">{problem.text}</p>
          <div class="compare">
            <div class="card">
              <span class="label">{problem.naive.label}</span>
              <div class="rows">
                <For each={problem.naive.rows}>
                  {(r) => (
                    <div class="row">
                      <span class="score">{r.score}</span>
                      <span>{r.claim}</span>
                    </div>
                  )}
                </For>
              </div>
              <p class="verdict warn">{problem.naive.verdict}</p>
            </div>
            <div class="card">
              <span class="label">{problem.verified.label}</span>
              <div class="rows">
                <For each={problem.verified.rows}>
                  {(r) => (
                    <div class="row">
                      <span class="tag ok">{r.state}</span>
                      <span>
                        {r.claim} <span class="ptr">{r.ptr}</span>
                      </span>
                    </div>
                  )}
                </For>
                <div class="row dim">{problem.verified.footer}</div>
              </div>
              <p class="verdict good">{problem.verified.verdict}</p>
            </div>
          </div>
        </section>

        <section class="wrap section" id="how">
          <h2>{how.title}</h2>
          <ol class="steps">
            <For each={how.steps}>
              {(s, i) => (
                <li>
                  <span class="num">{i() + 1}</span>
                  <h3>{s.title}</h3>
                  <p>{s.text}</p>
                </li>
              )}
            </For>
          </ol>
          <h3 class="label anchors-title">{how.anchorsTitle}</h3>
          <dl class="anchors">
            <For each={how.anchors}>
              {(a) => (
                <div>
                  <dt>
                    <code>{a.kind}:</code>
                  </dt>
                  <dd>
                    holds while {a.holds}
                    <code class="example">{a.example}</code>
                  </dd>
                </div>
              )}
            </For>
          </dl>
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
          <span>{footer.right}</span>
        </div>
      </footer>
    </>
  );
}
