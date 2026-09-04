// src/server/db/config-sync.ts
// Boot-time config→DB sync. Idempotent, safe to run on every boot:
// repositories/projects rows are a derived cache for referential integrity
// only — nothing edits them directly anymore. Never touches `lastSyncedAt`
// (that belongs to the seed/sync, which actually fetch data).
import { loadConfig, type AppConfig } from "@/server/config";
import { prisma } from "./client";

export interface ConfigSyncSummary {
  activeRepos: number;
  disabledRepos: number;
}

export async function syncConfigToDb(app: AppConfig = loadConfig()): Promise<ConfigSyncSummary> {
  const projectIdByGh = new Map<string, number>();
  for (const r of app.repos) {
    if (projectIdByGh.has(r.githubProjectId)) continue;
    const project = await prisma.project.upsert({
      where: { githubProjectId: r.githubProjectId },
      create: { githubProjectId: r.githubProjectId, title: r.projectTitle, enabled: true },
      update: { title: r.projectTitle, enabled: true },
    });
    projectIdByGh.set(r.githubProjectId, project.id);
  }

  for (const r of app.repos) {
    const slaProjectId = projectIdByGh.get(r.githubProjectId)!;
    await prisma.repository.upsert({
      where: { owner_name: { owner: r.owner, name: r.name } },
      create: {
        owner: r.owner,
        name: r.name,
        issueQuery: r.issueQuery,
        htmlUrl: `https://github.com/${r.owner}/${r.name}`,
        slaProjectId,
        enabled: true,
      },
      update: {
        issueQuery: r.issueQuery,
        htmlUrl: `https://github.com/${r.owner}/${r.name}`,
        slaProjectId,
        enabled: true,
      },
    });
  }

  // Disable (never delete) any Repository/Project row not present in the file
  // — FK children keep their history; routes already filter on `enabled`.
  const configuredRepoKeys = new Set(app.repos.map((r) => `${r.owner}/${r.name}`));
  const configuredProjectIds = new Set(app.repos.map((r) => r.githubProjectId));

  const enabledRepos = await prisma.repository.findMany({
    where: { enabled: true },
    select: { id: true, owner: true, name: true },
  });
  const repoIdsToDisable = enabledRepos
    .filter((r) => !configuredRepoKeys.has(`${r.owner}/${r.name}`))
    .map((r) => r.id);
  if (repoIdsToDisable.length > 0) {
    await prisma.repository.updateMany({ where: { id: { in: repoIdsToDisable } }, data: { enabled: false } });
  }

  const enabledProjects = await prisma.project.findMany({
    where: { enabled: true },
    select: { id: true, githubProjectId: true },
  });
  const projectIdsToDisable = enabledProjects
    .filter((p) => !configuredProjectIds.has(p.githubProjectId))
    .map((p) => p.id);
  if (projectIdsToDisable.length > 0) {
    await prisma.project.updateMany({ where: { id: { in: projectIdsToDisable } }, data: { enabled: false } });
  }

  return { activeRepos: app.repos.length, disabledRepos: repoIdsToDisable.length };
}
