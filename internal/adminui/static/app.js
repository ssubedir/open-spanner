const state = {
  meters: [],
};

const apiStatus = document.querySelector("#api-status");
const toast = document.querySelector("#toast");
const meterForm = document.querySelector("#meter-form");
const usageForm = document.querySelector("#usage-form");
const queryForm = document.querySelector("#query-form");
const metersBody = document.querySelector("#meters-body");
const usageBody = document.querySelector("#usage-body");
const usageTotal = document.querySelector("#usage-total");
const usageCount = document.querySelector("#usage-count");

function showToast(message, kind = "ok") {
  toast.textContent = message;
  toast.classList.toggle("error", kind === "error");
  toast.hidden = false;
  window.clearTimeout(showToast.timer);
  showToast.timer = window.setTimeout(() => {
    toast.hidden = true;
  }, 3200);
}

async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });

  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }

  if (response.status === 204) {
    return null;
  }

  return response.json();
}

function localDateTimeToISO(value) {
  if (!value) {
    return "";
  }
  return new Date(value).toISOString();
}

function setDefaultQueryDates() {
  const now = new Date();
  const from = new Date(now);
  const to = new Date(now);
  from.setDate(now.getDate() - 7);
  to.setDate(now.getDate() + 1);

  queryForm.elements.from.value = toInputDateTime(from);
  queryForm.elements.to.value = toInputDateTime(to);
}

function toInputDateTime(date) {
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000);
  return local.toISOString().slice(0, 16);
}

function renderMeters() {
  if (state.meters.length === 0) {
    metersBody.innerHTML = '<tr><td colspan="4" class="empty">No meters yet</td></tr>';
  } else {
    metersBody.innerHTML = state.meters.map((meter) => `
      <tr>
        <td>${escapeHTML(meter.name)}</td>
        <td>${escapeHTML(meter.unit)}</td>
        <td>${escapeHTML(meter.aggregation)}</td>
        <td class="mono">${escapeHTML(meter.id)}</td>
      </tr>
    `).join("");
  }

  for (const select of document.querySelectorAll('select[name="meter"]')) {
    const current = select.value;
    select.innerHTML = '<option value="">Select meter</option>' + state.meters.map((meter) => (
      `<option value="${escapeHTML(meter.name)}">${escapeHTML(meter.name)}</option>`
    )).join("");
    select.value = current;
  }
}

async function loadMeters() {
  state.meters = await request("/v1/meters");
  renderMeters();
}

async function checkAPI() {
  try {
    const response = await fetch("/health");
    if (!response.ok) {
      throw new Error(response.statusText);
    }
    apiStatus.textContent = "API online";
    apiStatus.className = "status ok";
  } catch (error) {
    apiStatus.textContent = "API offline";
    apiStatus.className = "status error";
  }
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

meterForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(meterForm);

  try {
    await request("/v1/meters", {
      method: "POST",
      body: JSON.stringify({
        name: form.get("name"),
        unit: form.get("unit"),
        description: form.get("description"),
        aggregation: "sum",
      }),
    });
    meterForm.reset();
    await loadMeters();
    showToast("Meter created");
  } catch (error) {
    showToast(error.message, "error");
  }
});

usageForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(usageForm);
  let metadata = {};

  try {
    const metadataText = String(form.get("metadata") || "").trim();
    if (metadataText !== "") {
      metadata = JSON.parse(metadataText);
    }

    await request("/v1/usages", {
      method: "POST",
      body: JSON.stringify({
        subject: form.get("subject"),
        meter: form.get("meter"),
        quantity: Number(form.get("quantity")),
        timestamp: localDateTimeToISO(form.get("timestamp")),
        metadata,
      }),
    });
    queryForm.elements.subject.value = form.get("subject");
    queryForm.elements.meter.value = form.get("meter");
    usageForm.reset();
    usageForm.elements.quantity.value = "1";
    showToast("Usage created");
  } catch (error) {
    showToast(error.message, "error");
  }
});

queryForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = new FormData(queryForm);
  const params = new URLSearchParams({
    subject: form.get("subject"),
    meter: form.get("meter"),
    from: localDateTimeToISO(form.get("from")),
    to: localDateTimeToISO(form.get("to")),
    bucket_size: form.get("bucket_size"),
  });

  try {
    const buckets = await request(`/v1/usages?${params.toString()}`);
    renderUsage(buckets);
    showToast("Usage loaded");
  } catch (error) {
    showToast(error.message, "error");
  }
});

function renderUsage(buckets) {
  if (buckets.length === 0) {
    usageBody.innerHTML = '<tr><td colspan="4" class="empty">No usage found</td></tr>';
  } else {
    usageBody.innerHTML = buckets.map((bucket) => `
      <tr>
        <td>${escapeHTML(bucket.bucket_start)}</td>
        <td>${escapeHTML(bucket.subject)}</td>
        <td>${escapeHTML(bucket.meter)}</td>
        <td>${escapeHTML(bucket.quantity)}</td>
      </tr>
    `).join("");
  }

  const total = buckets.reduce((sum, bucket) => sum + Number(bucket.quantity || 0), 0);
  usageTotal.textContent = String(total);
  usageCount.textContent = String(buckets.length);
}

document.querySelector("#refresh-meters").addEventListener("click", async () => {
  try {
    await loadMeters();
    showToast("Meters refreshed");
  } catch (error) {
    showToast(error.message, "error");
  }
});

setDefaultQueryDates();
checkAPI();
loadMeters().catch((error) => showToast(error.message, "error"));
