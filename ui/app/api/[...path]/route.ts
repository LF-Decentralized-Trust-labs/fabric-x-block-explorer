/**
 * Catch-all proxy: /api/* -> ${BACKEND_URL}/*
 *
 * BACKEND_URL is read from the environment on every request.
 * The more specific app/api/health/route.ts takes precedence over this catch-all.
 */
import { NextRequest } from "next/server";

// Never statically evaluate this route — it must run per request to read env + proxy.
export const dynamic = "force-dynamic";

// Hop-by-hop headers must not be forwarded. We also strip content-encoding/length
// because Node's fetch transparently decompresses the upstream body, leaving those
// headers inconsistent with the bytes we forward.
const STRIP_RESPONSE_HEADERS = [
  "connection",
  "keep-alive",
  "transfer-encoding",
  "te",
  "trailer",
  "upgrade",
  "proxy-authenticate",
  "proxy-authorization",
  "content-encoding",
  "content-length",
];

async function proxy(req: NextRequest, { params }: { params: { path: string[] } }) {
  const backend = (process.env.BACKEND_URL || "http://localhost:8080").replace(/\/+$/, "");
  const target = `${backend}/${params.path.join("/")}${req.nextUrl.search}`;

  const headers = new Headers(req.headers);
  headers.delete("host");

  const init: RequestInit & { duplex?: "half" } = {
    method: req.method,
    headers,
    redirect: "manual",
  };
  if (req.method !== "GET" && req.method !== "HEAD") {
    init.body = req.body;
    init.duplex = "half"; // required by Node fetch when streaming a request body
  }

  let upstream: Response;
  try {
    upstream = await fetch(target, init);
  } catch {
    return Response.json(
      { error: "Bad Gateway", detail: `Cannot reach backend at ${backend}` },
      { status: 502 },
    );
  }

  const resHeaders = new Headers(upstream.headers);
  STRIP_RESPONSE_HEADERS.forEach((h) => resHeaders.delete(h));

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: resHeaders,
  });
}

export const GET = proxy;
export const POST = proxy;
export const PUT = proxy;
export const PATCH = proxy;
export const DELETE = proxy;
export const HEAD = proxy;
export const OPTIONS = proxy;
