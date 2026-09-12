# Phase F0 — Web Foundation

> **Status: COMPLETE.** See the [Phase F0 Report](Phase F0 - Report.md) for what was built and how it was verified.

**Goal:** A pnpm workspace containing a runnable Next.js 16 app, a shared shadcn UI package, and a generated typed API client — wired to the Go API, tested, linted, and ready for feature phases.

**Depends on:** Phases 0–2 (auth, events) for the first real features; nothing at the API level is required to scaffold.

**Exit criteria:** `pnpm lint`, `pnpm typecheck`, `pnpm test`, and `pnpm build` pass from a clean clone; `apps/web` renders a shell page that calls a live Go endpoint; a shadcn component added via MCP lands in `packages/ui` and imports as `@workspace/ui/components/*`; `packages/api-client` regenerates from `api/openapi.yaml` with no diff.

---

## 1. Features

- pnpm workspace root with `apps/*` and `packages/*`.
- `apps/web`: Next.js 16 App Router, TypeScript strict, Tailwind CSS v4, ESLint 9 (Next 16 removed `next lint`), Prettier + Tailwind plugin.
- `packages/ui`: shadcn monorepo layout (`components.json`, `src/components`, `src/hooks`, `src/lib`, `src/styles/globals.css`) with the same style/baseColor/iconLibrary as the app.
- `packages/api-client`: `openapi-typescript` output + `openapi-fetch` wrapper that unwraps the `{ data, error }` envelope and injects a Bearer token provider.
- Test scaffolding: Vitest + Testing Library (jsdom) and Playwright with one smoke test.
- shadcn MCP configured in `opencode.json`.
- Repo plumbing: `.gitignore`, `.dockerignore`, `apps/web/.env.example`, CI `web` job.
- Docs: this plan, `doc/frontend/Conventions.md`, AGENTS.md commands, Test Strategy gate 6, Implementation Plan FE track.

---

## 2. Workspace layout

```text
package.json                  # private root, scripts only
pnpm-workspace.yaml
apps/
  web/
    src/app/                  # App Router routes
    src/components/           # app-specific compositions
    src/lib/                  # env, providers, query client
    e2e/                      # Playwright specs
    components.json
    .env.example
packages/
  ui/
    src/components/           # shadcn primitives
    src/lib/utils.ts          # cn()
    src/styles/globals.css    # Tailwind v4 + theme tokens
    components.json
  api-client/
    src/schema.d.ts           # generated, committed
    src/client.ts             # envelope unwrap + auth hook
    src/index.ts
opencode.json                 # shadcn MCP server
```

---

## 3. Tooling and versions

| Tool            | Choice                                   | Notes                                   |
| --------------- | ---------------------------------------- | --------------------------------------- |
| Runtime         | Node 24 (installed)                      | Next 16 requires >= 20.9                |
| Package manager | pnpm via `corepack enable`               | Do not commit npm/yarn lockfiles        |
| Framework       | Next.js 16.3.x                           | Turbopack default                       |
| UI              | shadcn/ui + Tailwind CSS v4              | Same style/baseColor in both workspaces |
| API types       | `openapi-typescript` + `openapi-fetch`   | Generated from `api/openapi.yaml`       |
| Server state    | `@tanstack/react-query` v5               |                                         |
| Forms           | `react-hook-form` + `zod` + shadcn form  |                                         |
| Unit tests      | Vitest + Testing Library                 | jsdom                                   |
| E2E             | Playwright                               | One smoke test for now                  |
| Formatting      | Prettier + `prettier-plugin-tailwindcss` |                                         |

Root scripts:

```text
pnpm dev         -> pnpm --filter web dev
pnpm build       -> pnpm --filter web build
pnpm lint        -> pnpm -r lint
pnpm typecheck   -> pnpm -r typecheck
pnpm test        -> pnpm -r test
pnpm test:e2e    -> pnpm --filter web test:e2e
pnpm gen:api     -> openapi-typescript api/openapi.yaml -o packages/api-client/src/schema.d.ts
```

No Turborepo until a second JS app exists.

---

## 4. Configuration

`apps/web/.env.example`:

```text
API_BASE_URL=http://localhost:8080
```

- `API_BASE_URL` is server-only (BFF route handlers and server components).
- Browser calls in later phases need a public base URL; add `NEXT_PUBLIC_API_BASE_URL` when the first direct browser call lands (public gallery, F4).
- Local port gotcha: if WinNAT reserves `:8080`, run the Go API with `HTTP_ADDR=:18080` and point `API_BASE_URL` at it.

---

## 5. shadcn MCP

`opencode.json` at the repo root:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "shadcn": {
      "type": "local",
      "command": ["npx", "shadcn@latest", "mcp"]
    }
  }
}
```

- Run shadcn commands from `apps/web` (`pnpm dlx shadcn@latest add <component>`) so the CLI resolves both `components.json` files and routes primitives to `packages/ui`.
- Requirement: matching `style`, `baseColor`, `iconLibrary` in both `components.json`; Tailwind v4 keeps the `tailwind.config` field empty.
- Windows path bug: shadcn CLI 4.20/4.21 truncates resolved paths when the repo path contains a dot (covered by `scripts/shadcn-path-fix.cjs`). Use `pnpm shadcn add <component> -c apps/web`; the MCP server loads the same fix via `NODE_OPTIONS`.

---

## 6. Test Plan

**Unit**

- [ ] `packages/api-client`: envelope unwrap returns `data`; `error` throws `ApiError` with code/status; auth header injected when a token provider is set.
- [ ] `apps/web`: a rendered shell component shows text and respects providers.

**E2E (Playwright)**

- [ ] App boots, renders the shell route, and the API health check through the app returns 200.

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e        # requires browsers installed
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
```

---

## 7. Definition of Done

- [ ] All master DoD items that apply (OpenAPI untouched in this phase; it is consumed, not changed).
- [ ] Clean clone: `corepack enable && pnpm install && pnpm build` succeeds.
- [ ] A shadcn component added via MCP is importable from `apps/web` as `@workspace/ui/components/<name>`.
- [ ] API types regenerate with no diff.
- [ ] `.gitignore` covers `node_modules/`, `.next/`, `test-results/`, `playwright-report/`, `*.tsbuildinfo`; `.dockerignore` keeps node artifacts out of the Go image context.
- [ ] CI has a `web` job running lint, typecheck, test, build (Go jobs re-enabled alongside).
- [ ] AGENTS.md, Test Strategy, and Implementation Plan updated.
- [ ] Completion report written.
