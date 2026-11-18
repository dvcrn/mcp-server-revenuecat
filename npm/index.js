#!/usr/bin/env node
const fs = require("fs");
const path = require("path");
const { spawn } = require("child_process");

const { installBinary } = require("./postinstall");
const { startUpdateCheck } = require("./update_check");

const exe =
  process.platform === "win32"
    ? "mcp-server-revenuecat.exe"
    : "mcp-server-revenuecat";
const binPath = path.join(__dirname, exe);

const PKG = (() => {
  try {
    return require("./package.json");
  } catch {
    return null;
  }
})();

async function main() {
  if (!fs.existsSync(binPath)) {
    await installBinary();
  }

  if (!fs.existsSync(binPath)) {
    console.error(`Binary not found: ${binPath}`);
    process.exit(1);
  }

  if (process.platform !== "win32") {
    try {
      if ((fs.statSync(binPath).mode & 0o111) === 0) {
        fs.chmodSync(binPath, 0o755);
      }
    } catch {}
  }

  const child = spawn(binPath, process.argv.slice(2), {
    stdio: "inherit",
    windowsHide: true,
  });

  const signalListeners = new Map();
  for (const sig of ["SIGINT", "SIGTERM"]) {
    const listener = () => {
      if (child.exitCode === null && child.signalCode === null) {
        try {
          child.kill(sig);
        } catch {}
      }
    };
    signalListeners.set(sig, listener);
    process.on(sig, listener);
  }

  // Abort on child exit so an in-flight check cannot hold the event loop open.
  const updateAbort = new AbortController();
  startUpdateCheck({
    installedVersion: PKG && PKG.version,
    signal: updateAbort.signal,
  });

  child.on("exit", (code, signal) => {
    for (const [sig, listener] of signalListeners) {
      process.off(sig, listener);
    }
    try {
      updateAbort.abort();
    } catch {}
    if (signal) {
      process.exitCode =
        signal === "SIGINT" ? 130 : signal === "SIGTERM" ? 143 : 1;
      return;
    }
    process.exitCode = code == null ? 1 : code;
  });
  child.on("error", (err) => {
    for (const [sig, listener] of signalListeners) {
      process.off(sig, listener);
    }
    try {
      updateAbort.abort();
    } catch {}
    console.error(err.message);
    process.exitCode = 1;
  });
}

main().catch((err) => {
  console.error(
    `mcp-server-revenuecat: failed to install binary: ${err.message}`,
  );
  process.exit(1);
});
