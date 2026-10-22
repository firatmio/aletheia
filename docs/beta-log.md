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
- **Follow-up:** add an upgrade note for Windows to the README and `/agents.md`: stop the agent first, or rename the old binary. The MCP server keeps running the old version until the agent restarts.

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

Hooks were added during this session. Claude Code loads hooks at session start, so prompt and Stop capture are expected to take effect from the next session. Check that in the first entry of the next session.
