import type { Skill, SkillCategory, SkillLevelType, UserSkill } from "@features/csm-skills/types";

export const SCALE_A = "00000000-0000-0000-0000-0000000000a1";
export const SCALE_B = "00000000-0000-0000-0000-0000000000b1";

export const levelTypes: SkillLevelType[] = [
  {
    id: SCALE_A,
    name: "Proficiency",
    levels: [
      { id: "lvl-a-expert", name: "Expert", value: 100 },
      { id: "lvl-a-beginner", name: "Beginner", value: 20, color: "#2e7d32" },
      { id: "lvl-a-intermediate", name: "Intermediate", value: 50 },
    ],
  },
  {
    id: SCALE_B,
    name: "Exposure",
    levels: [
      { id: "lvl-b-advanced", name: "Advanced", value: 2 },
      { id: "lvl-b-basic", name: "Basic", value: 1 },
    ],
  },
];

export const categories: SkillCategory[] = [
  { id: "cat-eng", name: "Engineering", categoryPath: "Engineering", isLowestCategory: false, parentId: null },
  { id: "cat-lang", name: "Languages", categoryPath: "Engineering / Languages", isLowestCategory: true, parentId: "cat-eng" },
  { id: "cat-ops", name: "Operations", categoryPath: "Operations", isLowestCategory: true, parentId: null },
];

export const skillGo: Skill = {
  id: "skill-go",
  name: "Go",
  description: "The Go programming language.",
  keywords: "golang",
  active: true,
  levelTypeId: SCALE_A,
  categories: [{ id: "cat-lang", name: "Languages", categoryPath: "Engineering / Languages" }],
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-02T00:00:00Z",
};

export const skillKube: Skill = {
  id: "skill-kube",
  name: "Kubernetes",
  description: null,
  keywords: null,
  active: true,
  levelTypeId: SCALE_B,
  categories: [{ id: "cat-ops", name: "Operations", categoryPath: "Operations" }],
  createdOn: "2026-01-01T00:00:00Z",
  updatedOn: "2026-01-02T00:00:00Z",
};

export const skillOld: Skill = {
  ...skillKube,
  id: "skill-old",
  name: "Legacy Tool",
  active: false,
  categories: [],
};

export const mineGo: UserSkill = {
  skillId: "skill-go",
  skillName: "Go",
  skillActive: true,
  categories: skillGo.categories,
  levelTypeId: SCALE_A,
  level: { id: "lvl-a-beginner", name: "Beginner", value: 20, color: "#2e7d32" },
  inherited: false,
  inheritedFromGroupId: null,
  updatedOn: "2026-02-01T00:00:00Z",
};

export const mineInherited: UserSkill = {
  skillId: "skill-kube",
  skillName: "Kubernetes",
  skillActive: true,
  categories: skillKube.categories,
  levelTypeId: SCALE_B,
  level: { id: "lvl-b-basic", name: "Basic", value: 1 },
  inherited: true,
  inheritedFromGroupId: "00000000-0000-0000-0000-0000000000c1",
  updatedOn: "2026-02-02T00:00:00Z",
};
