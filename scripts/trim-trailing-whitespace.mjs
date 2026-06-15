import fs from "node:fs";
import path from "node:path";

const roots = process.argv.slice(2);

if (roots.length === 0) {
  console.error("Usage: node scripts/trim-trailing-whitespace.mjs <path> [...]");
  process.exit(1);
}

const trimFile = (filePath) => {
  const source = fs.readFileSync(filePath, "utf8");
  const updated = source.replace(/\r\n/g, "\n").replace(/[ \t]+$/gm, "");

  if (updated !== source) {
    fs.writeFileSync(filePath, updated);
  }
};

const walk = (entryPath) => {
  if (!fs.existsSync(entryPath)) {
    return;
  }

  const stat = fs.statSync(entryPath);

  if (stat.isDirectory()) {
    for (const entry of fs.readdirSync(entryPath)) {
      walk(path.join(entryPath, entry));
    }

    return;
  }

  if (entryPath.endsWith(".cs") || entryPath.endsWith(".json") || entryPath.endsWith(".py")) {
    trimFile(entryPath);
  }
};

for (const root of roots) {
  walk(root);
}
