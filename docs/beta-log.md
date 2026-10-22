# Beta log

We use Aletheia on its own repository while we work on it, the site included. The lab runs (`docs/dogfood-4.2.md`) measure cost against a control arm. This log covers what they cannot: how the tool behaves in daily use, where it gets in the way, and which memories turn out wrong.

## Setup

- **Version:** `0.1.0-beta.1`, built from `main` (`go install -ldflags "-X main.version=0.1.0-beta.1" ./cmd/aletheia`).
- **Hooks:** `UserPromptSubmit` and `Stop` in `.claude/settings.local.json`. That file is git-ignored, so the hooks are not pushed on anyone who clones the repo.
- **MCP:** `aletheia serve`, registered with local scope.
- **Memories:** `.aletheia/` stays out of git (`.git/info/exclude`) until we decide whether dogfood memories belong in the public repo (PLAN 7.1).

## What to record

One entry per finding. Write down:
- **What happened:** the prompt or command, and what Aletheia did.
- **Kind:**
  - `recall`: relevance;
  - `false-ok`: an `[ok]` memory that was wrong;
  - `missed-stale`: a change that should have staled a memory and did not;
  - `needless-stale`: a true memory went stale;
  - `friction`: something that got in the way;
  - `good`: it helped.
- **Follow-up:** a fix, a test, or nothing.

At the end of each week, also log `aletheia verify` and `aletheia stats`.

## 2026-10-22: setup

**B1 · friction · upgrading on Windows while the MCP server runs.**
- **What happened:** `go install` failed with "file in use": the running `aletheia serve` holds `aletheia.exe`, and Windows does not let a running exe be overwritten. Renaming the old exe first works, because Windows allows renaming a running exe.
- **Follow-up:** done on branch `docs/windows-upgrade`. The README's quick start and `/agents.md` now have an upgrade note: quit the agent, or rename the old binary and install. The MCP server keeps running the old version until the agent restarts.

**B2 · recall · recalled memories off topic, code map on target.**
- **What happened:** the prompt was "how does the stop hook capture task maps".
  - Memories: 6 were recalled, and none was about capture. Examples: zero dependencies, the 4 MiB limit of `contains:`, the Jaccard check.
  - Code map: led with `cmd/aletheia/capture.go: captureStop 49-118` and `internal/engine/capture.go: Engine.CaptureMap 32-81`. That is exactly where to read.
- **Follow-up:** drop memories below a relevance floor rather than filling the limit; the store has no capture memory at all yet.

**B3 · good · two stale memories were really outdated, fixed in one call.**
- **What happened:** both memories were wrong, not merely stale:
  - `673a56`: `init --agents` now writes to CLAUDE.md as well as AGENTS.md.
  - `df04d4`: `assets/banner.svg` was removed in the README redesign.
  
  Both were superseded with a single `aletheia remember -` call. The regexes were written with single backslashes, as a shell hands them over (`\[`, `\.`), and were accepted. Stale count: 2 → 0.
- **Follow-up:** none.

**B4 · needless-stale (latent) · an automatic anchor on an untracked file.**
- **What happened:** the claim mentions CLAUDE.md and AGENTS.md, so `remember` added `exists:AGENTS.md`. In this repo AGENTS.md is untracked (dogfood output). On a fresh clone the memory `ae03a3` would be stale for a reason unrelated to its claim.
- **Follow-up:** consider skipping automatic anchors on files git does not track, or noting them when saving.

**B5 · good · the hooks were active right away.**
- **What happened:** we expected the hooks to start in the next session. They did not wait for one:
  - The prompt hook already ran on the next prompt after `settings.local.json` was written.
  - The Stop hook had saved two maps from this session's edits, one for the batch-remember task and one for the beta setup.
- **Follow-up:** none; README can say hooks apply without a restart in current Claude Code.

**B6 · needless-stale · memories are blind to branches.**
- **What happened:** the map for the beta setup anchors `exists:docs/beta-log.md`. That file exists only on the `docs/beta-log` branch, so the memory went stale and was hidden as soon as we switched to `main` for the next fix. The check is correct for the files on disk, but the memory is not wrong; it belongs to an unmerged branch.
- **Follow-up:** consider recording the branch in captured maps and saying "on branch X" instead of hiding, or simply let these come back on merge. Watch how often it happens.

**B2, resolved** on branch `fix/recall-relevance-floor`. There were three causes, each with its own fix:
- **Filler words in the query:** words like "how" and "does" are now dropped from queries.
- **The captured-map template:** its prefix ("Map: a task like") is no longer indexed.
- **Weak matches:** a new relevance floor at 30% of the best match drops them.

On this repo's store the B2 query now returns nothing; there is no capture memory. "how is recall ranked" returns the two ranking memories.

## 2026-10-22: after the B2 fix (0.1.0-beta.2)

**B7 · recall · Turkish prompts and short numbers.**
- **What happened:** the prompt was "#10 u merge ettim ama #11 hala duruyor haber olsun, eğer onun da merge edilmesi gerekirse dersin bana."
- **What the hook recalled:**
  - three maps captured from earlier Turkish prompts, which matched on Turkish function words ("bu", "ben", "ama", "olacak");
  - an MCP memory, because "#11" matched the protocol version "2024-11-05".
  
  Most prompts in this repo are Turkish, so in our own use this was a bigger source of noise than B2.
- **Follow-up:** fixed on branch `fix/recall-turkish-stopwords`. The stopword list gains Turkish function words and prompt filler, and numbers of one or two digits are no longer query terms. Years and ports still are.
- **Result:**
  - the same prompt recalls nothing;
  - "Siteyi vercel cli ile yayınlarsın" still finds the site map;
  - "recall nasıl sıralanıyor" finds the ranking memories, the same as "how is recall ranked".

## 2026-10-22: an agent sets Aletheia up in another project

Fırat asked a Claude Code session in another project to set Aletheia up from `/agents.md`. These are the session's own report and what happened around it.

**B8 · friction · the setup flow assumed the agent may configure itself.** Five problems, most serious first:

1. **Hooks are the user's step.** Writing `.claude/settings.json` was blocked as self-modification. This is Claude Code's safety model, not a bug: a hook runs a command on every prompt. The guide told the agent to add the hooks, and the session was flagged as suspicious when it tried.
2. **The guide's commands read like prompt injection.** Imperative lines such as "follow this page top to bottom" and "tell the user and stop" made the agent distrust the page. It only went on because Fırat asked directly. WebFetch also summarized the page, so commands and JSON could be altered on the way.
3. **The guide ignored the local state.** It said `go install` fails and not to try other ways, while `aletheia` was already installed on that machine.
4. **No help for the hook.** `aletheia hook --help` gave "unknown flag", and the stdin format was undocumented. The agent built a guessed test input, and running it was blocked as well.
5. **Cold start.** After `init`, recall finds nothing (correct, but no value on day one), and `init` did not say what comes next.

**Fixed on branch `feat/safer-setup`:**
- **`aletheia init --claude-hooks`:** merges the two hooks into `.claude/settings.json`, keeps other settings and their order, does nothing on a second run, and refuses broken JSON instead of overwriting it. The guide and README say to run it yourself.
- **`/agents.md` rewritten:**
  - It describes rather than commands, and says which steps are the user's.
  - It is shorter (3.5k characters, was about 5.5k), with everything meant to be copied verbatim in short code blocks.
  - It is served as `text/plain`.
  - Its first step is to check `aletheia version`.
- **Help:**
  - Every subcommand takes `-h`/`--help`.
  - `hook --help` documents both stdin formats.
  - `aletheia hook --dry-run "<prompt>"` shows what the hook would add, without reading stdin or writing state.
- **`init` prints the next steps:**
  - `--agents` and `--claude-hooks` when they are not done yet;
  - the MCP block;
  - the dry-run check;
  - that the code map works from the first prompt.
- **Not done:** a separate bootstrap scan for the cold start. The code map already gives value from the first prompt; the `init` output now says so.

## 2026-10-22: the second agent setup, after B8

A new session in the same project followed the new guide.
- **What went right:**
  - It did steps 1, 2 and 4.
  - It handed step 3 (`init --claude-hooks`) to Fırat without touching its own hooks.
  - It committed nothing.
- **Install:** it installed `main` from GitHub with Fırat's `gh` credentials and `GOPRIVATE`, after renaming the old exe as the Windows note says.

**B9 · friction · five smaller problems.**
1. **Private install:** `go install` without credentials fails with a misleading error. Go resolves a pseudo-version from GitHub, then says "could not read Username". The guide said it would fail but gave no recipe.
2. **WebFetch:** it still summarized the guide and dropped the hook JSON and command details. The agent fetched the exact text with curl.
3. **Ignored instruction file:** CLAUDE.md is git-ignored in that project. `init` said it added the note there, but the note never reaches git.
4. **Next steps:** beta.3 did not print them. 0.1.0 from `main` does (B8). No action.
5. **Dry-run example:** for "how is this project built" on an empty store, the code map showed unrelated files, matched on "run" and similar. Build files (go.mod, CI config) are not in the code map at all, since it indexes declarations. A weak first impression.

**Fixed on branch `fix/setup-feedback`:**
- **Guide:**
  - It opens with the whole setup in four commands, so a summary keeps them.
  - It gives the pre-release install recipe for users with access: `GOPRIVATE` plus `gh auth setup-git`, with the note that this changes the user's git config.
  - The quick-start prompt says `curl -fsSL` gets the exact text.
- **`init --agents`:** says when git ignores CLAUDE.md or AGENTS.md ("the note stays on this machine").
- **Dry-run example:** examples use `aletheia hook --dry-run` without a prompt, which shows the overall code map and memories. Help also shows one prompt example.
- **`aletheia version`:** for a build from source it adds the module version Go recorded, e.g. `0.1.0 (v0.0.0-20261022130735-0d57de42de32)`, so the commit is known.
- **Not done:** indexing build and CI files in the code map. Worth a look once the map is measured again.
