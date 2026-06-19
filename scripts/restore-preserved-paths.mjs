import fs from "node:fs";
import path from "node:path";

const [backupRoot] = process.argv.slice(2);

if (!backupRoot) {
  console.error("usage: node scripts/restore-preserved-paths.mjs <backup-dir>");
  process.exit(1);
}

const cwd = process.cwd();
const backupDir = path.resolve(cwd, backupRoot);
const manifestPath = path.join(backupDir, ".manifest.json");

if (!fs.existsSync(manifestPath)) {
  console.error(`preserved path manifest not found: ${manifestPath}`);
  process.exit(1);
}

const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));

for (const entry of manifest) {
  const targetPath = path.resolve(cwd, entry.path);
  const backupPath = path.join(backupDir, entry.path);

  if (!fs.existsSync(backupPath)) {
    continue;
  }

  fs.rmSync(targetPath, { force: true, recursive: true });
  fs.mkdirSync(path.dirname(targetPath), { recursive: true });
  fs.cpSync(backupPath, targetPath, { recursive: true });
}

fs.rmSync(backupDir, { force: true, recursive: true });
