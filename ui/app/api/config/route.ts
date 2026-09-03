/**
 * GET /api/config
 *
 * Returns runtime configuration that is not safe to bake in at build time.
 * The sidebar uses this to display the actual backend URL regardless of
 * when the Docker image was built.
 */
export const dynamic = "force-dynamic";

export function GET() {
  return Response.json({
    backendUrl: (process.env.BACKEND_URL || "http://localhost:8080").replace(/\/+$/, ""),
  });
}
