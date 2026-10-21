#!/usr/bin/env python3
"""Generate the animated SVGs used by the README (assets/*.svg).

Pure standard library. Animations use SMIL with a shared cycle so every
element stays in sync and loops forever; SMIL runs inside <img> tags on
GitHub, unlike scripts. Run from the repo root:

    python3 scripts/gen_assets.py
"""
from html import escape
from pathlib import Path

OUT = Path(__file__).resolve().parent.parent / "assets"

# Palette: warm parchment + deep ink, teal for truth, amber for staleness.
INK = "#1f1d2b"
INK_SOFT = "#4b4860"
PAPER = "#fbf7ef"
PAPER_EDGE = "#efe6d4"
TEAL = "#1f9e8f"
TEAL_SOFT = "#d6f1ec"
AMBER = "#e0912f"
AMBER_SOFT = "#fbe9cf"
LILAC = "#7b6fd6"
LILAC_SOFT = "#e6e2fb"

MONO = "ui-monospace, SFMono-Regular, 'Cascadia Code', Menlo, Consolas, monospace"
SERIF = "'Iowan Old Style', 'Palatino Linotype', Palatino, Georgia, serif"
SANS = "-apple-system, 'Segoe UI', Inter, Helvetica, Arial, sans-serif"


def owl(x: float, y: float, s: float, blink_dur: float = 6.0) -> str:
    """An original, geometric owl mascot. (x, y) is the top-left; s scales a 200x200 box."""
    def lid(cx: float, cy: float) -> str:
        # Eyelid: a body-coloured ellipse whose height animates closed and open again.
        return (
            f'<ellipse cx="{cx}" cy="{cy}" rx="27" ry="0" fill="#5b4e8c">'
            f'<animate attributeName="ry" dur="{blink_dur}s" repeatCount="indefinite" '
            f'values="0;0;27;0;0;27;0;0" keyTimes="0;0.55;0.58;0.61;0.86;0.88;0.9;1"/>'
            f"</ellipse>"
        )

    return f"""
<g transform="translate({x} {y}) scale({s})">
  <g>
    <animateTransform attributeName="transform" type="translate" dur="4s" repeatCount="indefinite"
      values="0 0; 0 -4; 0 0" keyTimes="0;0.5;1" calcMode="spline" keySplines="0.4 0 0.6 1;0.4 0 0.6 1"/>
    <ellipse cx="100" cy="192" rx="58" ry="6" fill="{INK}" opacity="0.08"/>
    <path d="M52 52 L44 18 L80 40 Z" fill="#5b4e8c"/>
    <path d="M148 52 L156 18 L120 40 Z" fill="#5b4e8c"/>
    <path d="M100 30 C150 30 172 70 172 118 C172 162 140 186 100 186 C60 186 28 162 28 118 C28 70 50 30 100 30 Z" fill="#6d5fa6"/>
    <path d="M100 96 C134 96 148 124 148 148 C148 172 126 184 100 184 C74 184 52 172 52 148 C52 124 66 96 100 96 Z" fill="{LILAC_SOFT}"/>
    <g fill="none" stroke="#b9b0ee" stroke-width="3" stroke-linecap="round">
      <path d="M82 132 q6 7 12 0"/><path d="M106 132 q6 7 12 0"/>
      <path d="M70 150 q6 7 12 0"/><path d="M94 150 q6 7 12 0"/><path d="M118 150 q6 7 12 0"/>
      <path d="M84 168 q6 7 12 0"/><path d="M106 168 q6 7 12 0"/>
    </g>
    <circle cx="70" cy="84" r="30" fill="{PAPER}"/>
    <circle cx="130" cy="84" r="30" fill="{PAPER}"/>
    <circle cx="72" cy="86" r="13" fill="{INK}"/>
    <circle cx="128" cy="86" r="13" fill="{INK}"/>
    <circle cx="76" cy="81" r="4" fill="#fff"/>
    <circle cx="132" cy="81" r="4" fill="#fff"/>
    {lid(70, 84)}
    {lid(130, 84)}
    <path d="M92 104 L108 104 L100 118 Z" fill="{AMBER}"/>
    <circle cx="48" cy="108" r="7" fill="#f5a3b5" opacity="0.55"/>
    <circle cx="152" cy="108" r="7" fill="#f5a3b5" opacity="0.55"/>
  </g>
  <g transform="translate(150 140)">
    <circle r="22" fill="{TEAL}">
      <animate attributeName="r" dur="2.4s" repeatCount="indefinite" values="22;24;22" keyTimes="0;0.5;1"/>
    </circle>
    <circle r="22" fill="none" stroke="{TEAL}" stroke-width="3" opacity="0">
      <animate attributeName="r" dur="2.4s" repeatCount="indefinite" values="22;36" keyTimes="0;1"/>
      <animate attributeName="opacity" dur="2.4s" repeatCount="indefinite" values="0.5;0" keyTimes="0;1"/>
    </circle>
    <path d="M-10 0 L-3 8 L11 -8" fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>
  </g>
</g>"""


def chip_width(label: str, text: str) -> float:
    return 22 + 8.6 * len(label) + 16 + 7.3 * len(text)


def chips(x: float, y: float, items: list, gap: float = 14) -> str:
    out = []
    for label, text, fg, bg in items:
        out.append(chip(x, y, label, text, fg, bg))
        x += chip_width(label, text) + gap
    return "".join(out)


def chip(x: float, y: float, label: str, text: str, fg: str, bg: str) -> str:
    w = chip_width(label, text)
    return f"""
<g transform="translate({x} {y})">
  <rect width="{w:.0f}" height="34" rx="17" fill="{bg}"/>
  <text x="16" y="22.5" font-family="{MONO}" font-size="14" font-weight="700" fill="{fg}">{escape(label)}</text>
  <text x="{22 + 8.6 * len(label):.0f}" y="22.5" font-family="{SANS}" font-size="14" fill="{INK_SOFT}">{escape(text)}</text>
</g>"""


def banner() -> str:
    w, h = 1280, 360
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-label="Aletheia: memory for AI agents that checks itself against reality">
  <title>Aletheia</title>
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0" stop-color="{PAPER}"/><stop offset="1" stop-color="{PAPER_EDGE}"/>
    </linearGradient>
    <pattern id="dots" width="22" height="22" patternUnits="userSpaceOnUse">
      <circle cx="2" cy="2" r="1.3" fill="{INK}" opacity="0.07"/>
    </pattern>
  </defs>
  <rect width="{w}" height="{h}" rx="28" fill="url(#bg)"/>
  <rect width="{w}" height="{h}" rx="28" fill="url(#dots)"/>
  <circle cx="1160" cy="40" r="120" fill="{TEAL_SOFT}" opacity="0.7"/>
  <circle cx="1230" cy="330" r="90" fill="{LILAC_SOFT}" opacity="0.8"/>
  {owl(70, 70, 1.1)}
  <text x="350" y="150" font-family="{SERIF}" font-size="92" font-weight="700" fill="{INK}" letter-spacing="-1">Aletheia</text>
  <text x="352" y="196" font-family="{SANS}" font-size="26" fill="{INK_SOFT}">Memory for AI agents that checks itself against reality.</text>
  {chips(352, 236, [
      ("[ok]", "verified against your files", TEAL, TEAL_SOFT),
      ("[?]", "hint, no evidence", LILAC, LILAC_SOFT),
      ("stale", "hidden before it misleads", "#b26a12", AMBER_SOFT),
  ])}
  <text x="352" y="316" font-family="{SERIF}" font-style="italic" font-size="18" fill="{INK_SOFT}" opacity="0.85">a·lḗtheia (ἀλήθεια): truth, literally “un-forgetting”</text>
</svg>
"""


# --- animated terminal --------------------------------------------------------

CYCLE = 24.0  # seconds for one loop
CHAR_W = 8.7  # approximate monospace advance at 14.5px
LINE_H = 25

# (kind, segments, start_time)
#   kind "cmd": typed out; segments rendered after a "$ " prompt
#   kind "out": appears at once
#   kind "note": a comment line between steps
SCRIPT = [
    ("cmd", [("aletheia remember ", "#e8e6f0"), ('"Service targets Go 1.23" ', "#f3d9a4"), ("--anchor file:go.mod", "#9fd6ce")], 0.6),
    ("out", [("saved ", "#a9a6b8"), ("9e4426 ", "#e8e6f0"), ("[ok] ", "#5fd3a8"), ("Service targets Go 1.23", "#e8e6f0")], 3.4),
    ("cmd", [("aletheia recall ", "#e8e6f0"), ('"go version"', "#f3d9a4")], 4.6),
    ("out", [("9e4426 ", "#e8e6f0"), ("[ok] ", "#5fd3a8"), ("Service targets Go 1.23", "#e8e6f0")], 6.4),
    ("note", [("# a teammate bumps go.mod to 1.24 …", "#7d7a91")], 7.8),
    ("cmd", [("aletheia recall ", "#e8e6f0"), ('"go version"', "#f3d9a4")], 9.2),
    ("out", [("(+1 stale hidden: 9e4426)", "#f0b35b")], 11.0),
    ("cmd", [("aletheia get 9e4426", "#e8e6f0")], 12.4),
    ("out", [("9e4426 ", "#e8e6f0"), ("[stale: go.mod: changed] ", "#f0b35b"), ("Service targets Go 1.23", "#a9a6b8")], 14.0),
    ("cmd", [("aletheia remember ", "#e8e6f0"), ('"Service targets Go 1.24" ', "#f3d9a4"), ("--anchor file:go.mod --supersedes 9e4426", "#9fd6ce")], 15.4),
    ("out", [("saved ", "#a9a6b8"), ("51b0c7 ", "#e8e6f0"), ("[ok] ", "#5fd3a8"), ("Service targets Go 1.24", "#e8e6f0")], 19.4),
    ("out", [("superseded: 9e4426", "#a9a6b8")], 19.7),
]
TYPE_SPEED = 0.038  # seconds per character
HOLD_END = 22.6     # everything fades out from here
FADE_END = 23.4


def kt(t: float) -> str:
    return f"{max(0.0, min(1.0, t / CYCLE)):.4f}"


def terminal() -> str:
    w = 980
    top = 58
    h = top + LINE_H * len(SCRIPT) + 30
    rows = []
    for i, (kind, segs, start) in enumerate(SCRIPT):
        y = top + 18 + i * LINE_H
        x0 = 28
        text = "".join(s for s, _ in segs)
        spans = "".join(f'<tspan fill="{c}">{escape(s)}</tspan>' for s, c in segs)
        # Visibility of the whole row over the cycle.
        vis = (
            f'<animate attributeName="opacity" dur="{CYCLE}s" repeatCount="indefinite" '
            f'values="0;0;1;1;0;0" keyTimes="0;{kt(start)};{kt(start + 0.01)};{kt(HOLD_END)};{kt(FADE_END)};1"/>'
        )
        if kind == "cmd":
            prompt = f'<text x="{x0}" y="{y}" font-family="{MONO}" font-size="14.5" fill="#5fd3a8">$</text>'
            tx = x0 + 2 * CHAR_W
            full = len(text) * CHAR_W + 4
            typed_end = start + len(text) * TYPE_SPEED
            clip_id = f"c{i}"
            rows.append(
                f'<clipPath id="{clip_id}"><rect x="{tx}" y="{y - 16}" height="22" width="0">'
                f'<animate attributeName="width" dur="{CYCLE}s" repeatCount="indefinite" '
                f'values="0;0;{full:.0f};{full:.0f};0" keyTimes="0;{kt(start)};{kt(typed_end)};{kt(FADE_END)};1"/>'
                f"</rect></clipPath>"
                f'<g opacity="0">{vis}{prompt}'
                f'<text x="{tx}" y="{y}" font-family="{MONO}" font-size="14.5" clip-path="url(#{clip_id})" xml:space="preserve">{spans}</text></g>'
            )
        else:
            style = ' font-style="italic"' if kind == "note" else ""
            rows.append(
                f'<g opacity="0">{vis}<text x="{x0}" y="{y}" font-family="{MONO}" font-size="14.5"{style} xml:space="preserve">{spans}</text></g>'
            )
    # Cursor: at the end of a command while it is typed, then on the empty
    # line below the latest output.
    cursor_vals, cursor_keys = [], []
    for i, (kind, segs, start) in enumerate(SCRIPT):
        if kind == "cmd":
            n = len("".join(s for s, _ in segs))
            cursor_keys.append(kt(start + n * TYPE_SPEED))
            cursor_vals.append(f"{(n + 2) * CHAR_W + 4:.0f} {i * LINE_H}")
        cursor_keys.append(kt(start + (n * TYPE_SPEED if kind == "cmd" else 0) + 0.02))
        cursor_vals.append(f"0 {(i + 1) * LINE_H}")
    cursor = (
        f'<rect x="{28}" y="{top + 4}" width="9" height="17" rx="2" fill="#5fd3a8" opacity="0.85">'
        f'<animateTransform attributeName="transform" type="translate" dur="{CYCLE}s" repeatCount="indefinite" calcMode="discrete" '
        f'values="{";".join(["0 0"] + cursor_vals)}" keyTimes="{";".join(["0"] + cursor_keys)}"/>'
        f'<animate attributeName="opacity" dur="1s" repeatCount="indefinite" values="0.85;0;0.85" keyTimes="0;0.5;1" calcMode="discrete"/>'
        f"</rect>"
    )
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-label="Terminal demo: a memory is verified, goes stale when go.mod changes, is hidden from recall, and is superseded">
  <title>Aletheia demo</title>
  <rect width="{w}" height="{h}" rx="16" fill="#1d1b26"/>
  <rect width="{w}" height="40" rx="16" fill="#26233a"/>
  <rect y="24" width="{w}" height="16" fill="#26233a"/>
  <circle cx="24" cy="20" r="6" fill="#ff6b6b"/><circle cx="44" cy="20" r="6" fill="#ffc24b"/><circle cx="64" cy="20" r="6" fill="#4cd07d"/>
  <text x="{w / 2}" y="25" text-anchor="middle" font-family="{SANS}" font-size="13" fill="#8e8aa6">aletheia · stale memory never reaches the context</text>
  {cursor}
  {"".join(rows)}
</svg>
"""


def states() -> str:
    """Three-card diagram of the freshness states."""
    w, h = 980, 210
    cards = [
        (TEAL, TEAL_SOFT, "[ok]", "verified", "Every anchor still holds against", "the files right now."),
        (LILAC, LILAC_SOFT, "[?]", "unanchored", "No evidence attached: a hint,", "ranked below verified facts."),
        ("#b26a12", AMBER_SOFT, "stale", "hidden", "An anchor failed or it expired.", "Kept on disk, never sent."),
    ]
    out = []
    cw, gap = 300, 30
    for i, (fg, bg, tag, name, l1, l2) in enumerate(cards):
        x = 10 + i * (cw + gap)
        out.append(f"""
<g transform="translate({x} 10)">
  <rect width="{cw}" height="{h - 20}" rx="18" fill="{bg}"/>
  <rect x="22" y="24" width="{24 + 10 * len(tag)}" height="32" rx="10" fill="#fff"/>
  <text x="34" y="46" font-family="{MONO}" font-size="16" font-weight="700" fill="{fg}">{escape(tag)}</text>
  <text x="22" y="98" font-family="{SERIF}" font-size="28" font-weight="700" fill="{INK}">{escape(name)}</text>
  <text x="22" y="134" font-family="{SANS}" font-size="14.5" fill="{INK_SOFT}">{escape(l1)}</text>
  <text x="22" y="156" font-family="{SANS}" font-size="14.5" fill="{INK_SOFT}">{escape(l2)}</text>
</g>""")
    arrows = []
    for i in range(2):
        x = 10 + (i + 1) * cw + i * gap + 8
        arrows.append(f'<path d="M{x} {h / 2} h{gap - 16}" stroke="{INK_SOFT}" stroke-width="2" stroke-dasharray="4 4" opacity="0.5"/>')
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-label="Freshness states: verified, unanchored, stale">
  <title>Freshness states</title>
  {"".join(arrows)}{"".join(out)}
</svg>
"""


def mascot() -> str:
    return f"""<svg xmlns="http://www.w3.org/2000/svg" width="120" height="120" viewBox="0 0 220 220" role="img" aria-label="Aletheia owl">
  <title>Aletheia owl</title>
  {owl(10, 10, 1.0)}
</svg>
"""


def main() -> None:
    OUT.mkdir(exist_ok=True)
    files = {"banner.svg": banner(), "demo.svg": terminal(), "states.svg": states(), "owl.svg": mascot()}
    for name, svg in files.items():
        (OUT / name).write_text(svg, encoding="utf-8")
        print(f"wrote assets/{name} ({len(svg.encode()):,} bytes)")


if __name__ == "__main__":
    main()
