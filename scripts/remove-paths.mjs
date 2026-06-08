import fs from "node:fs";

for (const path of process.argv.slice(2)) {
  fs.rmSync(path, { force: true, recursive: true });
}
