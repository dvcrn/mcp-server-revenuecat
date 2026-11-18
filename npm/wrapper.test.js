const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const { spawn, spawnSync } = require("node:child_process");

const { installBinary, resolveTarget } = require("./postinstall");
const { parseVersion, isNewerVersion } = require("./update_check");

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

test("resolveTarget maps platforms, architectures, and release URLs purely", () => {
  const linux = resolveTarget({
    platformRaw: "linux",
    archRaw: "x64",
    version: "v0.1.0",
  });
  assert.deepEqual(linux, {
    platform: "linux",
    arch: "amd64",
    exe: "mcp-server-revenuecat-org",
    assetName: "mcp-server-revenuecat-org_0.1.0_linux_amd64.tar.gz",
    bases: [
      "https://github.com/dvcrn/mcp-server-revenuecat-org/releases/download/0.1.0",
      "https://github.com/dvcrn/mcp-server-revenuecat-org/releases/download/v0.1.0",
    ],
  });

  const win = resolveTarget({
    platformRaw: "win32",
    archRaw: "arm64",
    version: "1.2.3",
    baseUrl: "https://example.invalid/releases/",
  });
  assert.equal(win.platform, "windows");
  assert.equal(win.exe, "mcp-server-revenuecat-org.exe");
  assert.equal(win.assetName, "mcp-server-revenuecat-org_1.2.3_windows_arm64.tar.gz");
  assert.deepEqual(win.bases, ["https://example.invalid/releases"]);

  assert.throws(
    () => resolveTarget({ platformRaw: "freebsd", archRaw: "x64", version: "0.1.0" }),
    /supports macOS/,
  );
  assert.throws(
    () => resolveTarget({ platformRaw: "darwin", archRaw: "arm", version: "0.1.0" }),
    /Unsupported darwin arch: armv7/,
  );
  assert.throws(
    () => resolveTarget({ platformRaw: "win32", archRaw: "arm", version: "0.1.0" }),
    /Unsupported windows arch: armv7/,
  );
  assert.throws(
    () => resolveTarget({ platformRaw: "linux", archRaw: "x64", version: "" }),
    /could not determine version/,
  );
});

test("parseVersion and isNewerVersion compare stable MAJOR.MINOR.PATCH tuples", () => {
  assert.deepEqual(parseVersion("v1.2.3"), {
    version: "1.2.3",
    tuple: [1, 2, 3],
  });
  assert.equal(parseVersion("01.2.3"), null);
  assert.equal(parseVersion("1.2.3-beta.1"), null);
  assert.equal(parseVersion(""), null);

  assert.equal(isNewerVersion("0.2.0", "0.1.9"), true);
  assert.equal(isNewerVersion("1.0.0", "0.99.99"), true);
  assert.equal(isNewerVersion("0.1.10", "0.1.2"), true);
  assert.equal(isNewerVersion("0.1.0", "0.1.0"), false);
  assert.equal(isNewerVersion("0.1.0", "0.2.0"), false);
  assert.equal(isNewerVersion("invalid", "0.1.0"), false);
});

test("installBinary extracts via relative tar args and renames atomically under concurrency", async () => {
  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "rc-npm-test-"));
  const fixtureDir = path.join(workDir, "fixture");
  const outDir = path.join(workDir, "out");
  fs.mkdirSync(fixtureDir);
  fs.mkdirSync(outDir);

  try {
    const exe = "mcp-server-revenuecat-org";
    const assetName = `${exe}_0.1.0_linux_amd64.tar.gz`;
    const expectedPayload = "#!/bin/sh\necho ok\n";
    const archiveBuf = buildArchive(fixtureDir, exe, expectedPayload);
    const checksumLine = `${sha256Hex(archiveBuf)}  ${assetName}\n`;

    const opts = {
      outDir,
      version: "0.1.0",
      platformRaw: "linux",
      archRaw: "x64",
      baseUrl: "https://example.invalid/releases",
      fetchBuffer: async (url) => {
        if (url.endsWith(`/${assetName}`)) return archiveBuf;
        if (url.endsWith("/checksums.txt")) return Buffer.from(checksumLine, "utf8");
        throw new Error(`unexpected URL ${url}`);
      },
    };

    await Promise.all([installBinary(opts), installBinary(opts)]);

    const binPath = path.join(outDir, exe);
    assert.equal(fs.readFileSync(binPath, "utf8"), expectedPayload);
    assert.notEqual(fs.statSync(binPath).mode & 0o111, 0);

    const remaining = fs.readdirSync(outDir);
    assert.deepEqual(remaining, [exe]);
  } finally {
    fs.rmSync(workDir, { recursive: true, force: true });
  }
});

test("installBinary rejects mismatched or missing checksums without writing binPath", async () => {
  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "rc-npm-sum-"));
  const fixtureDir = path.join(workDir, "fixture");
  const outDir = path.join(workDir, "out");
  fs.mkdirSync(fixtureDir);
  fs.mkdirSync(outDir);

  try {
    const exe = "mcp-server-revenuecat-org";
    const assetName = `${exe}_0.1.0_linux_amd64.tar.gz`;
    const archiveBuf = buildArchive(fixtureDir, exe, "#!/bin/sh\necho tampered\n");
    const wrongSum = "0".repeat(64);
    const baseOpts = {
      outDir,
      version: "0.1.0",
      platformRaw: "linux",
      archRaw: "x64",
      baseUrl: "https://example.invalid/releases",
    };

    await assert.rejects(
      () =>
        installBinary({
          ...baseOpts,
          fetchBuffer: async (url) => {
            if (url.endsWith(`/${assetName}`)) return archiveBuf;
            if (url.endsWith("/checksums.txt")) {
              return Buffer.from(`${wrongSum}  ${assetName}\n`, "utf8");
            }
            throw new Error(`unexpected URL ${url}`);
          },
        }),
      /checksum mismatch/,
    );
    assert.deepEqual(fs.readdirSync(outDir), []);

    await assert.rejects(
      () =>
        installBinary({
          ...baseOpts,
          fetchBuffer: async (url) => {
            if (url.endsWith(`/${assetName}`)) return archiveBuf;
            if (url.endsWith("/checksums.txt")) {
              return Buffer.from(`${wrongSum}  other_asset.tar.gz\n`, "utf8");
            }
            throw new Error(`unexpected URL ${url}`);
          },
        }),
      /missing from checksums\.txt/,
    );
    assert.deepEqual(fs.readdirSync(outDir), []);
  } finally {
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
