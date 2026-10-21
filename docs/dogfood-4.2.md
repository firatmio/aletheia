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

## 6. Lab runs: a real Claude Code agent (headless)

Setup:
- Lab repo `C:\source\aletheia-lab` ("shorty", a URL shortener using only the Go standard library, with a decisions doc).
- `claude -p` (Claude Code 2.1.168, Opus 5.5) runs in a clean environment. Aletheia is connected through `--mcp-config` and `--strict-mcp-config`.
- Prompts never mention Aletheia. Logs are in `lab-runs/` (git-excluded).

### R1: empty memory, `AGENTS.md` only, task "add DELETE /api/links/{code} with tests"
- MCP was connected and all 4 tools were listed. **0 recall, 0 remember.** The task succeeded in 14 turns, 42 s, about $0.23.
- The agent **did read `AGENTS.md`**, but only because it showed up in a `Glob **/*`. That was the 5th call, after it had already read three source files. It then ignored both instructions ("recall before exploring", "remember durable facts").
- Likely reason the MCP descriptions had no effect: the session has `ToolSearch`. In that mode Claude Code defers MCP tools and the model sees only their **names**, not the descriptions. "Search project memory before exploring the codebase" probably never reached the model. R1c below confirms it.
- Claude Code has its own **auto-memory** directory (`~/.claude/projects/<proj>/memory/`). It stayed empty here, but it is the built-in competitor Aletheia must coexist with.
- Harness noise: 2 turns were lost to denied `PowerShell` calls (not in `--allowedTools`).

### R1b (snippet in `CLAUDE.md`) and R1c (`AGENTS.md` only, `ENABLE_TOOL_SEARCH=false`)
Same task and the same pre-R1 commit for both.

| Run | Instructions reach the model through | First call | remember | Turns | Cache create / read | Cost |
|---|---|---|---|---|---|---|
| R1 | `AGENTS.md` (only found by Glob) | Glob | 0 | 14 | 17.9k / 185k | $0.23 |
| R1b | `CLAUDE.md` (auto-loaded) | `aletheia recall` **via Bash CLI** | 1 (CLI) | 13 | 17.3k / 132k | $0.22 |
| R1c | MCP descriptions (tool search off) | `mcp__aletheia__recall` | 1 (MCP) | 13 | 44.8k / 205k | $0.46 |

- **Where the instructions live decides everything.**
  - `CLAUDE.md` works.
  - MCP descriptions work only when tool search is off. That doubled the session cost, almost entirely because every *other* deferred tool got loaded, not because of Aletheia.
  - `AGENTS.md` alone does nothing for Claude Code.
  - → `init --agents` must write to `CLAUDE.md` (or both files).
- With `CLAUDE.md`, the agent used the **CLI through Bash**, not the MCP tools, even though MCP was connected. The snippet names the CLI first, and the MCP tools were deferred.
- Both runs recalled first; the store was empty, so there was nothing to gain yet. Both remembered **exactly one** memory, at the very end, and in both cases it was the security note from the agent's final answer.
- The memories reproduce the failure modes predicted in sections 2 and 5, this time from a real agent:
  - R1b: "API routes have no authentication…" anchored by `file:handlers.go` (the busiest file, so the next edit stales it) and by `contains:handlers.go::DELETE /api/links/\{code\}` (a proxy). If auth is added and the route string stays, the claim becomes false but still shows `[ok]`.
  - R1c: one memory holding three claims (route style, store flushing, no auth), anchored by two proxies (the DELETE route string and the `Store.Delete` signature). None of the anchors evidences any of the three claims.
- Takeaways for the snippet and tool descriptions:
  - one fact per memory;
  - anchor the line that *proves* the claim;
  - do not use `file:` on busy files;
  - "absence" claims (no auth, no lock) get no anchor and a TTL instead.

### R2: does recalled memory shorten exploration?
- **Setup:** CLAUDE.md, starting from the end of R1b.
- **Task:** "protect DELETE with `SHORTY_ADMIN_TOKEN` (401/403) + tests".
- **Runs:** two with R1b's single memory (`mem`), two with the memory removed (`ctrl`).

| Run | Recall at start | Exploration | Turns | Cache read | Cost |
|---|---|---|---|---|---|
| mem-1 | `8360ca [ok] …no authentication…` | cat config/handlers/main/AGENTS/decisions + tests, README | 19 | 318k | $0.39 |
| mem-2 | same | same | 17 | 282k | $0.33 |
| ctrl-1 | none | same | 16 | 253k | $0.33 |
| ctrl-2 | none | same | 19 | 296k | $0.41 |

- **No measurable saving.** The averages are 18 turns / $0.36 with memory and 17.5 turns / $0.37 without, which is within run-to-run noise.
  - The exploration was identical: every run `cat`s the whole repo in one or two calls. In a 6-file repo, reading everything costs less than trusting a memory.
  - The one recalled memory (a security note) is not navigation knowledge, so it could not replace any read.
  - **The token claim cannot be tested on a toy repo.** It needs a repo where exploration is expensive.
- **The stale trap was handled well, by the agent rather than by the anchors.** Both `mem` runs saw `[ok] …no authentication…` and, after adding auth, wrote a new memory with `--supersedes 8360ca`. The supersede workflow in the snippet is understood and used.
- **Memory quality repeats R1b's problems:**
  - **All 4 runs anchored `file:handlers.go`.** That is the busiest file, so the next edit stales the memory. This is systematic, not a one-off.
  - The claims are compound again. Two of them carry an "other routes are unauthenticated" absence claim that no anchor can evidence.
- Harness noise, equal across groups:
  - With `core.autocrlf=true`, branch checkouts gave CRLF files, so `gofmt -l` flagged every file and agents spent 2–3 turns on it. Fixed by setting `core.autocrlf=false` in the lab repo.
  - `python -` heredocs were denied, and the agents fell back to Edit.

### v2: after the fix (`init --agents` writes CLAUDE.md + AGENTS.md; anchor rules in the snippet and the `remember` description)
Same two tasks, run twice: R1v2-i (DELETE task from the pre-R1 commit), then R2v2-i (admin-token task on top of it).

| | Before (R1b, R1c, 4×R2) | After (2×R1v2, 2×R2v2) |
|---|---|---|
| Sessions that recalled first | 6/6 | 4/4 |
| Memories written | 6 (1 per session) | 11 (2–3 per session) |
| Compound claims | 6/6 | 0/11 (some still state slightly more than the anchor shows, e.g. "routed to `remove`, **returns 204/404**") |
| `file:` anchors | 5/6 | **0/11** |
| Absence claims ("no auth") | anchored with proxies (false-`[ok]` risk) | 3, all **unanchored with `--ttl 30d`** |
| Anchors on the proving line | 0 | 5 strong (e.g. the `subtle.ConstantTimeCompare(...)` line, the `AdminToken == ""` check), 3 partial |
| Old "no auth" memory superseded after adding auth | 2/2 | 2/2 |
| Cost per session | R1 ≈ $0.22, R2 ≈ $0.36 | R1 ≈ $0.22, R2 ≈ $0.32 |

- The rules in the snippet changed the agents' behaviour right away: one fact per memory, no `file:`, absence claims put on TTL.
- What is still open:
  - Claims are sometimes a little broader than their anchor (status codes mentioned but not anchored).
  - Exploration in this toy repo is still "cat everything", so the token claim still needs a bigger repo.
  - Harness noise: one denied `python -` heredoc per R2 run.

## 7. Engine fixes, measured (branch `feat/anchor-robustness`)

Three changes from section 5:
1. **Whitespace-tolerant `contains:`.** A run of spaces/tabs in the pattern matches any run of spaces/tabs. Escaped spaces, spaces inside character classes and quantified spaces keep their meaning.
2. **Comment-only warning.** `remember` adds a note when a `contains:` anchor matches only inside comments. This is a heuristic per language extension; unknown languages are never flagged.
3. **Default TTL for unanchored memories.** They expire after 90 days unless `--ttl` is given. `verify --reseal` without a ttl renews them for another 90 days.

The section 2 edit experiment was re-run on the same tree (commit `87e114d`, the 25 dogfood memories) with the old and new binaries:

| Outcome | Old | New |
|---|---|---|
| caught | 7 | 7 |
| NEEDLESS_STALE | 6 | **4** (gofmt realignment no longer stales: H2, H3) |
| FALSE_OK at verify time | 4 | 4 (unchanged: the fix acts at write time) |
| …of which flagged when saved | 0 | **2** (S5 "never executes" doc comment, S6 "(default 30)" field comment); no false warnings on the other 23 memories |
| SILENT_HINT | 3 | 3 today, but all 6 unanchored memories now expire after ≤90 days instead of never |

Still not covered:
- S3 (a function name that still appears elsewhere) and S8 (an unused constant): these need `symbol:` anchors or usage-aware checks.
- H1 (`file:` on any edit), H4 (reordered literal), H6 (a list grew) and H7 (an anchored comment was reworded; the new warning now discourages that anchor).

A real stale event happened along the way. PR #1 changed `installAgents`:
- Memory `673a56` ("init --agents appends … to AGENTS.md") went stale, which is correct.
- Memory `ddeb31` ("init --agents alone does not reach Claude Code") silently became false. It is unanchored, so only the new TTL would ever retire it.
