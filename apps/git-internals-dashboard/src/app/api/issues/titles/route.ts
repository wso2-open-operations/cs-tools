import type { NextRequest } from "next/server";
import { requireAuth } from "@/server/auth";
import { titlesBody } from "@/server/schemas";
import { getIssueTitles } from "@/server/lib/issue-titles";
import { RateLimiter } from "@/server/lib/rate-limit";

// ReteLimit: 30 requests/min/user
// Each request already caps at 200 ids (titlesBody); 
const titlesRateLimiter = new RateLimiter(30, 60_000);

// POST /api/issues/titles
// Return: Git issue titles of given ids
export const POST = requireAuth(async (req: NextRequest, _ctx, user) => {
  if (!titlesRateLimiter.allow(user.sub)) {
    return Response.json({ error: "rate_limited" }, { status: 429 });
  }
  const body = await req.json().catch(() => null);
  const parsed = titlesBody.safeParse(body);
  if (!parsed.success) return Response.json({ error: parsed.error.flatten() }, { status: 400 });
  const titles = await getIssueTitles(parsed.data.ids);
  return Response.json({ titles });
});
