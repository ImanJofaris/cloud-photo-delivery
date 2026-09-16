// Local E2E helper: the Go API rate-limits login/refresh per IP (10/min). The
// whole suite shares 127.0.0.1, so it exhausts the bucket and sessions die.
// This proxy forwards to the API with a fresh X-Forwarded-For per request so
// the limiter sees each request as a different client, like real traffic.
import http from "node:http"

const port = Number(process.env.E2E_PROXY_PORT ?? 18080)
const target = new URL(process.env.E2E_API_ORIGIN ?? "http://127.0.0.1:18081")
let counter = 0

function forwardedFor() {
  counter += 1
  const third = (counter >> 8) & 255
  const fourth = (counter % 250) + 1
  return `10.${(counter >> 16) & 255}.${third}.${fourth}`
}

const server = http.createServer((request, response) => {
  const headers = {
    ...request.headers,
    host: target.host,
    "x-forwarded-for": forwardedFor(),
  }

  const upstream = http.request(
    {
      hostname: target.hostname,
      port: target.port,
      path: request.url,
      method: request.method,
      headers,
    },
    (upstreamResponse) => {
      response.writeHead(
        upstreamResponse.statusCode ?? 502,
        upstreamResponse.headers
      )
      upstreamResponse.pipe(response)
    }
  )

  upstream.on("error", () => {
    if (!response.headersSent) {
      response.writeHead(502, { "content-type": "application/json" })
    }
    response.end(
      JSON.stringify({
        data: null,
        error: { code: "BAD_GATEWAY", message: "API unreachable" },
      })
    )
  })

  request.pipe(upstream)
})

server.listen(port, () => {
  console.log(`e2e api proxy listening on :${port} -> ${target.origin}`)
})
