const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const { spawn, spawnSync } = require("node:child_process");

const { installBinary } = require("./postinstall");

function sha256Hex(buf) {
  return crypto.createHash("sha256").update(buf).digest("hex");
}

function buildArchive(tmpDir, exeName, content) {
  const srcPath = path.join(tmpDir, exeName);
  fs.writeFileSync(srcPath, content, { mode: 0o755 });
  const archiveName = "fixture.tar.gz";
  const res = spawnSync("tar", ["-czf", archiveName, exeName], { cwd: tmpDir });
  assert.equal(res.status, 0, "failed to create fixture tar.gz");
  return fs.readFileSync(path.join(tmpDir, archiveName));
}

test("installBinary extracts via relative tar args and renames atomically under concurrency", async () => {
  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "rc-npm-test-"));
  const fixtureDir = path.join(workDir, "fixture");
  const outDir = path.join(workDir, "out");
  fs.mkdirSync(fixtureDir);
  fs.mkdirSync(outDir);

  const prevVersion = process.env.MCP_SERVER_REVENUECAT_ORG_VERSION;
  const prevPlatform = process.env.MCP_SERVER_REVENUECAT_ORG_PLATFORM;
  const prevArch = process.env.MCP_SERVER_REVENUECAT_ORG_ARCH;
  const prevBase = process.env.MCP_SERVER_REVENUECAT_ORG_BASE_URL;

  try {
    process.env.MCP_SERVER_REVENUECAT_ORG_VERSION = "0.1.0";
    process.env.MCP_SERVER_REVENUECAT_ORG_PLATFORM = "linux";
    process.env.MCP_SERVER_REVENUECAT_ORG_ARCH = "amd64";
    process.env.MCP_SERVER_REVENUECAT_ORG_BASE_URL = "https://example.invalid/releases";

    const exe = "mcp-server-revenuecat-org";
    const assetName = `${exe}_0.1.0_linux_amd64.tar.gz`;
    const expectedPayload = "#!/bin/sh\necho ok\n";
    const archiveBuf = buildArchive(fixtureDir, exe, expectedPayload);
    const checksumLine = `${sha256Hex(archiveBuf)}  ${assetName}\n`;

    const fetchBuffer = async (url) => {
      if (url.endsWith(`/${assetName}`)) return archiveBuf;
      if (url.endsWith("/checksums.txt")) return Buffer.from(checksumLine, "utf8");
      throw new Error(`unexpected URL ${url}`);
    };

    await Promise.all([
      installBinary({ outDir, fetchBuffer }),
      installBinary({ outDir, fetchBuffer }),
    ]);

    const binPath = path.join(outDir, exe);
    assert.equal(fs.readFileSync(binPath, "utf8"), expectedPayload);
    assert.notEqual(fs.statSync(binPath).mode & 0o111, 0);

    const remaining = fs.readdirSync(outDir);
    assert.deepEqual(remaining, [exe]);
  } finally {
    if (prevVersion === undefined) delete process.env.MCP_SERVER_REVENUECAT_ORG_VERSION;
    else process.env.MCP_SERVER_REVENUECAT_ORG_VERSION = prevVersion;
    if (prevPlatform === undefined) delete process.env.MCP_SERVER_REVENUECAT_ORG_PLATFORM;
    else process.env.MCP_SERVER_REVENUECAT_ORG_PLATFORM = prevPlatform;
    if (prevArch === undefined) delete process.env.MCP_SERVER_REVENUECAT_ORG_ARCH;
    else process.env.MCP_SERVER_REVENUECAT_ORG_ARCH = prevArch;
    if (prevBase === undefined) delete process.env.MCP_SERVER_REVENUECAT_ORG_BASE_URL;
    else process.env.MCP_SERVER_REVENUECAT_ORG_BASE_URL = prevBase;
    fs.rmSync(workDir, { recursive: true, force: true });
  }
});

test("index.js forwards SIGTERM to the spawned binary so the child is not orphaned", async (t) => {
  if (process.platform === "win32") {
    t.skip("POSIX signal forwarding check");
    return;
  }

  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "rc-npm-sig-"));
  try {
    for (const file of ["index.js", "postinstall.js", "update_check.js", "package.json"]) {
      fs.copyFileSync(path.join(__dirname, file), path.join(workDir, file));
    }

    const markerPath = path.join(workDir, "child.pid");
    const termPath = path.join(workDir, "child.term");
    const fakeBin = path.join(workDir, "mcp-server-revenuecat-org");
    fs.writeFileSync(
      fakeBin,
      [
        "#!/usr/bin/env node",
        'const fs = require("fs");',
        `fs.writeFileSync(${JSON.stringify(markerPath)}, String(process.pid));`,
        'process.on("SIGTERM", () => {',
        `  fs.writeFileSync(${JSON.stringify(termPath)}, "SIGTERM");`,
        "  process.exit(0);",
        "});",
        "setInterval(() => {}, 1000);",
      ].join("\n"),
      { mode: 0o755 },
    );

    const wrapper = spawn(process.execPath, [path.join(workDir, "index.js")], {
      env: { ...process.env, NO_UPDATE_NOTIFIER: "1" },
      stdio: ["pipe", "pipe", "pipe"],
    });

    const deadline = Date.now() + 5000;
    while (!fs.existsSync(markerPath)) {
      if (Date.now() > deadline) {
        wrapper.kill("SIGKILL");
        assert.fail("timed out waiting for fake binary to start");
      }
      await new Promise((r) => setTimeout(r, 25));
    }

    const childPid = Number(fs.readFileSync(markerPath, "utf8").trim());
    assert.ok(childPid > 0);

    const exitPromise = new Promise((resolve) => {
      wrapper.on("exit", (code, signal) => resolve({ code, signal }));
    });

    wrapper.kill("SIGTERM");
    const result = await exitPromise;
    assert.equal(result.code, 0);
    assert.equal(fs.readFileSync(termPath, "utf8"), "SIGTERM");

    let childAlive = true;
    try {
      process.kill(childPid, 0);
    } catch {
      childAlive = false;
    }
    assert.equal(childAlive, false);
  } finally {
    fs.rmSync(workDir, { recursive: true, force: true });
  }
});
