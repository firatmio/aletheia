import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { demo } from "../content";

type Phase = "before" | "after";

// The hero demo: the same recall before and after a file is renamed. It
// alternates on its own unless the visitor prefers reduced motion, hovers it,
// or picks a phase; the server renders the "after" state.
export default function RecallDemo() {
  const [phase, setPhase] = createSignal<Phase>("after");
  const [auto, setAuto] = createSignal(false);
  const [hover, setHover] = createSignal(false);
  const state = () => demo[phase()];

  onMount(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    setAuto(true);
    const id = setInterval(() => {
      if (auto() && !hover()) setPhase((p) => (p === "after" ? "before" : "after"));
    }, 4500);
    onCleanup(() => clearInterval(id));
  });

  const pick = (p: Phase) => {
    setAuto(false);
    setPhase(p);
  };

  return (
    <div class="demo" onMouseEnter={() => setHover(true)} onMouseLeave={() => setHover(false)}>
      <div class="panel-head">
        <span>{demo.query}</span>
        <div class="phases" role="group" aria-label="Show the recall">
          <For each={["before", "after"] as Phase[]}>
            {(p) => (
              <button type="button" aria-pressed={phase() === p} onClick={() => pick(p)}>
                {demo[p].label}
              </button>
            )}
          </For>
        </div>
      </div>
      <Show when={phase() === "after"}>
        <div class="event">
          <span class="prompt">$</span> {demo.event}
        </div>
      </Show>
      <div class="recall" aria-live="polite">
        <For each={state().memories as readonly { state: string; claim: string; ptr?: string; reason?: string }[]}>
          {(m) => (
            <div class={`mem ${m.state === "stale" ? "is-stale" : ""}`}>
              <span class={`tag ${m.state}`}>{m.state}</span>
              <span class="claim">
                {m.claim} <Show when={m.ptr}><span class="ptr">{m.ptr}</span></Show>
              </span>
              <Show when={m.reason}><span class="reason">{m.reason}</span></Show>
            </div>
          )}
        </For>
      </div>
      <div class="code">
        <div class="file">
          <b>{state().file}</b>
          <span>checked just now</span>
        </div>
        <For each={demo.code}>
          {(line) => (
            <div class={`ln ${"hit" in line && line.hit ? "hit" : ""}`}>
              <i>{line.n}</i>
              <span>
                <For each={line.t}>{([cls, text]) => <span class={cls || undefined}>{text}</span>}</For>
              </span>
            </div>
          )}
        </For>
      </div>
      <div class="link-row">
        <span class="dot" aria-hidden="true" />
        anchor <span class="anchor">{state().anchor}</span> holds
      </div>
    </div>
  );
}
