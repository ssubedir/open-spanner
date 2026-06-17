import fs from "node:fs";

const [inputPath, outputPath] = process.argv.slice(2);

if (!inputPath || !outputPath) {
  console.error("usage: node scripts/normalize-sdk-specs.mjs <input> <output>");
  process.exit(1);
}

const schemaNames = new Map([
  ["internal_metering_adapters_http_auth.APIKeyCreateResponse", "APIKeyCreateResponse"],
  ["internal_metering_adapters_http_auth.APIKeyListResponse", "APIKeyListResponse"],
  ["internal_metering_adapters_http_auth.APIKeyResponse", "APIKey"],
  ["internal_metering_adapters_http_auth.CreateAPIKeyRequest", "APIKeyCreateRequest"],
  ["internal_metering_adapters_http_auth.CreateUserRequest", "AuthUserCreateRequest"],
  ["internal_metering_adapters_http_auth.LoginRequest", "AuthLoginRequest"],
  ["internal_metering_adapters_http_auth.LoginResponse", "AuthLoginResponse"],
  ["internal_metering_adapters_http_auth.RefreshResponse", "AuthRefreshResponse"],
  ["internal_metering_adapters_http_auth.SessionResponse", "AuthSessionResponse"],
  ["internal_metering_adapters_http_auth.UserResponse", "AuthUser"],
  ["internal_metering_adapters_http_meter.CreateRequest", "MeterCreateRequest"],
  ["internal_metering_adapters_http_meter.DimensionRequest", "MeterDimensionRequest"],
  ["internal_metering_adapters_http_meter.DimensionResponse", "MeterDimension"],
  ["internal_metering_adapters_http_meter.ListResponse", "MeterListResponse"],
  ["internal_metering_adapters_http_meter.Response", "Meter"],
  ["internal_metering_adapters_http_meter.StatsListResponse", "MeterStatsListResponse"],
  ["internal_metering_adapters_http_meter.StatsResponse", "MeterStats"],
  ["internal_metering_adapters_http_meter.UpdateRequest", "MeterUpdateRequest"],
  ["internal_metering_adapters_http_subject.EventResponse", "SubjectUsageEvent"],
  ["internal_metering_adapters_http_subject.ListResponse", "SubjectStatsListResponse"],
  ["internal_metering_adapters_http_subject.Response", "SubjectStats"],
  ["internal_metering_adapters_http_system.LastPruneRunResponse", "SystemLastPruneRun"],
  ["internal_metering_adapters_http_system.StatsResponse", "SystemStats"],
  ["internal_metering_adapters_http_usage.BulkFailureResponse", "UsageBulkFailure"],
  ["internal_metering_adapters_http_usage.BulkResponse", "UsageBulkResult"],
  ["internal_metering_adapters_http_usage.BreakdownListResponse", "UsageBreakdownListResponse"],
  ["internal_metering_adapters_http_usage.BreakdownRequest", "UsageBreakdownRequest"],
  ["internal_metering_adapters_http_usage.BreakdownResponse", "UsageBreakdown"],
  ["internal_metering_adapters_http_usage.CreateRequest", "UsageCreateRequest"],
  ["internal_metering_adapters_http_usage.DimensionValueListResponse", "UsageDimensionValueListResponse"],
  ["internal_metering_adapters_http_usage.DimensionValueResponse", "UsageDimensionValue"],
  ["internal_metering_adapters_http_usage.EventListResponse", "UsageEventListResponse"],
  ["internal_metering_adapters_http_usage.IngestionListResponse", "UsageIngestionListResponse"],
  ["internal_metering_adapters_http_usage.IngestionResponse", "UsageIngestionRun"],
  ["internal_metering_adapters_http_usage.ListItemResponse", "UsageBucket"],
  ["internal_metering_adapters_http_usage.PruneListResponse", "UsagePruneListResponse"],
  ["internal_metering_adapters_http_usage.PruneMeterResponse", "UsagePruneMeter"],
  ["internal_metering_adapters_http_usage.PruneResponse", "UsagePruneRun"],
  ["internal_metering_adapters_http_usage.Response", "UsageEvent"],
  ["github_com_ssubedir_open-spanner_internal_metering_adapters_http_internal_respond.ErrorBody", "ErrorBody"],
  ["github_com_ssubedir_open-spanner_internal_metering_adapters_http_internal_respond.ErrorResponse", "ErrorResponse"],
  ["open-spanner_internal_metering_adapters_http_internal_respond.ErrorBody", "ErrorBody"],
  ["open-spanner_internal_metering_adapters_http_internal_respond.ErrorResponse", "ErrorResponse"],
]);

const spec = JSON.parse(fs.readFileSync(inputPath, "utf8"));

const sdkOperations = new Map([
  ["/health", new Set(["get"])],
  ["/ready", new Set(["get"])],
  ["/v1/meters", new Set(["get", "post"])],
  ["/v1/meters/{id}", new Set(["delete", "get", "put"])],
  ["/v1/usages", new Set(["post"])],
  ["/v1/usages/breakdowns/search", new Set(["post"])],
  ["/v1/usages/bulk", new Set(["post"])],
  ["/v1/usages/dimensions", new Set(["get"])],
  ["/v1/usages/export", new Set(["get", "post"])],
  ["/v1/usages/search", new Set(["post"])],
]);

for (const [path, pathItem] of Object.entries(spec.paths ?? {})) {
  const allowedMethods = sdkOperations.get(path);
  if (!allowedMethods) {
    delete spec.paths[path];
    continue;
  }

  for (const key of Object.keys(pathItem)) {
    if (isHTTPMethod(key) && !allowedMethods.has(key)) {
      delete pathItem[key];
    }
  }

  if (!Object.keys(pathItem).some(isHTTPMethod)) {
    delete spec.paths[path];
  }
}

function rewriteRefs(value) {
  if (Array.isArray(value)) {
    for (const item of value) {
      rewriteRefs(item);
    }
    return;
  }
  if (!value || typeof value !== "object") {
    return;
  }

  if (typeof value.$ref === "string") {
    value.$ref = rewriteRef(value.$ref);
  }

  for (const nested of Object.values(value)) {
    rewriteRefs(nested);
  }
}

function rewriteRef(ref) {
  for (const [from, to] of schemaNames.entries()) {
    if (ref === `#/definitions/${from}`) {
      return `#/definitions/${to}`;
    }
    if (ref === `#/components/schemas/${from}`) {
      return `#/components/schemas/${to}`;
    }
  }
  return ref;
}

function renameSchemaContainer(container) {
  if (!container) {
    return;
  }

  for (const [from, to] of schemaNames.entries()) {
    if (!Object.prototype.hasOwnProperty.call(container, from)) {
      continue;
    }
    container[to] = container[from];
    container[to].title = to;
    delete container[from];
  }
}

renameSchemaContainer(spec.definitions);
renameSchemaContainer(spec.components?.schemas);
rewriteRefs(spec);
pruneUnreferencedSchemas(spec);

fs.writeFileSync(outputPath, `${JSON.stringify(spec, null, 2)}\n`);

function pruneUnreferencedSchemas(document) {
  const container = document.definitions ?? document.components?.schemas;
  if (!container) {
    return;
  }

  const refs = new Set();
  const queue = [];

  collectRefs(document.paths, refs, queue);

  for (let index = 0; index < queue.length; index += 1) {
    const name = queue[index];
    if (container[name]) {
      collectRefs(container[name], refs, queue);
    }
  }

  for (const name of Object.keys(container)) {
    if (!refs.has(name)) {
      delete container[name];
    }
  }
}

function collectRefs(value, refs, queue) {
  if (Array.isArray(value)) {
    for (const item of value) {
      collectRefs(item, refs, queue);
    }
    return;
  }
  if (!value || typeof value !== "object") {
    return;
  }

  if (typeof value.$ref === "string") {
    const name = schemaNameFromRef(value.$ref);
    if (name && !refs.has(name)) {
      refs.add(name);
      queue.push(name);
    }
  }

  for (const nested of Object.values(value)) {
    collectRefs(nested, refs, queue);
  }
}

function schemaNameFromRef(ref) {
  const definitionsPrefix = "#/definitions/";
  if (ref.startsWith(definitionsPrefix)) {
    return ref.slice(definitionsPrefix.length);
  }

  const schemasPrefix = "#/components/schemas/";
  if (ref.startsWith(schemasPrefix)) {
    return ref.slice(schemasPrefix.length);
  }

  return undefined;
}

function isHTTPMethod(key) {
  return ["delete", "get", "head", "options", "patch", "post", "put", "trace"].includes(key);
}
