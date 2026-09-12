# Phase N — <Name> — Completion Report

> This template is used for every completed phase. Copy it to
> `doc/phases/Phase N - Report.md` and fill it in. Keep it factual and
> reproducible: another engineer should be able to verify your claims.

**Status:** IN PROGRESS | COMPLETE | BLOCKED
**Completed:** <date>
**Author:** <name>

---

## 1. Summary

2–4 sentences. What did this phase deliver, and why does it matter? What is now possible that was not before?

---

## 2. Exit criteria — verification

Copy the Exit criteria from the phase plan and mark each. Include the actual evidence (command output, response bodies).

| Criteria | Result | Evidence |
|---|---|---|
| <criterion> | PASS/FAIL | <command or output> |

---

## 3. What was built

Describe each component, grouped logically. Include file paths.

### 3.1 <component>

- Responsibility
- Key files

### 3.2 <component>

...

---

## 4. Database changes

List migrations added and what they do.

```text
migrations/000X_<name>.sql
```

Note any indexes, constraints, or data considerations. State how to roll back.

---

## 5. API surface added

```http
METHOD /api/v1/...
```

Include a request/response example for the most important endpoint.

---

## 6. Files created / modified

```text
<path>
<path>
```

---

## 7. Tests

### Unit
- <what is covered>

### Integration (`//go:build integration`)
- <what is covered>

### E2E
- <the critical flow covered>

### Verification commands and results

```text
gofmt -l .                      -> <result>
go build ./...                  -> <result>
go vet ./...                    -> <result>
go test ./...                   -> <result>
go test -tags=integration ./... -> <result>
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| <issue> | <fix> |

---

## 9. Known limitations / follow-ups

- <limitation and which phase addresses it>

---

## 10. How to try it

Step-by-step commands a reviewer can run to see the new functionality working.

```text
<commands>
```
