import { cpSync, existsSync, mkdirSync, renameSync, rmSync } from "node:fs";
import { basename, dirname, join, relative, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(scriptDir, "..");
const releasesDir = join(projectRoot, "releases");
const runId = process.env.RELEASE_RUN_ID ?? buildRunId();
const stagingDir = join(releasesDir, `.release-staging-${runId}`);
const archivePath = join(releasesDir, "release.zip");
const archiveTmpPath = join(releasesDir, `.release-${runId}.zip`);

const manifest = [
  { path: "docker-compose.yml" },
  { path: ".env.production.example" },
  { path: "README.md" },
  { path: "docs/operations/runbooks.md" },
  { path: "apps/api/.dockerignore" },
  { path: "apps/api/Dockerfile" },
  { path: "apps/api/Dockerfile.worker" },
  { path: "apps/api/alerts.yml" },
  { path: "apps/api/go.mod" },
  { path: "apps/api/go.sum" },
  { path: "apps/api/requirements.txt" },
  { path: "apps/api/cmd" },
  { path: "apps/api/config", exclude: [isGoTestFile] },
  { path: "apps/api/internal", exclude: [isGoTestFile, isTestDataPath] },
  { path: "apps/api/migrations" },
  { path: "apps/api/scripts/backup-db.sh" },
  { path: "apps/api/scripts/docker-entrypoint.sh" },
  { path: "apps/web/.dockerignore" },
  { path: "apps/web/Dockerfile" },
  { path: "apps/web/package.json" },
  { path: "apps/web/package-lock.json" },
  { path: "apps/web/next.config.ts" },
  { path: "apps/web/next-env.d.ts" },
  { path: "apps/web/instrumentation.ts" },
  { path: "apps/web/middleware.ts" },
  { path: "apps/web/postcss.config.mjs" },
  { path: "apps/web/tsconfig.json" },
  { path: "apps/web/sentry.client.config.ts" },
  { path: "apps/web/sentry.edge.config.ts" },
  { path: "apps/web/sentry.server.config.ts" },
  { path: "apps/web/messages" },
  { path: "apps/web/public" },
  { path: "apps/web/src", exclude: [isFrontendTestFile, isFrontendTestPath] },
];

prepareReleaseDirectory();
copyManifest();
createArchive(archiveTmpPath);
const finalArchivePath = finalizeArchive();
rmSync(stagingDir, { recursive: true, force: true });

console.log(`Created ${finalArchivePath}`);

function prepareReleaseDirectory() {
  mkdirSync(releasesDir, { recursive: true });

  // Use a unique staging directory per run so we don't depend on being
  // able to delete leftover files from a prior (possibly sudo/root) run.
  rmSync(stagingDir, { recursive: true, force: true });
  mkdirSync(stagingDir, { recursive: true });

  // Ensure we start with a clean temp archive path.
  rmSync(archiveTmpPath, { force: true });
}

function copyManifest() {
  for (const entry of manifest) {
    const sourcePath = join(projectRoot, entry.path);
    const destinationPath = join(stagingDir, entry.path);

    if (!existsSync(sourcePath)) {
      throw new Error(`Missing required release file: ${entry.path}`);
    }

    mkdirSync(dirname(destinationPath), { recursive: true });
    cpSync(sourcePath, destinationPath, {
      recursive: true,
      filter: buildFilter(sourcePath, entry.exclude ?? []),
    });
  }
}

function buildFilter(sourceRoot, excludeChecks) {
  return (sourcePath) => {
    const name = basename(sourcePath);
    if (name === ".DS_Store") {
      return false;
    }

    const relativePath = relative(sourceRoot, sourcePath).replaceAll("\\", "/");
    if (relativePath === "") {
      return true;
    }

    return !excludeChecks.some((check) => check(relativePath));
  };
}

function isGoTestFile(relativePath) {
  return relativePath.endsWith("_test.go");
}

function isTestDataPath(relativePath) {
  return relativePath === "testdata" || relativePath.startsWith("testdata/");
}

function isFrontendTestFile(relativePath) {
  return /\.test\.(ts|tsx)$/.test(relativePath);
}

function isFrontendTestPath(relativePath) {
  return relativePath === "test" || relativePath.startsWith("test/");
}

function createArchive(outputPath) {
  const zipfileResult = spawnSync("python3", ["-m", "zipfile", "-c", outputPath, "."], {
    cwd: stagingDir,
    stdio: "inherit",
  });

  if (!zipfileResult.error && zipfileResult.status === 0) {
    return;
  }

  if (zipfileResult.error && zipfileResult.error.code !== "ENOENT") {
    throw zipfileResult.error;
  }

  const zipResult = spawnSync("zip", ["-rq", outputPath, "."], {
    cwd: stagingDir,
    stdio: "inherit",
  });

  if (!zipResult.error && zipResult.status === 0) {
    return;
  }

  if (zipResult.error && zipResult.error.code !== "ENOENT") {
    throw zipResult.error;
  }

  throw new Error("Unable to create releases/release.zip. Install python3 or zip on the machine running npm run release.");
}

function finalizeArchive() {
  try {
    rmSync(archivePath, { force: true });
  } catch (error) {
    if (error && (error.code === "EACCES" || error.code === "EPERM")) {
      console.warn(
        `WARN: Unable to remove existing ${archivePath} (${error.code}); leaving new archive at ${archiveTmpPath}`,
      );
      console.warn(`Fix: sudo chown -R \"$USER\":\"$USER\" ${releasesDir}  (or remove releases/release.zip manually)`);
      return archiveTmpPath;
    }
    throw error;
  }

  try {
    renameSync(archiveTmpPath, archivePath);
    return archivePath;
  } catch (error) {
    if (error && (error.code === "EACCES" || error.code === "EPERM")) {
      console.warn(
        `WARN: Unable to move archive into place at ${archivePath} (${error.code}); leaving new archive at ${archiveTmpPath}`,
      );
      console.warn(`Fix: sudo chown -R \"$USER\":\"$USER\" ${releasesDir}  (or move the file manually)`);
      return archiveTmpPath;
    }
    throw error;
  }
}

function buildRunId() {
  // Compact, filesystem-safe, sortable run id.
  // Example: 20260501-154233-1234
  const now = new Date();
  const pad2 = (n) => String(n).padStart(2, "0");
  const pad3 = (n) => String(n).padStart(3, "0");
  const y = now.getFullYear();
  const m = pad2(now.getMonth() + 1);
  const d = pad2(now.getDate());
  const hh = pad2(now.getHours());
  const mm = pad2(now.getMinutes());
  const ss = pad2(now.getSeconds());
  const ms = pad3(now.getMilliseconds());
  return `${y}${m}${d}-${hh}${mm}${ss}-${ms}`;
}