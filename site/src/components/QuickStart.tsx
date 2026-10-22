import { createSignal, For, Show } from "solid-js";
import { quickstart } from "../content";
import Copy from "./Copy";

export default function QuickStart() {
  const [tab, setTab] = createSignal(quickstart.tabs[0].id);
  const current = () => quickstart.tabs.find((t) => t.id === tab())!;
  return (
    <div class="qs">
      <div class="tabs" role="tablist" aria-label="Set up Aletheia">
        <For each={quickstart.tabs}>
          {(t) => (
            <button
              type="button"
              role="tab"
              id={`tab-${t.id}`}
              aria-selected={tab() === t.id}
              aria-controls="qs-panel"
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          )}
        </For>
      </div>
      <div class="qs-panel" id="qs-panel" role="tabpanel" aria-labelledby={`tab-${tab()}`}>
        <For each={current().blocks}>
          {(b) => {
            let pre: HTMLPreElement | undefined;
            return (
              <div class="qs-block">
                <p>{b.note}</p>
                <Show when={b.code}>
                  <div class="snippet">
                    <pre ref={pre} classList={{ wrap: "wrap" in b && !!b.wrap }}>{b.code}</pre>
                    <Copy text={b.code} target={() => pre} />
                  </div>
                </Show>
              </div>
            );
          }}
        </For>
      </div>
    </div>
  );
}
