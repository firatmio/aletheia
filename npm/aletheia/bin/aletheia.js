#!/usr/bin/env node
"use strict";

// Runs the aletheia binary for this platform. npm installs exactly one of
// the @firatmio/aletheia-<platform>-<arch> packages (os and cpu fields), so
// nothing is downloaded at install time and no script runs then.

const { spawn } = require("node:child_process");
const fs = require("node:fs");

const target = `${process.platform}-${process.arch}`;
const exe = process.platform === "win32" ? "aletheia.exe" : "aletheia";

let bin;
try {
  bin = require.resolve(`@firatmio/aletheia-${target}/bin/${exe}`);
} catch {
  console.error(
    `aletheia: no prebuilt binary for ${target}.\n` +
      "Supported: linux, darwin and win32 on x64 and arm64.\n" +
      "Elsewhere, install with Go 1.24+: go install github.com/firatmio/aletheia/cmd/aletheia@latest",
  );
  process.exit(1);
}

// A package packed on Windows can lose the executable bit.
if (process.platform !== "win32") {
  try {
    fs.accessSync(bin, fs.constants.X_OK);
  } catch {
    try {
      fs.chmodSync(bin, 0o755);
    } catch {}
  }
}

// stdio is inherited, so `aletheia serve` speaks MCP over this process's
// stdin and stdout, and hooks get their JSON through untouched.
const child = spawn(bin, process.argv.slice(2), { stdio: "inherit", windowsHide: true });
for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(signal, () => child.kill(signal));
}
child.on("error", (err) => {
  console.error(`aletheia: ${err.message}`);
  process.exit(1);
});
child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  } else {
    process.exit(code ?? 1);
  }
});
