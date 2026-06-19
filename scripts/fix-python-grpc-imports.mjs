import { readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import path from "node:path";

const root = "sdk/python/open_spanner_client/grpc/pb";

function walk(dir) {
  for (const entry of readdirSync(dir)) {
    const fullPath = path.join(dir, entry);
    if (statSync(fullPath).isDirectory()) {
      walk(fullPath);
      continue;
    }

    if (!entry.endsWith("_pb2_grpc.py")) {
      continue;
    }

    const source = readFileSync(fullPath, "utf8");
    const fixed = source.replace(/^from open_spanner\.v1 import (\w+_pb2) as ([\w_]+)$/gm, "from . import $1 as $2");
    if (fixed !== source) {
      writeFileSync(fullPath, fixed);
    }
  }
}

walk(root);
