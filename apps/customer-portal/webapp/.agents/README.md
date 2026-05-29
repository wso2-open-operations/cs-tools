# Customer Portal Webapp — Agent Architecture Docs

This directory contains comprehensive architectural documentation for the Customer Portal webapp,
intended for agentic development, LLM-assisted maintenance, and onboarding new AI agents.

## Files

| File | Contents |
|------|----------|
| `01-OVERVIEW.md` | Tech stack, folder structure, path aliases, conventions, license headers |
| `02-NAVIGATION.md` | Router setup, full route table, layouts, guards, URL params, session state, cross-feature nav |
| `03-API-LAYER.md` | Auth fetch, React Query config, all API hooks by feature, error handling, pagination patterns |
| `04-FEATURES-CORE.md` | Dashboard, Support, Engagements — pages, components, state, user flows |
| `05-FEATURES-OTHER.md` | Operations, Security, Announcements, Project Details, Settings, Updates, Usage Metrics, Project Hub |
| `06-SHARED-LAYER.md` | Providers, shared components, hooks, contexts, types, utils, constants |
| `07-PACKAGING.md` | Build system, dist output, microapp deployment, `config.js` pattern, zip instructions |

## Quick orientation

- **Stack:** React 19 + Vite 7 + TypeScript 5.9 + TanStack Query 5 + React Router 7 + Oxygen UI (WSO2 MUI fork)
- **Auth:** Asgardeo OIDC via `@asgardeo/react`
- **State:** React Query for server state; `useSessionState` (sessionStorage) for list UI; React local state for forms/modals
- **Styling:** Oxygen UI `sx` props exclusively — no CSS files; Acrylic Orange theme
- **Test:** Vitest + @testing-library/react (colocated `__tests__/`)
- **Entry:** `src/main.tsx` → `AppWithConfig.tsx` → `App.tsx` → route tree

## Key invariants every agent must know

1. **Every source file** must start with the WSO2 Apache 2.0 license header (15 `//` comment lines) followed by a blank line. See `src/layouts/AppLayout.tsx` lines 1–15 for the canonical template.
2. **No border radius** except `borderRadius: "50%"` for circular elements (avatars, dots).
3. **Pages are composition-only** — no inline component implementations inside `src/features/*/pages/`.
4. **Path aliases** are declared in both `vite.config.ts` and `tsconfig.app.json` — keep them in sync when adding new ones.
5. **`window.config`** is the sole runtime config mechanism — no `import.meta.env`, no `.env` files used by app code.
6. **`issueIds: number[]`** (not `issueId`) is the multi-select category filter field introduced for the category multi-select feature. Backend compatibility should be confirmed.
7. All API hooks gate on `isSignedIn && !isAuthLoading` from `useAsgardeo()`.
8. `useSessionState` with `popOnly: true` is the standard pattern for list page filter/sort/page persistence.
