import { createSignal } from "solid-js";

// A copy button that falls back to selecting the text when the clipboard API
// is unavailable (insecure context, permissions).
export default function Copy(props: { text: string; target?: () => HTMLElement | undefined }) {
  const [label, setLabel] = createSignal("Copy");
  const flash = (msg: string) => {
    setLabel(msg);
    setTimeout(() => setLabel("Copy"), 1600);
  };
  const select = () => {
    const el = props.target?.();
    if (!el) return;
    const range = document.createRange();
    range.selectNodeContents(el);
    const sel = window.getSelection();
    sel?.removeAllRanges();
    sel?.addRange(range);
  };
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(props.text);
      flash("Copied");
    } catch {
      select();
      flash("Press Ctrl+C");
    }
  };
  return (
    <button class="copy" type="button" onClick={copy} aria-live="polite">
      {label()}
    </button>
  );
}
