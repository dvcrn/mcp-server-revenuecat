#!/usr/bin/env node
const fs = require("fs");
const path = require("path");
const os = require("os");
const https = require("https");
const crypto = require("crypto");
const { spawnSync } = require("child_process");

const OWNER = "dvcrn";
const REPO = "mcp-server-revenuecat-org";
const BIN = "mcp-server-revenuecat-org";
const VERSION_ENV = "MCP_SERVER_REVENUECAT_ORG_VERSION";
const BASE_URL_ENV = "MCP_SERVER_REVENUECAT_ORG_BASE_URL";
const ARCH_ENV = "MCP_SERVER_REVENUECAT_ORG_ARCH";
const PLATFORM_ENV = "MCP_SERVER_REVENUECAT_ORG_PLATFORM";
const REQUEST_TIMEOUT_MS = 15000;
const MAX_REDIRECTS = 5;

function httpGet(url, { headers } = {}, redirectsLeft = MAX_REDIRECTS) {
  return new Promise((resolve, reject) => {
    const req = https.get(url, { headers }, (res) => {
      if (
        res.statusCode &&
        res.statusCode >= 300 &&
        res.statusCode < 400 &&
        res.headers.location
      ) {
        res.resume();
        if (redirectsLeft <= 0) {
          reject(new Error(`GET ${url} -> too many redirects`));
          return;
        }
        const nextUrl = new URL(res.headers.location, url).toString();
        resolve(httpGet(nextUrl, { headers }, redirectsLeft - 1));
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`GET ${url} -> ${res.statusCode}`));
        return;
      }
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    });
    req.on("error", reject);
    req.setTimeout(REQUEST_TIMEOUT_MS, () => {
      req.destroy(new Error(`GET ${url} -> timeout`));
    });
  });
}

function sha256(buf) {
  const h = crypto.createHash("sha256");
  h.update(buf);
  return h.digest("hex");
}

function parseChecksums(text) {
  const map = new Map();
  const lines = text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);
  for (const line of lines) {
    let m = line.match(/^([a-f0-9]{64})\s+(.+)$/i);
    if (m) {
      map.set(m[2], m[1]);
      continue;
    }
    m = line.match(/^sha256:([a-f0-9]{64})\s+(.+)$/i);
    if (m) {
      map.set(m[2], m[1]);
      continue;
    }
    m = line.match(/^SHA256\s+\((.+)\)\s+=\s+([a-f0-9]{64})$/i);
    if (m) {
      map.set(m[1], m[2]);
      continue;
    }
  }
  return map;
}

async function installBinary({ outDir = __dirname, fetchBuffer = httpGet } = {}) {
  const platformRaw = process.env[PLATFORM_ENV] || process.platform;
  const platform = platformRaw === "win32" ? "windows" : platformRaw;
  if (!["darwin", "linux", "windows"].includes(platform)) {
    throw new Error(
      "mcp-server-revenuecat-org: npm install supports macOS (darwin), Linux, and Windows only",
    );
  }

  const pkg = JSON.parse(
    fs.readFileSync(path.join(__dirname, "package.json"), "utf8"),
  );
  const version = process.env[VERSION_ENV] || pkg.version || "";
  if (!version) {
    throw new Error("postinstall: could not determine version");
  }

  const detectedArch =
    process.arch === "x64"
      ? "amd64"
      : process.arch === "arm64"
        ? "arm64"
        : process.arch === "arm"
          ? "armv7"
          : process.arch;
  const arch = process.env[ARCH_ENV] || detectedArch;
  if (!["amd64", "arm64", "armv7"].includes(arch)) {
    throw new Error(`Unsupported arch: ${arch}`);
  }
  if (platform === "windows" && arch === "armv7") {
    throw new Error(`Unsupported Windows arch: ${arch}`);
  }

  const assetName = `${BIN}_${version}_${platform}_${arch}.tar.gz`;
  const baseOverride = process.env[BASE_URL_ENV];
  const bases = baseOverride
    ? [baseOverride]
    : [
        `https://github.com/${OWNER}/${REPO}/releases/download/${version}`,
        `https://github.com/${OWNER}/${REPO}/releases/download/v${version}`,
      ];

  const headers = { "User-Agent": `${REPO}-postinstall` };
  const exe = platform === "windows" ? `${BIN}.exe` : BIN;
  const binPath = path.join(outDir, exe);

  if (fs.existsSync(binPath)) {
    if (platform !== "windows") {
      try {
        if ((fs.statSync(binPath).mode & 0o111) === 0) {
          fs.chmodSync(binPath, 0o755);
        }
      } catch {}
    }
    return;
  }

  let tarGz = null;
  let baseUsed = "";
  let lastErr = null;
  for (const base of bases) {
    const url = `${base}/${assetName}`;
    console.error(`postinstall: downloading ${assetName} from ${url}`);
    try {
      tarGz = await fetchBuffer(url, { headers });
      baseUsed = base;
      break;
    } catch (e) {
      lastErr = e;
    }
  }
  if (!tarGz) throw lastErr || new Error("failed to download binary");

  const checksumsUrl = `${baseUsed}/checksums.txt`;
  const checksumsBuf = await fetchBuffer(checksumsUrl, { headers });
  const checksums = parseChecksums(checksumsBuf.toString("utf8"));
  const sumExpected = checksums.get(assetName);
  if (!sumExpected) {
    throw new Error(`postinstall: ${assetName} missing from checksums.txt`);
  }
  const sumActual = sha256(tarGz);
  if (sumActual.toLowerCase() !== sumExpected.toLowerCase()) {
    throw new Error("postinstall: checksum mismatch");
  }
  console.error("postinstall: checksum OK");

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), `${REPO}-`));
  const stagedPath = path.join(
    outDir,
    `.${exe}.${process.pid}.${crypto.randomBytes(4).toString("hex")}.tmp`,
  );
  try {
    fs.writeFileSync(path.join(tmpDir, assetName), tarGz);
    // Relative arguments from tmpDir keep GNU tar on Windows from reading C: as a remote host,
    // and stdio keeps stdout untouched when index.js installs on first run of a stdio server.
    const tarRes = spawnSync("tar", ["-xzf", assetName, exe], {
      cwd: tmpDir,
      stdio: ["ignore", 2, 2],
    });
    if (tarRes.error || tarRes.status !== 0) {
      throw new Error("postinstall: failed to extract binary");
    }
    fs.copyFileSync(path.join(tmpDir, exe), stagedPath);
    if (platform !== "windows") {
      fs.chmodSync(stagedPath, 0o755);
    }
    try {
      fs.renameSync(stagedPath, binPath);
    } catch (err) {
      if (!(platform === "windows" && fs.existsSync(binPath))) {
        throw err;
      }
    }
  } finally {
    try {
      fs.rmSync(stagedPath, { force: true });
    } catch {}
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true });
    } catch {}
  }
  console.error(`postinstall: installed ${exe} to ${outDir}`);
}

if (require.main === module) {
  installBinary().catch((err) => {
    console.error(`postinstall error: ${err.message}`);
    process.exit(1);
  });
}

module.exports = { installBinary };
