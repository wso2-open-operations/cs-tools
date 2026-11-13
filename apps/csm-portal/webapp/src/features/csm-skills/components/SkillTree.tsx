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

import { Box, Typography } from "@wso2/oxygen-ui";
import { Check, ChevronDown, ChevronRight } from "@wso2/oxygen-ui-icons-react";
import type { JSX, KeyboardEvent } from "react";
import type { SkillTreeNode } from "@features/csm-skills/utils/skillTree";
import type { Skill } from "@features/csm-skills/types";

interface SkillTreeProps {
  nodes: readonly SkillTreeNode[];
  expanded: ReadonlySet<string>;
  onToggle: (categoryId: string) => void;
  selectedSkillId: string | null;
  onSelect: (skill: Skill) => void;
  /** Skills on the caller's profile, marked with a tick. */
  heldSkillIds: ReadonlySet<string>;
}

const ROW_SX = {
  display: "flex",
  alignItems: "center",
  gap: 0.75,
  py: 0.5,
  pr: 1,
  borderRadius: 1,
  cursor: "pointer",
  outline: "none",
  "&:hover": { bgcolor: "action.hover" },
  "&:focus-visible": { outline: "2px solid", outlineColor: "primary.main", outlineOffset: "-2px" },
} as const;

/** Activates a row on Enter or Space, and opens or closes a category with the arrow keys. */
function rowKeys(e: KeyboardEvent, onActivate: () => void, arrows?: { open: boolean; toggle: () => void }): void {
  if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    onActivate();
  } else if (arrows && ((e.key === "ArrowRight" && !arrows.open) || (e.key === "ArrowLeft" && arrows.open))) {
    e.preventDefault();
    arrows.toggle();
  }
}

/**
 * The All Skills tree: categories (with how many skills sit under each) that
 * open and close, and the skills filed under them. Selecting a skill shows it
 * beside the tree. A tick marks the caller's own skills.
 */
export default function SkillTree({ nodes, expanded, onToggle, selectedSkillId, onSelect, heldSkillIds }: SkillTreeProps): JSX.Element {
  const renderSkill = (skill: Skill, depth: number, parentId: string): JSX.Element => {
    const selected = skill.id === selectedSkillId;
    return (
      <Box
        key={`${parentId}:${skill.id}`}
        component="li"
        role="treeitem"
        aria-selected={selected}
        tabIndex={0}
        onClick={() => onSelect(skill)}
        onKeyDown={(e) => rowKeys(e, () => onSelect(skill))}
        sx={[
          ROW_SX,
          { pl: `${8 + depth * 16 + 22}px` },
          selected ? { bgcolor: "action.selected", "&:hover": { bgcolor: "action.selected" } } : {},
        ]}
      >
        <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0, fontWeight: selected ? 600 : 400 }}>
          {skill.name}
        </Typography>
        {heldSkillIds.has(skill.id) && (
          <Box component="span" aria-label="On your profile" sx={{ display: "inline-flex", color: "success.main" }}>
            <Check size={14} />
          </Box>
        )}
      </Box>
    );
  };

  const renderNode = (node: SkillTreeNode, depth: number): JSX.Element => {
    const open = expanded.has(node.id);
    return (
      <Box key={node.id} component="li" role="treeitem" aria-expanded={open} aria-selected={false} sx={{ listStyle: "none" }}>
        <Box
          tabIndex={0}
          onClick={() => onToggle(node.id)}
          onKeyDown={(e) => rowKeys(e, () => onToggle(node.id), { open, toggle: () => onToggle(node.id) })}
          sx={[ROW_SX, { pl: `${8 + depth * 16}px` }]}
        >
          <Box component="span" aria-hidden sx={{ display: "inline-flex", color: "text.secondary" }}>
            {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          </Box>
          <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0, fontWeight: 600 }}>
            {node.name}
          </Typography>
          <Typography variant="caption" color="text.secondary" aria-label={`${node.total} skills`}>
            {node.total}
          </Typography>
        </Box>
        {open && (
          <Box component="ul" role="group" sx={{ m: 0, p: 0, listStyle: "none" }}>
            {node.children.map((c) => renderNode(c, depth + 1))}
            {node.skills.map((s) => renderSkill(s, depth + 1, node.id))}
          </Box>
        )}
      </Box>
    );
  };

  return (
    <Box component="ul" role="tree" aria-label="Skills by category" sx={{ m: 0, p: 0.5, listStyle: "none" }}>
      {nodes.map((n) => renderNode(n, 0))}
    </Box>
  );
}
