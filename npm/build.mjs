// Builds the npm packages from a goreleaser run: one package per platform
// with its binary, and @firatmio/aletheia, which depends on all of them as
// optional dependencies so npm installs only the one that fits.
//
//   node npm/build.mjs [--version 0.1.0] [--dist dist] [--out dist/npm]
//
// The version defaults to goreleaser's (dist/metadata.json). Build and
// publish on Linux or macOS: a package packed on Windows loses the binaries'
// executable bit. From Windows, in Docker, platform packages first:
//
//   docker run --rm -v "$PWD:/src" -w /src -e NPM_TOKEN node:22-slim sh -c \
//     'echo "//registry.npmjs.org/:_authToken=$NPM_TOKEN" > ~/.npmrc && node npm/build.mjs &&
//      for d in dist/npm/aletheia-*/ dist/npm/aletheia/; do npm publish --access public "$d"; done'

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.dirname(here);

const args = process.argv.slice(2);
const opt = (name, fallback) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback;
};
const dist = path.resolve(root, opt("dist", "dist"));
const out = path.resolve(root, opt("out", path.join("dist", "npm")));
const built = JSON.parse(fs.readFileSync(path.join(dist, "metadata.json"), "utf8")).version;
const version = opt("version", built);
if (version !== built) {
  console.warn(`warning: packaging ${version}, but the binaries are goreleaser's ${built}`);
}

// Go's names for platforms, and Node's.
const nodeOS = { linux: "linux", darwin: "darwin", windows: "win32" };
const nodeCPU = { amd64: "x64", arm64: "arm64" };

const main = JSON.parse(fs.readFileSync(path.join(here, "aletheia", "package.json"), "utf8"));
const shared = { homepage: main.homepage, repository: main.repository, license: main.license, author: main.author };

fs.rmSync(out, { recursive: true, force: true });
const artifacts = JSON.parse(fs.readFileSync(path.join(dist, "artifacts.json"), "utf8"));
const optional = {};
for (const a of artifacts) {
  if (a.type !== "Binary" || !nodeOS[a.goos] || !nodeCPU[a.goarch]) {
    continue;
  }
  const os = nodeOS[a.goos];
  const cpu = nodeCPU[a.goarch];
  const name = `@firatmio/aletheia-${os}-${cpu}`;
  const dir = path.join(out, `aletheia-${os}-${cpu}`);
  const exe = path.join(dir, "bin", path.basename(a.path));
  fs.mkdirSync(path.dirname(exe), { recursive: true });
  fs.copyFileSync(path.resolve(root, a.path), exe);
  fs.chmodSync(exe, 0o755);
  fs.copyFileSync(path.join(root, "LICENSE"), path.join(dir, "LICENSE"));
  const pkg = {
    name,
    version,
    description: `The aletheia binary for ${os} ${cpu}. Install @firatmio/aletheia instead.`,
    ...shared,
    os: [os],
    cpu: [cpu],
    files: ["bin", "LICENSE"],
    preferUnplugged: true,
  };
  fs.writeFileSync(path.join(dir, "package.json"), JSON.stringify(pkg, null, 2) + "\n");
  optional[name] = version;
  console.log(`${name}@${version}`);
}
if (Object.keys(optional).length === 0) {
  console.error("no binaries in artifacts.json: run goreleaser first");
  process.exit(1);
}

const dir = path.join(out, "aletheia");
fs.cpSync(path.join(here, "aletheia"), dir, { recursive: true });
fs.chmodSync(path.join(dir, "bin", "aletheia.js"), 0o755);
fs.copyFileSync(path.join(root, "LICENSE"), path.join(dir, "LICENSE"));
main.version = version;
main.optionalDependencies = Object.fromEntries(Object.entries(optional).sort());
fs.writeFileSync(path.join(dir, "package.json"), JSON.stringify(main, null, 2) + "\n");
console.log(`${main.name}@${version}`);
