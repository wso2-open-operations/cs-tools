import { requireAuth } from "@/server/auth";

// GET /api/auth/me
// Return:
//   stub mode: Always reports authorized
//   asgardeo mode: Reports the verified subject once requireAuth has already checked the group membership.
export const GET = requireAuth(async (_req, _ctx, user) => {
  const mode = (process.env.AUTH_MODE ?? "stub").trim();
  if (mode !== "asgardeo") return Response.json({ mode: "stub", authorized: true });
  return Response.json({ mode: "asgardeo", authorized: true, sub: user.sub });
});
