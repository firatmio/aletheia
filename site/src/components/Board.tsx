import { createEffect, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { board } from "../content";

// The symbol: anchor of the editable claim, checked in the browser the way
// Aletheia checks it: the file must still declare Builtins.
const declaresBuiltins = (line: string) => /^\s*var\s+Builtins\b/.test(line);

export default function Board() {
  const [active, setActive] = createSignal("builtins");
  const [line, setLine] = createSignal(board.tryValue);
  const [path, setPath] = createSignal("");
  const holds = () => declaresBuiltins(line());
  const claim = () => board.claims.find((c) => c.id === active())!;
  let root!: HTMLDivElement;

  // Draws the connector from the active claim to its proving line.
  const draw = () => {
    const card = root.querySelector<HTMLElement>(".claim-card.active");
    const hit = root.querySelector<HTMLElement>(".code .hit");
    if (!card || !hit || window.innerWidth < 880) return setPath("");
    const o = root.getBoundingClientRect();
    const a = card.getBoundingClientRect();
    const b = hit.getBoundingClientRect();
    const x1 = a.right - o.left, y1 = a.top + a.height / 2 - o.top;
    const x2 = b.left - o.left, y2 = b.top + b.height / 2 - o.top;
    const mx = (x1 + x2) / 2;
    setPath(`M${x1} ${y1} C${mx} ${y1} ${mx} ${y2} ${x2} ${y2}`);
  };
  onMount(() => {
    draw();
    window.addEventListener("resize", draw);
    onCleanup(() => window.removeEventListener("resize", draw));
  });
  createEffect(() => (active(), line(), queueMicrotask(draw)));

  const broken = (id: string) => id === "builtins" && !holds();

  return (
    <section class="wrap section board" id="board">
      <span class="eyebrow">{board.eyebrow}</span>
      <h2>{board.title}</h2>
      <p class="sub">{board.sub}</p>
      <div class="board-grid" ref={root}>
        <svg class="connector" classList={{ broken: broken(active()) }} aria-hidden="true">
          <Show when={path()}>
            <path d={path()} />
          </Show>
        </svg>
        <div class="claims">
          <For each={board.claims}>
            {(c) => (
              <button
                type="button"
                class="claim-card"
                classList={{ active: active() === c.id, broken: broken(c.id) }}
                aria-pressed={active() === c.id}
                onMouseEnter={() => setActive(c.id)}
                onFocus={() => setActive(c.id)}
                onClick={() => setActive(c.id)}
              >
                <span class="claim-text">{c.claim}</span>
                <span class="claim-anchor">
                  {c.anchor} · {broken(c.id) ? "stale" : "holds"}
                </span>
              </button>
            )}
          </For>
        </div>
        <div class="code">
          <div class="code-head">
            <span>{claim().file}</span>
            <span>checked just now</span>
          </div>
          <div class="code-body">
            <For each={claim().lines}>
              {(l) => (
                <div class="ln" classList={{ hit: !!("hit" in l && l.hit), broken: !!("hit" in l && l.hit) && broken(active()) }}>
                  <i>{l.n}</i>
                  <span>
                    <Show when={"editable" in claim() && "hit" in l && l.hit} fallback={<For each={l.t}>{([cls, text]) => <span class={cls || undefined}>{text}</span>}</For>}>
                      {line()}
                    </Show>
                  </span>
                </div>
              )}
            </For>
          </div>
        </div>
      </div>

      <div class="try">
        <div class="try-head">
          <h3>{board.tryTitle}</h3>
          <span class="fine">{board.tryNote}</span>
        </div>
        <label for="try-line">{board.tryLabel}</label>
        <input
          id="try-line"
          value={line()}
          spellcheck={false}
          autocomplete="off"
          classList={{ broken: !holds() }}
          onInput={(e) => (setLine(e.currentTarget.value), setActive("builtins"))}
        />
        <p class="try-state" classList={{ broken: !holds() }} aria-live="polite">
          ● {holds() ? board.holds : board.stale}
        </p>
      </div>
    </section>
  );
}
