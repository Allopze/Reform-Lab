import { cpSync, existsSync, mkdirSync, rmSync } from "node:fs";
import { basename, dirname, join, relative, resolve } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(scriptDir, "..");
const releasesDir = join(projectRoot, "releases");
const stagingDir = join(releasesDir, "release");
const archivePath = join(releasesDir, "release.zip");

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
createArchive();
rmSync(stagingDir, { recursive: true, force: true });

console.log(`Created ${archivePath}`);

function prepareReleaseDirectory() {
  mkdirSync(releasesDir, { recursive: true });
  rmSync(stagingDir, { recursive: true, force: true });
  rmSync(archivePath, { force: true });
  mkdirSync(stagingDir, { recursive: true });
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

function createArchive() {
  const zipfileResult = spawnSync("python3", ["-m", "zipfile", "-c", archivePath, "."], {
    cwd: stagingDir,
    stdio: "inherit",
  });

  if (!zipfileResult.error && zipfileResult.status === 0) {
    return;
  }

  if (zipfileResult.error && zipfileResult.error.code !== "ENOENT") {
    throw zipfileResult.error;
  }

  const zipResult = spawnSync("zip", ["-rq", archivePath, "."], {
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