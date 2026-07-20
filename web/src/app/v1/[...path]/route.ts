const hopByHopHeaders = new Set([
  "connection",
  "content-encoding",
  "content-length",
  "keep-alive",
  "proxy-authenticate",
  "proxy-authorization",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
])

type RouteContext = {
  params: Promise<{ path: string[] }>
}

async function proxy(request: Request, context: RouteContext) {
  const { path } = await context.params
  const target = new URL(process.env.OPEN_SPANNER_API_PROXY_URL ?? "http://127.0.0.1:18080")
  const requestURL = new URL(request.url)

  target.pathname = `/v1/${path.map(encodeURIComponent).join("/")}`
  target.search = requestURL.search

  const headers = new Headers(request.headers)
  headers.delete("connection")
  headers.delete("content-length")
  headers.delete("host")
  headers.set("accept-encoding", "identity")
  headers.set("x-forwarded-host", requestURL.host)
  headers.set("x-forwarded-proto", request.headers.get("x-forwarded-proto") ?? requestURL.protocol.slice(0, -1))

  const hasBody = request.method !== "GET" && request.method !== "HEAD"
  let upstream: Response
  try {
    upstream = await fetch(target, {
      body: hasBody ? await request.arrayBuffer() : undefined,
      headers,
      method: request.method,
      redirect: "manual",
      signal: request.signal,
    })
  } catch (error) {
    if (request.signal.aborted) {
      return new Response(null, { status: 499 })
    }
    throw error
  }

  const responseHeaders = new Headers(upstream.headers)
  for (const header of hopByHopHeaders) {
    responseHeaders.delete(header)
  }

  return new Response(request.method === "HEAD" ? null : upstream.body, {
    headers: responseHeaders,
    status: upstream.status,
    statusText: upstream.statusText,
  })
}

export const dynamic = "force-dynamic"

export const DELETE = proxy
export const GET = proxy
export const HEAD = proxy
export const OPTIONS = proxy
export const PATCH = proxy
export const POST = proxy
export const PUT = proxy
