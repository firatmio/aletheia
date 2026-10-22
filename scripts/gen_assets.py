"""Generate the README "how it works" diagram (assets/how-it-works-*.svg).

Pure standard-library Python. Run from the repository root after editing:

    python3 scripts/gen_assets.py

The logo files in assets/brand/ are static; see docs/brand/README.md.
"""
import os
import sys

out = sys.argv[1] if len(sys.argv) > 1 else "assets"
W, H = 1200, 430
SANS = "-apple-system, 'Segoe UI', Helvetica, Arial, sans-serif"
MONO = "ui-monospace, SFMono-Regular, Consolas, 'Liberation Mono', monospace"

THEMES = {
    "light": dict(bg="#f4f6f7", card="#ffffff", rule="#d6dde3", ink="#142033", soft="#4a586c",
                  ok="#0f7a5a", okbg="#e2f2ec", stale="#a85f00", stalebg="#f8ecd9", code="#0f1724", codeink="#d7deea"),
    "dark": dict(bg="#0b111b", card="#121a27", rule="#253246", ink="#e5eaf1", soft="#9aa7b8",
                 ok="#3fc69a", okbg="#10322a", stale="#e3a14a", stalebg="#3a2a12", code="#070b12", codeink="#d7deea"),
}


def esc(s):
    return s.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def text(x, y, s, size=15, fill="ink", weight=400, font=SANS, anchor="start", t=None):
    return (f'<text x="{x}" y="{y}" font-family="{font}" font-size="{size}" font-weight="{weight}" '
            f'fill="{t[fill]}" text-anchor="{anchor}">{esc(s)}</text>')


def build(t):
    p = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" role="img" '
         'aria-label="How Aletheia works: on every prompt it adds verified memories and a code map, '
         'the agent reads the exact lines, and when the agent stops Aletheia records which files changed.">',
         f'<rect width="{W}" height="{H}" rx="16" fill="{t["bg"]}"/>',
         '<defs><marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto">'
         f'<path d="M1 1 L9 5 L1 9" fill="none" stroke="{t["soft"]}" stroke-width="1.6"/></marker></defs>']
    cols = [(40, "1  You send a prompt", "The prompt hook recalls for it."),
            (430, "2  The agent works", "It starts at the right lines."),
            (820, "3  The agent stops", "The stop hook records the change.")]
    for x, title, sub in cols:
        p.append(f'<rect x="{x}" y="40" width="340" height="300" rx="12" fill="{t["card"]}" stroke="{t["rule"]}"/>')
        p.append(text(x + 24, 78, title, 18, weight=600, t=t))
        p.append(text(x + 24, 102, sub, 14, "soft", t=t))
    # Column 1: what lands in the agent's context.
    x = 64
    p.append(f'<rect x="{x}" y="124" width="292" height="190" rx="8" fill="{t["code"]}"/>')
    rows = [("ok", "New builtin: register in", "registry.go:35-1081"),
            ("ok", "Arity entry needed in", "builtin_test.go:263"),
            ("stale", "Builtins live in builtin.go", None)]
    y = 154
    for tag, claim, ptr in rows:
        fill, bg = (t["ok"], t["okbg"]) if tag == "ok" else (t["stale"], t["stalebg"])
        p.append(f'<rect x="{x + 14}" y="{y - 13}" width="{42 if tag == "ok" else 50}" height="18" rx="4" fill="{bg}"/>')
        p.append(f'<text x="{x + 21}" y="{y}" font-family="{MONO}" font-size="12" fill="{fill}">{tag}</text>')
        cx = x + (64 if tag == "ok" else 72)
        style = ' text-decoration="line-through"' if tag == "stale" else ""
        p.append(f'<text x="{cx}" y="{y}" font-family="{MONO}" font-size="12" fill="{t["codeink"]}"{style}>{esc(claim)}</text>')
        if ptr:
            p.append(f'<text x="{cx}" y="{y + 18}" font-family="{MONO}" font-size="12" fill="{t["ok"]}">{esc(ptr)}</text>')
        y += 44 if ptr else 30
    p.append(f'<text x="{x + 14}" y="{y + 4}" font-family="{MONO}" font-size="11" fill="#6f7d91">stale: hidden from the agent</text>')
    p.append(f'<text x="{x + 14}" y="{y + 30}" font-family="{MONO}" font-size="12" fill="{t["codeink"]}">code map: 4 files, 18 symbols</text>')
    # Column 2: the read that a pointer buys.
    x = 454
    p.append(f'<rect x="{x}" y="124" width="292" height="190" rx="8" fill="{t["code"]}"/>')
    p.append(f'<text x="{x + 14}" y="148" font-family="{MONO}" font-size="12" fill="#6f7d91">Read builtin/registry.go</text>')
    p.append(f'<text x="{x + 14}" y="166" font-family="{MONO}" font-size="12" fill="#6f7d91">     lines 35-60, not 1081</text>')
    for i, line in enumerate(["var Builtins = []*Function{", "    {", '        Name: "abs",', "        Fast: Abs,", "    },"]):
        yy = 196 + i * 20
        if i == 0:
            p.append(f'<rect x="{x + 8}" y="{yy - 14}" width="276" height="20" fill="{t["ok"]}" opacity="0.22"/>')
        p.append(f'<text xml:space="preserve" x="{x + 14}" y="{yy}" font-family="{MONO}" font-size="12" fill="{t["codeink"]}">{esc(line)}</text>')
    # Column 3: the map memory written without the agent.
    x = 844
    p.append(f'<rect x="{x}" y="124" width="292" height="190" rx="8" fill="{t["code"]}"/>')
    lines = [("#6f7d91", "saved from the transcript:"), (t["codeink"], 'Map: a task like "Add a new'),
             (t["codeink"], 'builtin sign(x)" changed'), (t["ok"], "builtin/registry.go,"),
             (t["ok"], "builtin/lib.go, docs, tests"), ("#6f7d91", "anchors: exists: each file"),
             ("#6f7d91", "agent turns spent: 0")]
    for i, (c, s) in enumerate(lines):
        p.append(f'<text x="{x + 14}" y="{148 + i * 22}" font-family="{MONO}" font-size="12" fill="{c}">{esc(s)}</text>')
    # Arrows between the steps and the loop back.
    for x1 in (380, 770):
        p.append(f'<line x1="{x1 + 4}" y1="190" x2="{x1 + 46}" y2="190" stroke="{t["soft"]}" stroke-width="1.6" marker-end="url(#ah)"/>')
    p.append(f'<path d="M990 340 V378 Q990 392 976 392 H224 Q210 392 210 378 V346" fill="none" stroke="{t["soft"]}" '
             f'stroke-width="1.6" stroke-dasharray="5 5" marker-end="url(#ah)"/>')
    p.append(f'<rect x="420" y="380" width="360" height="26" rx="13" fill="{t["bg"]}"/>')
    p.append(text(600, 398, "every recall re-checks the anchors against your files", 13, "soft", anchor="middle", t=t))
    p.append("</svg>\n")
    return "".join(p)


for name, t in THEMES.items():
    path = os.path.join(out, f"how-it-works-{name}.svg")
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(build(t))
    print(path)
