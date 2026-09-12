# Phase F0 — Web Foundation — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-12
**Author:** opencode

---

## 1. Summary

Phase F0 stood up the frontend workspace: a pnpm monorepo with a Next.js 16 app (`apps/web`), a shared shadcn/ui package (`packages/ui`), and a generated typed API client (`packages/api-client`). The Go repository at the root is untouched. `apps/web` renders a dynamic shell page that probes the Go API's `/readyz` endpoint, and all frontend gates (lint, typecheck, unit tests, build, Playwright smoke) pass. The shadcn CLI and its MCP server are configured, including a workaround for a Windows path bug in the CLI.

---

## 2. Exit criteria — verification

| Criteria                                              | Result | Evidence                                                                                                   |
| ----------------------------------------------------- | ------ | ---------------------------------------------------------------------------------------------------------- |
| pnpm workspace with `apps/*` and `packages/*`         | PASS   | `pnpm install` → "Scope: all 4 workspace projects"                                                         |
| Next.js 16 App Router + TS strict + Tailwind v4       | PASS   | `pnpm build` → Next.js 16.3.5 (Turbopack) compiled successfully                                            |
| shadcn component added via CLI lands in `packages/ui` | PASS   | `pnpm shadcn add card -c apps/web` → created `packages/ui/src/components/card.tsx`                         |
| App imports `@workspace/ui/components/*`              | PASS   | `apps/web/src/app/page.tsx` imports `Button`; `pnpm build` resolves it                                     |
| Generated API client with envelope unwrap             | PASS   | `pnpm gen:api` from `api/openapi.yaml`; 6 unit tests pass                                                  |
| Shell page calls a live Go endpoint                   | PASS   | `src/app/page.tsx` fetches `${API_BASE_URL}/readyz` (force-dynamic)                                        |
| Lint                                                  | PASS   | `pnpm lint` → eslint, no output                                                                            |
| Typecheck                                             | PASS   | `pnpm typecheck` → tsc clean for api-client, ui, web                                                       |
| Unit tests                                            | PASS   | `pnpm test` → 9 tests passed (6 api-client, 3 web)                                                         |
| Build                                                 | PASS   | `pnpm build` → static `/` now dynamic, no errors                                                           |
| Playwright smoke                                      | PASS   | `pnpm test:e2e` → 1 passed (10.5s)                                                                         |
| CI web job                                            | PASS   | `.github/workflows/ci.yml` has `web` job (install → gen:api drift check → lint → typecheck → test → build) |
| Docs updated                                          | PASS   | AGENTS.md, Test Strategy, Implementation Plan, `doc/frontend/Conventions.md`, phase F0–F2 plans            |

---

## 3. What was built

### 3.1 Workspace root

- `pnpm-workspace.yaml`, private root `package.json` with `dev/build/lint/typecheck/test/test:e2e/format/gen:api/shadcn` scripts.
- `.prettierrc` (+ Tailwind plugin), `.prettierignore`, `.gitignore` node entries, `.dockerignore` (keeps node artifacts out of the Go image context).

### 3.2 `apps/web`

- Next.js 16.3.5, React 19.2.8, Tailwind CSS v4, ESLint 9.
- `src/app/layout.tsx`: imports `@workspace/ui/globals.css`, Geist fonts, theme provider.
- `src/app/page.tsx`: dynamic shell page fetching `/readyz`.
- `src/components/api-status.tsx` + theme provider.
- `components.json` pointing its `css` at `packages/ui`, aliases to `@workspace/ui/*`.
- `postcss.config.mjs` re-exports the UI package's PostCSS config.
- Vitest (`vitest.config.ts`, `vitest.setup.ts`) and Playwright (`playwright.config.ts`, `e2e/app.spec.ts`).

### 3.3 `packages/ui`

- shadcn monorepo layout: `components.json` (base-nova, neutral, lucide), `src/components` (button, card), `src/lib/utils.ts`, `src/styles/globals.css`, `postcss.config.mjs`.
- `exports` map for `./globals.css`, `./components/*`, `./lib/*`, `./hooks/*`.

### 3.4 `packages/api-client`

- `openapi-typescript` output (`src/schema.d.ts`, committed) generated from `api/openapi.yaml`.
- `src/client.ts`: `createApiClient` (base URL + Bearer token middleware) and `unwrap`/`unwrapEnvelope` that turn the `{ data, error }` envelope into data or a typed `ApiError`.

### 3.5 Tooling workarounds

- `scripts/shadcn-path-fix.cjs` + `scripts/shadcn.cjs`: shadcn CLI 4.20/4.21 truncates resolved paths when the repo path contains a dot (`C:\Users\UF-Iman.Jofaris\...` → `C:\Users\UF-Iman`). The fix rewrites the CLI's extension-stripping regex to exclude backslashes; the wrapper injects it via `NODE_OPTIONS`.
- `opencode.json`: shadcn MCP server with the same `NODE_OPTIONS` fix.

---

## 4. Database changes

None.

---

## 5. API surface added

None. `packages/api-client` consumes `api/openapi.yaml` as-is.

---

## 6. Files created / modified

```text
package.json
pnpm-workspace.yaml
pnpm-lock.yaml
opencode.json
.prettierrc
.prettierignore
.dockerignore
.gitignore
.github/workflows/ci.yml
scripts/shadcn.cjs
scripts/shadcn-path-fix.cjs
apps/web/package.json
apps/web/components.json
apps/web/postcss.config.mjs
apps/web/tsconfig.json
apps/web/next.config.ts
apps/web/eslint.config.mjs
apps/web/playwright.config.ts
apps/web/vitest.config.ts
apps/web/vitest.setup.ts
apps/web/.env.example
apps/web/e2e/app.spec.ts
apps/web/src/app/layout.tsx
apps/web/src/app/page.tsx
apps/web/src/components/theme-provider.tsx
apps/web/src/components/api-status.tsx
apps/web/src/components/api-status.test.tsx
packages/ui/package.json
packages/ui/components.json
packages/ui/tsconfig.json
packages/ui/postcss.config.mjs
packages/ui/src/lib/utils.ts
packages/ui/src/styles/globals.css
packages/ui/src/components/button.tsx
packages/ui/src/components/card.tsx
packages/api-client/package.json
packages/api-client/tsconfig.json
packages/api-client/src/schema.d.ts
packages/api-client/src/client.ts
packages/api-client/src/client.test.ts
packages/api-client/src/index.ts
doc/frontend/Conventions.md
doc/phases/Phase F0 - Web Foundation.md
doc/phases/Phase F1 - Auth and Shell.md
doc/phases/Phase F2 - Events Dashboard.md
AGENTS.md
doc/Test Strategy.md
doc/Implementation Plan.md
```

---

## 7. Tests

### Unit

- `packages/api-client/src/client.test.ts`: envelope success, envelope error (code/status), transport error, non-OK without body, Bearer injection, no token.
- `apps/web/src/components/api-status.test.tsx`: online/offline/unconfigured rendering.

### E2E

- `apps/web/e2e/app.spec.ts`: app shell renders heading, API status badge, and primary button.

### Verification commands and results

```text
pnpm lint        -> eslint clean
pnpm typecheck   -> tsc clean (api-client, ui, web)
pnpm test        -> 9 passed
pnpm build       -> Next.js build succeeded, no type errors
pnpm test:e2e    -> 1 passed
pnpm gen:api     -> schema.d.ts regenerated, no unexpected diff
pnpm format:check -> all files formatted
```

---

## 8. Issues found and fixed

| Issue                                                                                             | Fix                                                                  |
| ------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| `corepack enable` fails with EPERM (Node under `C:\Program Files`)                                | Installed pnpm with `npm install -g pnpm`; documented in AGENTS.md   |
| shadcn CLI truncates paths with dots on Windows (`C:\Users\UF-Iman.Jofaris` → `C:\Users\UF-Iman`) | Added `scripts/shadcn-path-fix.cjs` + wrapper and MCP `NODE_OPTIONS` |
| Vitest 5 + Vite warning about ESM config loaded as CJS                                            | Added `"type": "module"` to `apps/web/package.json`                  |
| RTL rendered into the same DOM across tests (multiple matches)                                    | Added `afterEach(cleanup)` in `vitest.setup.ts`                      |
| Root `/` prerendered as static, so the runtime API check would freeze                             | Added `export const dynamic = "force-dynamic"`                       |
| `pnpm --filter` add commands did not install the UI package's deps                                | Ran a full `pnpm install` after writing `packages/ui/package.json`   |

---

## 9. Known limitations / follow-ups

- `packages/ui` has no ESLint script yet (typechecked only). Add a library ESLint config if the package grows.
- The shadcn path fix is a workaround for an upstream CLI bug; remove it once the CLI fixes the Windows regex.
- `pnpm test:e2e` requires a one-time `playwright install chromium`.
- No hosting decision was made; the app is platform-agnostic (server-only `API_BASE_URL`).
- Public gallery UI (F4) and upload UI (F3) are deferred; the API for both exists.

---

## 10. How to try it

```text
corepack enable            # or: npm install -g pnpm
pnpm install
pnpm gen:api

# terminal 1: Go API (use :18080 if WinNAT reserves :8080)
$env:HTTP_ADDR=":18080"; go run ./cmd/api

# terminal 2: frontend
# put API_BASE_URL=http://localhost:18080 in apps/web/.env.local
pnpm dev
# open http://localhost:3000

pnpm lint; pnpm typecheck; pnpm test; pnpm build
pnpm test:e2e
```
