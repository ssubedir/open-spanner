import fs from "node:fs";

const [inputPath, outputPath] = process.argv.slice(2);

if (!inputPath || !outputPath) {
  console.error("usage: node scripts/normalize-sdk-specs.mjs <input> <output>");
  process.exit(1);
}

const schemaNames = new Map([
  ["internal_metering_adapters_http_meter.CreateRequest", "MeterCreateRequest"],
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
  ["internal_metering_adapters_http_usage.CreateRequest", "UsageCreateRequest"],
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

fs.writeFileSync(outputPath, `${JSON.stringify(spec, null, 2)}\n`);
