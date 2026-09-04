let started = false;

export async function register() {
  if (process.env.NEXT_RUNTIME !== "nodejs") return;
  if (started) return;
  started = true;

  const { syncConfigToDb } = await import("@/server/db");
  const { startRecomputeJob } = await import("@/server/jobs/recompute");
  const { logger } = await import("@/server/lib/logger");

  const summary = await syncConfigToDb();
  logger.info(`config sync: ${summary.activeRepos} repos active, ${summary.disabledRepos} disabled`);

  if ((process.env.AUTH_MODE ?? "stub").trim() !== "asgardeo") {
    if (process.env.NODE_ENV === "production") {
      throw new Error(
        'AUTH_MODE must be "asgardeo" in production — refusing to boot with unauthenticated stub auth.'
      );
    }
    logger.warn("AUTH_MODE=stub — every request is an unauthenticated stub user. Never run this in production.");
  }

  await startRecomputeJob(logger);
}
