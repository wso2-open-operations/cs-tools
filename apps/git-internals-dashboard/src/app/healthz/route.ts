// GET /healthz
// Return: 200 OK if the server is healthy
export async function GET() {
  return Response.json({ ok: true });
}
