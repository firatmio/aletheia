# Dogfood log: step 4.2 (2026-10-21)

The first real use of Aletheia on its own repository. Claude Code (Opus 5.5) wrote the memories through the CLI. MCP was registered (`claude mcp add -s local aletheia -- aletheia serve`), but tools only show up in a new session, so how an agent uses MCP is **not** observed yet.

`.aletheia/` is local only (listed in `.git/info/exclude`), as PLAN.md asks.

## 1. What was recorded

25 memories about this repo, written the way an agent would after working here.

| Class | Count | Meaning | Examples |
|---|---|---|---|
| Strong anchor | 13 (52%) | The anchor tests the claim itself | recall defaults, id regex, freshness factors, CI matrix, where `Judge` lives |
| Proxy anchor | 6 (24%) | The anchor tests a name, comment, constant or whole file, not the behaviour | "no dependencies" → `file:go.mod`; "never executes" → a doc comment; "CRLF normalized" → the function name |
| No honest anchor | 6 (24%) | Decisions, rationale, environment, absence of something | GitHub account restricted; why BM25; why Markdown is the source of truth; no cross-process lock |

So about half the memories can be anchored honestly. The other half are either proxies or unanchored.

What kind of memory this was matters too. Most strong anchors cover facts an agent could rediscover by reading one line (constants, regexes). The memories that would save the most exploration (decisions, environment, "why") are the ones that cannot be anchored.

## 2. Edit experiment: false `[ok]` and needless stale

A copy of the repo got 21 realistic edits, one at a time. After each edit, `aletheia verify` was compared with the ground truth of every claim. The script is `experiment.py` in the session scratchpad (not committed, because it hard-codes local memory ids).

- **Harmless edits** (7): every claim stays true.
- **Semantic edits** (14): the edit makes exactly one claim false.

| Outcome | Count | Meaning |
|---|---|---|
| caught | 7 | Claim went false, memory hidden as stale ✅ |
| **FALSE_OK** | **4** | Claim went false, memory still shown as `[ok]` ❌ |
| **SILENT_HINT** | **3** | Claim went false, memory still shown as `[?]`; nothing can ever flag it |
| **NEEDLESS_STALE** | **6** | Claim still true, memory hidden anyway |
| quiet and correct | the rest | — |

Of the 14 semantic edits, 7 were caught (50%). Of the 7 harmless edits, 6 hid a true memory (86%).

### False `[ok]` (the dangerous ones)
| Edit | Why the anchor still held |
|---|---|
| `hashFile` stops normalizing CRLF | The anchor checks that `normalizeText(` exists, and it still does in `readHead` |
| A `cmd:` anchor kind is added | The anchor points at a doc comment ("It never executes"), and the comment was left alone |
| `lethe` default changed from 30 to 14 days | The anchor points at the field comment "(default 30)" |
| The similarity check hard-codes 0.8 | The anchor checks the constant `SimilarityThreshold = 0.6`, which nobody uses any more |

All 4 came from **proxy anchors**. Strong anchors produced no false `[ok]` in this run.

### Needless stale (memory loss)
| Edit | Cause |
|---|---|
| `go` directive bumped from 1.24.7 to 1.24.9 | `file:go.mod` breaks on any edit |
| A new const makes gofmt realign the block | The regex had the exact column spacing (`DefaultLimit  = 8`) |
| A new var makes gofmt realign the `var` block | Same: `idPattern  =` |
| Map literal entries reordered | The regex fixed the order |
| One more MCP protocol version accepted | The regex fixed the whole list |
| A doc comment reworded | The anchor was the comment text |

Four of the six come from regexes that are too literal (whitespace twice, order, a full list). One comes from anchoring a comment, and one from `file:`.

### Caveat
The same agent wrote the anchors and designed the edits, so these numbers show **failure modes, not rates**. Real rates need time: run `aletheia verify` weekly while the code actually changes and log each stale or false `[ok]` here.

## 3. Recall quality (8 natural questions, `--limit 3`)

- Top-1 relevant: 4/8. Relevant in top 3: 6/8.
- Misses:
  - "how are memories ranked": `ranked` → `ranked` and `ranking` → `rankin` with the 6-rune stemmer, so they never match.
  - "what happens when two memories say the same thing": `same` vs `similarity`, a synonym BM25 cannot bridge.
- "why not sqlite database": an irrelevant `[ok]` memory came first because of the word "not". The relevant `[?]` decision came second; the 0.7 factor pushes decisions down.
- Words like "memories", "not" and "why" match almost everything ("+7 more" tails). There is no stopword list.

## 4. Other observations

- **`init --agents` writes `AGENTS.md`, but Claude Code reads `CLAUDE.md`.** (Recorded as an unverified `[?]` memory; check against current Claude Code docs.) If that holds, Claude Code never sees the instructions unless the user also edits CLAUDE.md.
- Writing good anchors is expensive. Every anchor needed a grep for the exact line plus regex and shell escaping. An agent pays that in tokens on every `remember`.
- TTL exists, but the agent (me) did not use it for the environment fact ("account restricted"). That memory will stay `[?]` and quietly wrong once the debt is paid. Tooling cannot rely on agent discipline here.
- No duplicate/similar conflicts were triggered across 25 memories; the Jaccard check stayed quiet.

## 5. Ideas this suggests (not implemented; discuss first)

1. **Whitespace-tolerant `contains:`.** Match with runs of spaces and tabs collapsed, so gofmt realignment does not stale anything. This removes 2 of the 6 needless stales.
2. **Refuse or warn on comment-only anchors at write time.** If a `contains:` pattern only matches inside a comment, say so. This would have prevented 2 of the 4 false `[ok]` (and one needless stale). The other two (an unused constant, a function name that still appears elsewhere) need a different kind of anchor; see 6.
3. **Default TTL for unanchored memories** (e.g. 90 days), so `[?]` hints decay and need re-confirming instead of living forever.
4. **Recall:**
   - a small stopword list (EN + TR);
   - a stemmer that does not split `ranked`/`ranking`;
   - reconsider the 0.7 factor for `[?]` when the relevance gap is large.
5. **AGENTS.md / CLAUDE.md:** have `init --agents` also handle CLAUDE.md, or print the one line to add.
6. The `symbol:` anchor on the roadmap would turn most proxy anchors on names into strong ones.
