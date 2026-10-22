import { createEffect, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { story, type Item, type Seg } from "../content";

const clamp = (x: number) => Math.min(1, Math.max(0, x));
const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
const hash = (n: number) => {
  let x = Math.imul(n ^ 0x9e3779b9, 0x85ebca6b);
  x ^= x >>> 13;
  return (Math.imul(x, 0xc2b2ae35) >>> 0) / 4294967296;
};
const NOISE = "░▒▓";

// Replaces a growing share of characters with block glyphs as g goes 0 → 1.
function scramble(text: string, g: number, seed: number) {
  if (g <= 0) return text;
  let out = "";
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    const h = hash(seed * 997 + i);
    out += c !== " " && h < g ? NOISE[Math.floor(h * 1000) % 3] : c;
  }
  return out;
}

// The sticky terminal. Everything on screen is a function of the scroll
// progress p (0..1 over the section), so scrolling back rewinds the scene and
// the page never changes the scroll speed. Without motion, CSS shows the
// whole session as a static transcript.
export default function Story() {
  let section!: HTMLElement;
  let body!: HTMLDivElement;
  let start = 0;
  const [p, setP] = createSignal(0);
  const [enter, setEnter] = createSignal(0);
  const [clicked, setClicked] = createSignal(false);

  const span = () => section.getBoundingClientRect().height - window.innerHeight;
  onMount(() => {
    const measure = () => (start = section.getBoundingClientRect().top + window.scrollY);
    const update = () => {
      const top = section.getBoundingClientRect().top;
      setP(clamp(-top / Math.max(1, span())));
      setEnter(clamp(1 - top / Math.max(1, start)));
    };
    // Browsers already fire scroll at most once per frame.
    const onScroll = update;
    const onResize = () => (measure(), update());
    measure();
    update();
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onResize);
    onCleanup(() => {
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onResize);
    });
  });

  const renamed = () => clicked() || p() >= story.renameAuto;
  // The newest line stays in view, like a terminal that scrolls.
  createEffect(() => (p(), clicked(), queueMicrotask(() => (body.scrollTop = body.scrollHeight))));
  const visible = (it: Item) => p() >= it.at && (!("afterRename" in it && it.afterRename) || renamed());
  const act = () => {
    let i = 0;
    story.acts.forEach((a, j) => p() >= a.start && (i = j));
    return i;
  };
  const g = () => clamp((p() - story.glitch) / (1 - story.glitch));
  const inRange = (r: readonly [number, number]) => p() >= r[0] && p() < r[1];
  const jump = (i: number) => {
    const target = start + story.acts[i].start * span() + 2;
    const smooth = !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    window.scrollTo({ top: target, behavior: smooth ? "smooth" : "auto" });
  };

  const okVisible = () =>
    story.items.filter((it) => it.kind === "box" && visible(it) && p() < story.acts[1].start)
      .reduce((n, it) => n + (it.kind === "box" ? it.mems.filter((m) => m.state === "ok").length : 0), 0);

  // A line that survives the glitch stays sharp; everything else scrambles,
  // drifts sideways and fades.
  const drift = (seed: number, keep: boolean) =>
    keep || g() === 0
      ? undefined
      : { transform: `translateX(${(hash(seed) < 0.5 ? -1 : 1) * g() * (16 + hash(seed + 7) * 48)}px)`, opacity: 1 - g() * 0.9 };
  const segs = (list: readonly Seg[], seed: number, keep: () => boolean) => (
    <For each={list}>{([cls, text], j) => <span class={cls || undefined}>{keep() ? text : scramble(text, g(), seed * 31 + j())}</span>}</For>
  );
  const never = () => false;

  return (
    <section class="story" ref={section} aria-label="An agent working with Aletheia">
      <div class="story-sticky">
        <div class="story-bar" style={{ width: `${p() * 100}%` }} />
        <nav class="rail" classList={{ on: enter() > 0.98 }} aria-label="Story">
          <For each={story.acts}>
            {(a, i) => (
              <button type="button" aria-current={act() === i() ? "step" : undefined} onClick={() => jump(i())}>
                <span class="num">{String(i() + 1).padStart(2, "0")}</span>
                <span class="label">{a.label}</span>
              </button>
            )}
          </For>
          <a class="skip" href="#board">Skip the story ↓</a>
        </nav>

        <div class="captions" aria-hidden="true">
          <For each={story.acts}>
            {(a, i) => <p classList={{ on: enter() > 0.98 && act() === i() && (i() < 3 || g() > 0.35) }}>{a.caption}</p>}
          </For>
        </div>

        <div class="ghost" classList={{ on: inRange(story.ghost) }} aria-hidden="true">
          <div class="ghost-head">
            <span>{story.ghostLabel}</span>
            <span>{story.ghostNote}</span>
          </div>
          <For each={story.ghostLines}>{([cls, text]) => <div class={cls || undefined}>{text}</div>}</For>
        </div>

        <div class="card" classList={{ on: inRange(story.cardAt) }} aria-hidden="true">
          <div class="card-head">{story.card.file}</div>
          <div class="card-body">
            <For each={story.card.lines}>{([cls, text]) => <span class={cls || undefined}>{text}</span>}</For>
          </div>
          <div class="card-foot">{story.card.note}</div>
        </div>

        <div class="term" style={{ transform: `scale(${0.92 + 0.08 * enter()})`, "--g": g() }}>
          <div class="term-head">
            <span>{scramble("claude · ~/src/expr", g(), 1)}</span>
            <span class="hud">
              <Show when={act() === 0}>
                <span>turns <b>{Math.round(lerp(0, story.hud.turns, clamp(p() / 0.32)))}</b></span>
                <span>read before first edit <b class="ok">{Math.round(lerp(0, story.hud.read, clamp(p() / 0.26)))}k</b> chars</span>
                <span>memories <b>{okVisible()}</b></span>
              </Show>
              <Show when={act() === 1}>
                <span>moved <b class="ptr">{renamed() ? 1 : 0}</b></span>
                <span>stale hidden <b class="stale">{renamed() ? 1 : 0}</b></span>
              </Show>
              <Show when={act() >= 2}>
                <span>agent turns spent on memory <b class="ok">0</b></span>
              </Show>
            </span>
          </div>
          <div class="hud-ctrl" classList={{ on: act() === 0 && p() >= 0.32 }}>
            without Aletheia: {story.hud.ctrlTurns} turns · {story.hud.ctrlRead}k chars
          </div>
          <div class="term-body" ref={body}>
            <For each={story.items}>
              {(it, i) => (
                <div class={`t-item t-${it.kind}`} classList={{ on: visible(it) }}>
                  <Show when={it.kind === "prompt" && it}>
                    {(x) => (
                      <div style={drift(i(), false)}>
                        <span class="dim">&gt; </span>
                        {segs(x().segs, i(), never)}
                      </div>
                    )}
                  </Show>
                  <Show when={it.kind === "line" && it}>{(x) => <div style={drift(i(), false)}>{segs(x().segs, i(), never)}</div>}</Show>
                  <Show when={it.kind === "rename"}>
                    <div class="rename" style={drift(i(), false)}>
                      <button type="button" class="chip" disabled={renamed()} onClick={() => setClicked(true)}>
                        {story.renameChip}
                      </button>
                      <span class="dim">{renamed() ? "" : story.renameHint}</span>
                    </div>
                    <div class="stale t-cmd" classList={{ on: renamed() }} style={drift(i() + 100, false)}>
                      $ {scramble(story.renameCmd, g(), 77)}
                    </div>
                  </Show>
                  <Show when={it.kind === "box" && it}>
                    {(x) => (
                      <div class="box">
                        <div class="box-head" style={drift(i() + 200, false)}>
                          <span>
                            <span class="ok">● </span>
                            {scramble(x().head, g(), i() + 300)}
                          </span>
                          <span>{scramble(x().meta ?? "", g(), i() + 400)}</span>
                        </div>
                        <For each={x().mems}>
                          {(m, j) => {
                            const keep = () => (m.state === "ok" || m.state === "moved") && p() >= story.acts[2].start;
                            return (
                              <div class={`mem is-${m.state}`} classList={{ keep: keep() && g() > 0 }} style={drift(i() * 10 + j(), keep())}>
                                <span class="tag">{m.state}</span>
                                <span class="claim">{segs(m.segs, i() * 10 + j(), keep)}</span>
                                <Show when={m.reason}>
                                  <span class="reason">{scramble(m.reason!, g(), i() * 10 + j() + 500)}</span>
                                </Show>
                              </div>
                            );
                          }}
                        </For>
                      </div>
                    )}
                  </Show>
                </div>
              )}
            </For>
          </div>
          <div class="term-foot">{story.acts[act()].source}</div>
        </div>
      </div>
    </section>
  );
}
