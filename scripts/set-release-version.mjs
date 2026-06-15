import fs from "node:fs";

const [rawVersion, ...extraArgs] = process.argv.slice(2);

if (!rawVersion || extraArgs.length > 0) {
  console.error("Usage: node scripts/set-release-version.mjs <version>");
  console.error("Example: node scripts/set-release-version.mjs 0.1.4");
  process.exit(1);
}

const version = rawVersion.replace(/^v/, "");

if (!/^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/.test(version)) {
  console.error("Version must look like 0.1.4 or 0.1.4-rc.1");
  process.exit(1);
}

const updateTextFile = (path, update) => {
  const source = fs.readFileSync(path, "utf8");
  const updated = update(source);

  if (updated === source) {
    return false;
  }

  fs.writeFileSync(path, updated);
  return true;
};

const updateJsonFile = (path, update) =>
  updateTextFile(path, (source) => {
    const json = JSON.parse(source);
    update(json);
    return `${JSON.stringify(json, null, 2)}\n`;
  });

const replaceRequired = (source, pattern, replacement, path) => {
  if (!pattern.test(source)) {
    throw new Error(`Could not find version field in ${path}`);
  }

  return source.replace(pattern, replacement);
};

const changed = [];

if (
  updateJsonFile("sdk/typescript/package.json", (pkg) => {
    pkg.version = version;
  })
) {
  changed.push("sdk/typescript/package.json");
}

if (
  updateJsonFile("sdk/typescript/package-lock.json", (lockfile) => {
    lockfile.version = version;

    if (lockfile.packages?.[""]) {
      lockfile.packages[""].version = version;
    }
  })
) {
  changed.push("sdk/typescript/package-lock.json");
}

if (
  updateTextFile("sdk/python/pyproject.toml", (source) =>
    replaceRequired(source, /^version = "[^"]+"$/m, `version = "${version}"`, "sdk/python/pyproject.toml"),
  )
) {
  changed.push("sdk/python/pyproject.toml");
}

if (
  updateTextFile("sdk/python/uv.lock", (source) =>
    replaceRequired(
      source,
      /(name = "open-spanner"\r?\nversion = ")[^"]+(")/,
      `$1${version}$2`,
      "sdk/python/uv.lock",
    ),
  )
) {
  changed.push("sdk/python/uv.lock");
}

if (
  updateTextFile("sdk/python-client.yml", (source) =>
    replaceRequired(
      source,
      /^package_version_override: .+$/m,
      `package_version_override: ${version}`,
      "sdk/python-client.yml",
    ),
  )
) {
  changed.push("sdk/python-client.yml");
}

if (
  updateTextFile("sdk/csharp/OpenSpanner.csproj", (source) =>
    replaceRequired(
      source,
      /<Version>[^<]+<\/Version>/,
      `<Version>${version}</Version>`,
      "sdk/csharp/OpenSpanner.csproj",
    ),
  )
) {
  changed.push("sdk/csharp/OpenSpanner.csproj");
}

if (changed.length === 0) {
  console.log(`Release version is already ${version}.`);
} else {
  console.log(`Updated release version to ${version}:`);
  for (const path of changed) {
    console.log(`- ${path}`);
  }
}
