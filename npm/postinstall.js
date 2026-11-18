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
        let nextUrl;
        try {
          nextUrl = new URL(res.headers.location, url).toString();
        } catch (err) {
          reject(err);
          return;
        }
        resolve(httpGet(nextUrl, { headers }, redirectsLeft - 1));
        return;
      }
      if (res.statusCode !== 200) {
        res.resume();
        reject(new Error(`GET ${url} -> ${res.statusCode}`));
        return;
      }
      const chunks = [];
      res.on("error", reject);
      res.on("close", () => {
        if (!res.complete) {
          reject(new Error(`GET ${url} -> truncated response`));
        }
      });
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
  return crypto.createHash("sha256").update(buf).digest("hex");
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

function readPackageVersion() {
  const pkg = JSON.parse(
    fs.readFileSync(path.join(__dirname, "package.json"), "utf8"),
  );
  return pkg.version || "";
}

function resolveTarget({
  platformRaw = process.platform,
  archRaw = process.arch,
  version,
  baseUrl,
} = {}) {
  const platform = platformRaw === "win32" ? "windows" : platformRaw;
  if (!["darwin", "linux", "windows"].includes(platform)) {
    throw new Error(
      "mcp-server-revenuecat-org: npm install supports macOS (darwin), Linux, and Windows only",
    );
  }

  const cleanVersion =
    typeof version === "string" ? version.trim().replace(/^v/, "") : "";
  if (!cleanVersion) {
    throw new Error("postinstall: could not determine version");
  }

  const arch =
    archRaw === "x64"
      ? "amd64"
      : archRaw === "arm64"
        ? "arm64"
        : archRaw === "arm"
          ? "armv7"
          : archRaw;
  if (!["amd64", "arm64", "armv7"].includes(arch)) {
    throw new Error(`Unsupported arch: ${arch}`);
  }
  if (platform !== "linux" && arch === "armv7") {
    throw new Error(`Unsupported ${platform} arch: ${arch}`);
  }

  const exe = platform === "windows" ? `${BIN}.exe` : BIN;
  const assetName = `${BIN}_${cleanVersion}_${platform}_${arch}.tar.gz`;
  const bases = baseUrl
    ? [baseUrl.replace(/\/+$/, "")]
    : [
        `https://github.com/${OWNER}/${REPO}/releases/download/${cleanVersion}`,
        `https://github.com/${OWNER}/${REPO}/releases/download/v${cleanVersion}`,
      ];

  return { platform, arch, exe, assetName, bases };
}

async function downloadVerifiedArchive(fetchBuffer, { assetName, bases, headers }) {
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

  const checksumsBuf = await fetchBuffer(`${baseUsed}/checksums.txt`, { headers });
  const sumExpected = parseChecksums(checksumsBuf.toString("utf8")).get(assetName);
  if (!sumExpected) {
    throw new Error(`postinstall: ${assetName} missing from checksums.txt`);
  }
  if (sha256(tarGz).toLowerCase() !== sumExpected.toLowerCase()) {
    throw new Error("postinstall: checksum mismatch");
  }
  console.error("postinstall: checksum OK");
  return tarGz;
}

function extractArchiveToBin(tarGz, { outDir, assetName, exe, binPath, platform }) {
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
}

async function installBinary({
  outDir = __dirname,
  fetchBuffer = httpGet,
  platformRaw,
  archRaw,
  version,
  baseUrl,
} = {}) {
  const resolvedVersion = version !== undefined ? version : readPackageVersion();
  const { platform, exe, assetName, bases } = resolveTarget({
    platformRaw,
    archRaw,
    version: resolvedVersion,
    baseUrl,
  });

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

  const headers = { "User-Agent": `${REPO}-postinstall` };
  const tarGz = await downloadVerifiedArchive(fetchBuffer, {
    assetName,
    bases,
    headers,
  });
  extractArchiveToBin(tarGz, { outDir, assetName, exe, binPath, platform });
  console.error(`postinstall: installed ${exe} to ${outDir}`);
}

if (require.main === module) {
  installBinary().catch((err) => {
    console.error(`postinstall error: ${err.message}`);
    process.exit(1);
  });
}

module.exports = { installBinary, resolveTarget };
