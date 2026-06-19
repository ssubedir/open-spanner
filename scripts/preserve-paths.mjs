import fs from "node:fs";
import path from "node:path";

const [backupRoot, ...paths] = process.argv.slice(2);

if (!backupRoot || paths.length === 0) {
  console.error("usage: node scripts/preserve-paths.mjs <backup-dir> <path>...");
  process.exit(1);
}

const cwd = process.cwd();
const backupDir = path.resolve(cwd, backupRoot);
const manifest = [];

fs.rmSync(backupDir, { force: true, recursive: true });

for (const inputPath of paths) {
  const absolutePath = path.resolve(cwd, inputPath);

  if (!fs.existsSync(absolutePath)) {
    continue;
  }

  const stats = fs.statSync(absolutePath);
  const relativePath = path.relative(cwd, absolutePath);
  const backupPath = path.join(backupDir, relativePath);

  fs.mkdirSync(path.dirname(backupPath), { recursive: true });
  fs.cpSync(absolutePath, backupPath, { recursive: true });

  manifest.push({
    path: relativePath,
    type: stats.isDirectory() ? "directory" : "file",
  });
}

fs.mkdirSync(backupDir, { recursive: true });
fs.writeFileSync(path.join(backupDir, ".manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
