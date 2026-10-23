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

## 2026-10-22: the third agent setup, after B9

The same project, a new session. Three of B9's points held:
- **Install:** the `GOPRIVATE` recipe worked; `gh auth setup-git` had already been done there.
- **Ignored file:** `init` noted the git-ignored CLAUDE.md, in that repo and in a fresh test repo.
- **Next steps:** `init` printed them, consistent with the guide.

**B10 · friction · two left.**
1. **Old cached guide.**
   - **What happened:** WebFetch returned the guide as it was before the last deploy: "go install doesn't work yet", no `GOPRIVATE` step. WebFetch keeps fetched pages for a while.
   - **Cause:** client-side. Vercel already serves `Cache-Control: public, max-age=0, must-revalidate`.
2. **"How is this project built" still answered with unrelated files.**
   - **What happened:** the map showed `internal/corpus/run.go` and `internal/policy/poisoning.go`, matched on "project". go.mod, scripts/, `.gitlab-ci.yml` and `.github/` never showed: the code map indexes declarations, and build files have none.

**Fixed on branch `feat/codemap-build-files`:**
- **Date line:** the guide opens with "Updated <build date>" and says `curl -fsSL` gets the current text, so an agent can tell a cached copy may be old.
- **Build files:** the code map records them:
  - manifests at the root and one folder down (go.mod, package.json, Cargo.toml, pyproject.toml, Makefile, Dockerfile and similar);
  - `scripts/`;
  - `.gitlab-ci.yml` and `.goreleaser.yaml`;
  - `.github/workflows/*`.
  
  A prompt about building, testing or shipping, and the overview without a prompt, now start with `build and CI: …`.
- **Stopwords:** "project", "repo", "repository" and "codebase" are now stopwords, so they no longer pull in files.
- **Result on this repo:** "how is this project built" leads with `build and CI: .goreleaser.yaml, go.mod, …, .github/workflows/ci.yml, .github/workflows/release.yml`. "add a CLI flag to recall" gets no build line.

## 2026-10-22: the fourth agent setup, after B10

What held:
- **Install:** the PowerShell `GOPRIVATE` command worked on the first try.
- **The guide:** WebFetch served the current guide this time, date line included. It still summarizes, but the instructions survive the summary.
- **init:** the ignored-file note and the next steps work.
- **Build line:** it appears in the map.

**B11 · friction · two left.**
1. **The Windows upgrade command fails on the second upgrade.**
   - **What happened:** `Rename-Item … aletheia-old.exe` fails when `aletheia-old.exe` already exists ("Cannot create a file when that file already exists"). The install still finished because the exe was not running at the time. With the hooks active, the exe is locked and the upgrade would fail.
   - **Also:** the line after the private-install block started with a stray space. The template had `${privateInstall} To upgrade…`.
2. **Unrelated files under the build line.** "how is this project built" still listed `internal/corpus/run.go` and `poisoning.go` under the build line: weak matches that are only noise.

**Fixed on branch `fix/windows-upgrade-and-map-noise`:**
- **Upgrade command:** it removes `aletheia-old*.exe` copies that are no longer in use, then renames the current exe to `aletheia-old-<random>.exe`. A locked old copy stays and cannot collide. Tested twice in a row with an old copy already present. README and the guide use it, and the stray space is gone.
- **Build-only questions:** when every topic word is about building, testing or shipping, the map is the build line alone.
- **Relevance floor:** the code map gets the same floor as recall: files below 30% of the best match are dropped. On this repo, "add a CLI flag to recall" went from six files to the two that matter (`cmd/aletheia/main.go`, `internal/engine/recall.go`).

## 2026-10-22: the fifth agent setup, after B11 (`0.1.0 (v0.0.0-20261022134418-9e1c7df1c841)`)

The agent moved the old Aletheia files aside, fetched the guide with curl and followed it exactly.
- **B9 to B11:** resolved.
  - "how is this project built" returns only the build line.
  - The new Windows upgrade command removed the old `aletheia-old.exe` and renamed the current exe without a collision.
- **Init, dry-run and stats:** ran without errors.

**B12 · open, for launch · left as notes, not fixed yet.**
1. **WebFetch cache.** WebFetch still returned the previous Windows upgrade command after the deploy. An agent that only uses WebFetch would hit the bug fixed in B11.
   - **Weak date line:** "Updated 2026-10-22" is the same in all four versions of the day, so an agent cannot tell its copy is old.
   - **Idea:** put a build stamp in the guide, such as a commit hash or a timestamp, that an agent can compare with `aletheia version` or with the curl result.
2. **Dry-run heading.** Without a prompt the heading still says "Code map for this prompt". It should say "Code map" or "Project code map".
3. **Not Aletheia's problem, worth knowing.** Claude Code's auto mode blocked deleting the project's old Aletheia files during the reset. `Remove-Item "$bin\aletheia-old*.exe"` passed.

## 2026-10-22: 0.1.0-beta.4 (main b9bcfb9, with [moved])

**B13 · good · the Windows upgrade command, live.** We upgraded with exactly the guide's three PowerShell lines while this session's `aletheia serve` held the exe open.
- An earlier old copy that was no longer in use was deleted.
- The locked exe was moved aside as `aletheia-old-<random>.exe`.
- `go install` succeeded.

`aletheia version` now shows the commit: `0.1.0-beta.4 (v0.0.0-20261022184255-b9bcfb902aed+dirty)`. `+dirty` comes from the untracked files in this checkout (AGENTS.md, launch.json).

**B14 · good · a memory went stale because of our own change.** `175ba5` said the freshness factors were "verified 1, unverifiable 0.7, stale 0.3". Adding `Moved: 0.9` in #19 broke its `contains:` pattern, and it was hidden as stale, correctly: the claim had become incomplete. It was superseded by `b90953`, anchored with a sealed `symbol:internal/engine/recall.go::freshnessFactor` and the new `contains:` line. Stale count: 1 → 0.

**B15 · finding · agents rarely use symbol anchors, so [moved] covers little so far.**
- **The count:** across this repo's 34 active memories, the anchors are 58 `exists:` (mostly captured maps), 28 `contains:`, 2 `file:` and 1 `symbol:`. That one we wrote ourselves.
- **What it means:** `[moved]` only helps sealed symbol anchors, so in our own store it currently protects one memory. The snippet already says to prefer `symbol:` for claims about one declaration; agents still reach for `contains:`.
- **Follow-up:** worth measuring in the next lab run, and possibly nudging. One option: when a `contains:` match lies inside a single Go declaration, `remember` could suggest, or add, a sealed `symbol:` anchor for it. The `contains:` anchor would then have to move with that declaration, or it would still stale the memory on its own.

**B15, done** on branch `feat/contains-follows-symbol`.
- **Automatic symbol anchor:** when a `contains:` match lies inside a single Go declaration, `remember` adds a sealed `symbol:` anchor for that declaration. A comment-only match gets none, since it is a weak anchor already. A note tells the agent.
- **The contains anchor follows:** when that declaration moves unchanged, the `contains:` anchor moves with it, but only if its pattern still matches inside the moved declaration's lines. The pattern appearing elsewhere in the new file does not count. `get` and `--reseal` treat both anchors the same way.
- **On a Hugo clone:**
  - **The memory:** an agent-style memory with only `contains:tpl/math/math.go::af, err := cast\.ToFloat64E\(n\)`.
  - **When saved:** it gained `symbol:tpl/math/math.go::Namespace.Abs@a5964fbf…`.
  - **After `git mv` to `arith.go`:** it comes back as `[moved: tpl/math/math.go → tpl/math/arith.go] … (tpl/math/arith.go:48, tpl/math/arith.go:47-54)`.

**B12, items 1 and 2 done** on branch `fix/launch-notes-b12`.
- **Build stamp:** the guide now opens with "Build 2026-10-22 19:16 UTC", to the minute and different on every deploy. An agent can tell its copy is cached and stale by comparing it with what curl returns.
- **Dry-run heading:** without a prompt the map is headed "Code map of the project" instead of "Code map for this prompt".

## 2026-10-23: the Claude Code plugin, first live run (main fcb4c9d)

**B16 · good, with two fixes · the plugin in a real session.** Fırat installed it from the local marketplace in a small Go project of his: `/plugin marketplace add C:\source\aletheia`, then `/plugin install aletheia@aletheia`, then `/aletheia:setup`.
- **Worked:** the install, and the setup skill. The agent checked the binary, ran `init --agents` and skipped `--claude-hooks` because the plugin brings the hooks. It found no older hook entries.
- **Fix 1 · init still suggested `--claude-hooks`:** the agent had to tell the user to ignore that line. Now `init` reads `enabledPlugins` from Claude Code's settings, local then project then user. With the plugin on, it says the hooks are already there, and it warns when `--claude-hooks` would add them a second time.
- **Fix 2 · the dry run came back empty without saying why:** the project has 6 source files, under the 25 that a code map starts at, and no memories yet. That result is correct, but the agent called it "worth checking". Now an empty dry run lists the reasons. `init` also no longer promises a code map in a project too small for one.
- **Not Aletheia's:** the Serena MCP server failed to connect in that session.

**B17 · fixed · a map labelled with the wrong prompt.** The turn that fixed B16 started from a message with only a screenshot. The Stop hook saw no text in it, took it for a machine-written message such as a tool result, and so did not start a new task. The files changed in that turn were captured under the previous prompt ("Push docs/name-decision…").
- **The fix:** a prompt with an image now starts a new task. With no words there is nothing to name the task by, so its files are not captured. An image with text is labelled by the text.
- **Test:** `TestStopHookStartsANewTaskOnAnImageOnlyPrompt`, which fails on the old code.
