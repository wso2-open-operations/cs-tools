// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Alert, Box, Button, Skeleton, TextField, Typography } from "@wso2/oxygen-ui";
import { useMemo, useState, type JSX } from "react";
import { useSearchParams } from "react-router";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useMySkills } from "@features/csm-skills/api/useMySkills";
import { useAllSkills, useSkill, useSkillCategories } from "@features/csm-skills/api/useSkillCatalogue";
import AddSkillDialog from "@features/csm-skills/components/AddSkillDialog";
import RequestSkillDialog from "@features/csm-skills/components/RequestSkillDialog";
import SkillDetailPanel from "@features/csm-skills/components/SkillDetailPanel";
import SkillTree from "@features/csm-skills/components/SkillTree";
import { allNodeIds, buildSkillTree, pathToSkill } from "@features/csm-skills/utils/skillTree";
import type { Skill } from "@features/csm-skills/types";

const SKILL_PARAM = "skill";

interface SkillCatalogueBrowserProps {
  /** The directory capability: whether the people holding a skill are shown. */
  canViewPeople: boolean;
}

/**
 * "All Skills": the active catalogue as a category tree beside the selected
 * skill. The search runs on the backend and opens every branch that matches.
 * The selected skill is kept in the URL (?skill=), so a skill can be linked
 * to. With directory access the selected skill lists the people who hold it.
 */
export default function SkillCatalogueBrowser({ canViewPeople }: SkillCatalogueBrowserProps): JSX.Element {
  const [params, setParams] = useSearchParams();
  const selectedId = params.get(SKILL_PARAM);
  const [search, setSearch] = useState("");
  const debounced = useDebouncedValue(search.trim(), 300);

  const categories = useSkillCategories();
  const skills = useAllSkills(debounced);
  const mine = useMySkills();
  const held = useMemo(() => new Map((mine.data?.skills ?? []).map((s) => [s.skillId, s])), [mine.data]);
  const heldIds = useMemo(() => new Set(held.keys()), [held]);

  const tree = useMemo(() => buildSkillTree(categories.data ?? [], skills.data ?? []), [categories.data, skills.data]);
  const fromList = skills.data?.find((s) => s.id === selectedId);
  // A linked skill outside the current search (or not loaded yet) is read on its own.
  const linked = useSkill(selectedId && !fromList ? selectedId : undefined);
  const selected: Skill | null = fromList ?? (linked.data?.active ? linked.data : null);

  // Open state: the top level to start with; every matching branch while
  // searching; and the way to a skill opened from a link.
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [openedFor, setOpenedFor] = useState<string | null>(null);
  const treeKey = `${debounced}|${tree.length > 0}|${selectedId ?? ""}`;
  if (tree.length > 0 && openedFor !== treeKey) {
    setOpenedFor(treeKey);
    const next = new Set(debounced ? allNodeIds(tree) : openedFor === null ? tree.map((n) => n.id) : expanded);
    for (const id of selectedId ? (pathToSkill(tree, selectedId) ?? []) : []) next.add(id);
    setExpanded(next);
  }
  const toggle = (id: string): void =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const select = (skill: Skill): void =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.set(SKILL_PARAM, skill.id);
        return next;
      },
      { replace: true },
    );

  const [adding, setAdding] = useState<false | { skillId?: string }>(false);
  const [requesting, setRequesting] = useState<false | string>(false);
  const [notice, setNotice] = useState<string | null>(null);

  const loading = skills.isLoading || categories.isLoading;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <TextField
          size="small"
          label="Search skills"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          sx={{ flex: "1 1 240px", maxWidth: 360 }}
          slotProps={{ htmlInput: { "aria-label": "Search skills by name or keyword" } }}
        />
        <Button size="small" onClick={() => setExpanded(new Set(allNodeIds(tree)))} disabled={tree.length === 0}>
          Expand all
        </Button>
        <Button size="small" onClick={() => setExpanded(new Set())} disabled={expanded.size === 0}>
          Collapse all
        </Button>
        <Box sx={{ flexGrow: 1 }} />
        <RefreshButton
          onRefresh={() => void skills.refetch()}
          isFetching={skills.isFetching}
          updatedAt={skills.dataUpdatedAt}
          label="Refresh skills"
        />
      </Box>

      {notice && (
        <Alert severity="success" role="status" onClose={() => setNotice(null)}>
          {notice}
        </Alert>
      )}

      <Box
        sx={{
          display: "grid",
          gap: 2,
          gridTemplateColumns: { xs: "minmax(0, 1fr)", md: "minmax(240px, 320px) minmax(0, 1fr)" },
          alignItems: "start",
        }}
      >
        <Box
          sx={{
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            maxHeight: { md: "70vh" },
            overflowY: "auto",
            opacity: skills.isFetching && !skills.isLoading ? 0.6 : 1,
          }}
        >
          {loading ? (
            <Skeleton variant="rounded" height={240} aria-label="Loading skills" />
          ) : skills.isError ? (
            <QueryErrorState message={skills.error.message || "Failed to load skills."} error={skills.error} onRetry={() => void skills.refetch()} />
          ) : tree.length === 0 ? (
            <Box sx={{ p: 3, textAlign: "center" }}>
              <Typography variant="body2">No skills found.</Typography>
              <Button sx={{ mt: 1.5 }} variant="outlined" size="small" onClick={() => setRequesting(search.trim())}>
                {search.trim() ? `Request “${search.trim()}”` : "Request a skill"}
              </Button>
            </Box>
          ) : (
            <SkillTree
              nodes={tree}
              expanded={expanded}
              onToggle={toggle}
              selectedSkillId={selected?.id ?? null}
              onSelect={select}
              heldSkillIds={heldIds}
            />
          )}
        </Box>

        <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 2, minHeight: 240 }}>
          {selected ? (
            <SkillDetailPanel
              skill={selected}
              mine={held.get(selected.id)}
              canViewPeople={canViewPeople}
              onAdd={(s) => setAdding({ skillId: s.id })}
            />
          ) : selectedId && linked.isLoading ? (
            <Skeleton variant="rounded" height={160} aria-label="Loading skill" />
          ) : (
            <Box sx={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 1, py: 6, textAlign: "center" }}>
              <Typography variant="body1">Select a skill</Typography>
              <Typography variant="body2" color="text.secondary">
                {canViewPeople
                  ? "Choose a skill in the tree to see what it covers and who has it."
                  : "Choose a skill in the tree to see what it covers and its levels."}
              </Typography>
            </Box>
          )}
        </Box>
      </Box>

      {adding && (
        <AddSkillDialog
          heldSkillIds={[...heldIds]}
          initialSkillId={adding.skillId}
          onClose={() => setAdding(false)}
          onRequestSkill={() => {
            setAdding(false);
            setRequesting("");
          }}
        />
      )}
      {requesting !== false && (
        <RequestSkillDialog
          initialName={requesting}
          heldSkillIds={[...heldIds]}
          onClose={(sent) => {
            setRequesting(false);
            if (sent) setNotice(`${sent} You can follow it on the My Skills tab.`);
          }}
          onAddExisting={(skill) => {
            setRequesting(false);
            setAdding({ skillId: skill.id });
          }}
        />
      )}
    </Box>
  );
}
