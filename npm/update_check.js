const fs = require("fs");
const path = require("path");
const os = require("os");
const https = require("https");

const DEFAULT_PACKAGE_NAME = "mcp-server-revenuecat-org";
const DEFAULT_UPDATE_COMMAND = "npm install -g mcp-server-revenuecat-org";
const CACHE_DIR_NAME = "mcp-server-revenuecat-org";

const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;
const UPDATE_CHECK_TIMEOUT_MS = 1200;

function isDisabled() {
  return (
    process.env.MCP_SERVER_REVENUECAT_ORG_NO_UPDATE_NOTICE === "1" ||
    process.env.MCP_SERVER_REVENUECAT_ORG_NO_UPDATE_NOTICE === "true" ||
    process.env.NO_UPDATE_NOTIFIER === "1" ||
    process.env.NO_UPDATE_NOTIFIER === "true" ||
    process.env.CI === "1" ||
    process.env.CI === "true" ||
    process.env.CI === "yes"
  );
}

function getCacheFile() {
  const home = os.homedir();
  if (!home) return null;

  if (process.platform === "win32") {
    const base = process.env.LOCALAPPDATA || process.env.APPDATA;
    if (!base)
      return path.join(home, "AppData", "Local", CACHE_DIR_NAME, "update.json");
    return path.join(base, CACHE_DIR_NAME, "update.json");
  }

  if (process.platform === "darwin") {
    return path.join(home, "Library", "Caches", CACHE_DIR_NAME, "update.json");
  }

  const base = process.env.XDG_CACHE_HOME || path.join(home, ".cache");
  return path.join(base, CACHE_DIR_NAME, "update.json");
}

function readCache() {
  const file = getCacheFile();
  if (!file) return null;
  try {
    return JSON.parse(fs.readFileSync(file, "utf8"));
  } catch {
    return null;
  }
}

function writeCache(data) {
  const file = getCacheFile();
  if (!file) return;
  try {
    fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, JSON.stringify(data), "utf8");
  } catch {}
}

function fetchLatestVersion(packageName, timeoutMs, signal) {
  const url = `https://registry.npmjs.org/${encodeURIComponent(packageName)}/latest`;

  return new Promise((resolve, reject) => {
    const req = https.get(
      url,
      { headers: { "User-Agent": `${CACHE_DIR_NAME}-update-check` } },
      (res) => {
        if (res.statusCode !== 200) {
          reject(new Error(`GET ${url} -> ${res.statusCode}`));
          res.resume();
          return;
        }

        const chunks = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () => {
          try {
            const json = JSON.parse(Buffer.concat(chunks).toString("utf8"));
            resolve(typeof json.version === "string" ? json.version : "");
          } catch (e) {
            reject(e);
          }
        });
      },
    );

    req.on("error", reject);
    req.setTimeout(timeoutMs, () => req.destroy(new Error("timeout")));

    if (signal) {
      if (signal.aborted) {
        req.destroy(new Error("aborted"));
        return;
      }
      signal.addEventListener("abort", () => req.destroy(new Error("aborted")), {
        once: true,
      });
    }
  });
}

function parseVersion(raw) {
  if (typeof raw !== "string") return null;
  const m = raw
    .trim()
    .match(/^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/);
  if (!m) return null;
  const tuple = [Number(m[1]), Number(m[2]), Number(m[3])];
  if (!tuple.every(Number.isSafeInteger)) return null;
  return { version: `${tuple[0]}.${tuple[1]}.${tuple[2]}`, tuple };
}

function isNewerVersion(latestRaw, installedRaw) {
  const latest = parseVersion(latestRaw);
  const installed = parseVersion(installedRaw);
  if (!latest || !installed) return false;
  for (let i = 0; i < 3; i++) {
    if (latest.tuple[i] !== installed.tuple[i]) {
      return latest.tuple[i] > installed.tuple[i];
    }
  }
  return false;
}

function printNotice({ packageName, installed, latest, updateCommand }) {
  const prefix = `${packageName}:`;
  if (installed && latest) {
    console.error(
      `${prefix} an update is available (installed ${installed}, latest ${latest}). Update with: ${updateCommand}`,
    );
    return;
  }
  console.error(`${prefix} an update is available. Update with: ${updateCommand}`);
}

async function runUpdateCheck({
  packageName,
  installedVersion,
  updateCommand,
  signal,
}) {
  if (isDisabled()) return;
  if (!process.stderr.isTTY) return;

  const installedParsed = parseVersion(installedVersion);
  if (!installedParsed) return;
  const installed = installedParsed.version;

  const cache = readCache() || {};
  const lastChecked = Number.isFinite(cache.lastChecked) ? cache.lastChecked : 0;
  const lastNotified = Number.isFinite(cache.lastNotified)
    ? cache.lastNotified
    : 0;
  const cachedLatest = typeof cache.latest === "string" ? cache.latest : "";

  const now = Date.now();

  if (lastChecked && now - lastChecked < UPDATE_CHECK_INTERVAL_MS) {
    const cachedParsed = parseVersion(cachedLatest);
    if (cachedParsed && isNewerVersion(cachedParsed.version, installed)) {
      if (!lastNotified || now - lastNotified >= UPDATE_CHECK_INTERVAL_MS) {
        printNotice({
          packageName,
          installed,
          latest: cachedParsed.version,
          updateCommand,
        });
        writeCache({ ...cache, lastNotified: now });
      }
    }
    return;
  }

  const latestRaw = await fetchLatestVersion(
    packageName,
    UPDATE_CHECK_TIMEOUT_MS,
    signal,
  );
  const latestParsed = parseVersion(latestRaw);
  const latest = latestParsed ? latestParsed.version : "";

  writeCache({ lastChecked: now, lastNotified, latest: latest || latestRaw });

  if (latest && isNewerVersion(latest, installed)) {
    if (!lastNotified || now - lastNotified >= UPDATE_CHECK_INTERVAL_MS) {
      printNotice({ packageName, installed, latest, updateCommand });
      writeCache({ lastChecked: now, lastNotified: now, latest });
    }
  }
}

function startUpdateCheck({
  packageName = DEFAULT_PACKAGE_NAME,
  installedVersion,
  updateCommand = DEFAULT_UPDATE_COMMAND,
  signal,
} = {}) {
  runUpdateCheck({
    packageName,
    installedVersion,
    updateCommand,
    signal,
  }).catch(() => {});
}

module.exports = {
  startUpdateCheck,
  parseVersion,
  isNewerVersion,
};
