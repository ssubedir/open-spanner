const readinessTimeoutMilliseconds = 3_000

async function readiness() {
  const target = new URL("/ready", process.env.OPEN_SPANNER_API_PROXY_URL ?? "http://127.0.0.1:18080")

  try {
    const upstream = await fetch(target, {
      cache: "no-store",
      headers: { accept: "application/json" },
      signal: AbortSignal.timeout(readinessTimeoutMilliseconds),
    })
    if (!upstream.ok) {
      return new Response(null, {
        headers: { "cache-control": "no-store" },
        status: 503,
      })
    }
  } catch {
    return new Response(null, {
      headers: { "cache-control": "no-store" },
      status: 503,
    })
  }

  return new Response(null, {
    headers: { "cache-control": "no-store" },
    status: 204,
  })
}

export const dynamic = "force-dynamic"

export const GET = readiness
export const HEAD = readiness
