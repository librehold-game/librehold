// Runs the Go server, then the account site (:5173) and the game (:5174) on Vite, with labelled output; Ctrl+C or one
// of them exiting stops all three. Run `cd server && make db` first.
import { spawn, spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { createInterface } from "node:readline";
import { fileURLToPath } from "node:url";
import { styleText } from "node:util";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const serverDir = join(root, "server");
// A built binary, not go run: SIGTERM to go run leaves the server running.
const binary = join(
  serverDir,
  "bin",
  process.platform === "win32" ? "server.exe" : "server",
);
const vite = join(root, "node_modules", "vite", "bin", "vite.js");
// The README's settings; LH_* variables already in the environment win.
const serverEnv = {
  LH_DATABASE_URL:
    "postgres://librehold:librehold@localhost:5433/librehold?sslmode=disable",
  LH_COOKIE_SECURE: "false",
  LH_ADMIN_USERNAME: "admin",
  LH_ADMIN_PASSWORD: "adminadmin",
  ...process.env,
};
const COLORS = {
  dev: "yellow",
  server: "cyan",
  account: "magenta",
  game: "green",
};

const children = new Map();
let ready = false;
let stopping = false;

const say = (name, line) =>
  console.log(`${styleText(COLORS[name], name.padEnd(7))} | ${line}`);

function start(name, command, args, options) {
  const child = spawn(command, args, {
    stdio: ["ignore", "pipe", "pipe"],
    ...options,
  });
  children.set(name, child);
  for (const stream of [child.stdout, child.stderr]) {
    createInterface({ input: stream }).on("line", (line) => {
      if (line.trim()) say(name, line);
    });
  }
  child.on("error", (err) => say(name, err.message));
  child.on("close", (code, signal) => {
    children.delete(name);
    if (stopping) return;
    say(name, `exited with ${signal ?? `code ${code}`}`);
    if (!ready)
      say("dev", "if the database is not running: cd server && make db");
    process.exitCode = 1;
    stop();
  });
}

function stop() {
  if (stopping) return;
  stopping = true;
  for (const child of children.values()) child.kill("SIGTERM");
  setTimeout(() => {
    for (const child of children.values()) child.kill("SIGKILL");
  }, 5000).unref();
}

say("dev", "building the server");
const build = spawnSync("go", ["build", "-o", binary, "./cmd/server"], {
  cwd: serverDir,
  stdio: "inherit",
});
if (build.status !== 0) {
  say(
    "dev",
    build.error
      ? `could not run go: ${build.error.message}`
      : "the server did not build",
  );
  process.exit(1);
}

for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) process.on(signal, stop);
start("server", binary, [], { cwd: serverDir, env: serverEnv });
while (!ready && !stopping) {
  const answer = fetch("http://localhost:8080/api/v1/me", {
    signal: AbortSignal.timeout(2000),
  });
  ready = await answer.then(
    () => true,
    () => false,
  );
  if (!ready) await new Promise((resolve) => setTimeout(resolve, 200));
}
if (!stopping) {
  say(
    "dev",
    "account site http://localhost:5173/, game http://localhost:5174/game/game.html?world=<id>",
  );
  start(
    "account",
    process.execPath,
    [
      vite,
      "--config",
      "vite.account.config.ts",
      "--port",
      "5173",
      "--strictPort",
    ],
    { cwd: root },
  );
  start("game", process.execPath, [vite, "--port", "5174", "--strictPort"], {
    cwd: root,
  });
}
