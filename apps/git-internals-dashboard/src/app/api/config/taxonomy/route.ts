import { requireAuth } from "@/server/auth";
import { getCsStatuses, getStatusDefinitions } from "@/server/lib/taxonomy";

// GET /api/config/taxonomy
// Return: Full status taxonomy, config-driven.
export const GET = requireAuth(async () => {
  const [statuses, csStatuses] = await Promise.all([getStatusDefinitions(), getCsStatuses()]);
  return Response.json({ statuses, csStatuses });
});
