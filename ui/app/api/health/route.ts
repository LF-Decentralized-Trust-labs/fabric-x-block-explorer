/**
 * GET /api/health
 *
 * Liveness probe for the Next.js UI container.
 * Used by Docker Compose `depends_on: condition: service_healthy` and by
 * external load balancers / Kubernetes readiness probes.
 *
 * Returns 200 immediately — no backend call is made here.
 * Backend health is checked separately via the explorer's own /healthz route.
 */
export function GET() {
  return Response.json(
    { status: "ok", service: "fabric-x-block-explorer-ui" },
    { status: 200 }
  );
}
