---
name: ishos-feature-workflow
description: 'Phased implementation workflow for the IshosFactory Go backend. Use when adding a new feature, fixing a bug, extending an endpoint, adding a handler, service, or repository, or making any change to the layered Chi/Ent backend. Produces a structured Phase 1 (Understand) → Phase 2 (Implement) → Phase 3 (Verify) checklist before touching any code. Enforces minimal diffs, correct layer placement, and Go idioms specific to this project.'
argument-hint: Describe the feature or fix you need to implement
---

# IshosFactory Feature Workflow

## When to Use
- Adding a new API endpoint or HTTP handler
- Extending an existing handler (new field, new query param, new route action)
- Adding or modifying a service method (business logic)
- Adding or modifying a repository method (Ent ORM query/mutation)
- Fixing a bug anywhere in the `internal/` layers
- Adding middleware, a new schema entity, or a migration

---

## Procedure

### 0. Read the Prompt Fully

Before doing anything, confirm you understand:
- What outcome is expected (new endpoint? changed behavior? bug fix?)
- Which layer is affected (handler / service / repository / middleware / schema)
- Whether a migration is needed

If any requirement is unclear, list up to **5 clarifying questions** and stop until answered.

---

### Phase 1 — Understand

**Goal:** Build enough context to act confidently without over-exploring.

**Files to inspect (select relevant ones):**

| What you need | Where to look |
|---|---|
| Routing / endpoint registration | `internal/server/server.go` |
| HTTP contract for the domain | `docs/API.md` |
| Existing handler for the domain | `internal/handlers/<domain>.go` |
| Business logic | `internal/services/<domain>_service.go` |
| Data access | `internal/repository/<domain>_repo.go` |
| Ent schema entity | `ent/schema/<entity>.go` |
| Auth / middleware | `internal/middleware/` |
| Validation rules | `internal/validator/rules.go` |
| Response helpers | `pkg/response/response.go` |

**Tasks:**
- [ ] Identify which file(s) need to change.
- [ ] Confirm no duplicate logic already exists.
- [ ] Note the existing response struct shape (if extending an endpoint).
- [ ] Check whether the Ent schema or a migration is required.

---

### Phase 2 — Implement

**Goal:** Make the smallest possible changes that satisfy the requirement.

**Ordered tasks:**

1. **Schema / Migration** *(only if entity fields change)*
   - Edit `ent/schema/<entity>.go` — add field(s) only.
   - Run `go generate ./ent` to regenerate Ent code.
   - Add a new `.sql` file in `migrations/` following the naming pattern `000N_<description>.sql`.

2. **Repository** *(only if a new query or mutation is needed)*
   - Add the method to `internal/repository/<domain>_repo.go`.
   - Use Ent fluent API only — no raw SQL, no side effects.

3. **Service** *(only if business logic changes)*
   - Add or modify the method in `internal/services/<domain>_service.go`.
   - Call the repository method; orchestrate side effects (WhatsApp, tracking tokens, uploads) here.

4. **Validator** *(only if request payload changes)*
   - Add or extend rules in `internal/validator/rules.go`.

5. **Handler** *(always for new or changed HTTP endpoints)*
   - Add or modify the method in `internal/handlers/<domain>.go`.
   - Follow the existing pattern: decode → validate → call service → encode response.
   - Use `response.JSON` / `response.Error` from `pkg/response`.
   - Add the `// METHOD /path` doc comment above the function.

6. **Router** *(only if a new route is registered)*
   - Register the route in `internal/server/server.go` under the correct auth group.
   - Keep the route path consistent with `docs/API.md`.

7. **API Docs** *(update if endpoint contract changes)*
   - Update `docs/API.md` to reflect new/changed endpoints, request bodies, or response shapes.

**Per-change rules:**
- Make the **smallest possible diff** — do NOT rewrite surrounding code.
- Do NOT add docstrings, comments, or type annotations to code you didn't change.
- Do NOT add error handling for scenarios that cannot happen.
- Only delete code if it is clearly incorrect or obsolete.
- Add brief logical comments only at decision points that aren't obvious.

---

### Phase 3 — Verify

**Goal:** Confirm correctness before considering the task done.

**Steps:**

```sh
# 1. Compile — catch type errors immediately
go build ./...

# 2. Vet — catch common Go mistakes
go vet ./...

# 3. Run tests (if any exist for the changed domain)
go test ./tests/...
go test ./internal/...

# 4. Start the server locally
make dev
# or: go run ./cmd/api

# 5. Exercise the changed endpoint manually (Bruno collection or curl)
# Bruno collection is in ishosfactorytest/
```

**Checklist before marking done:**
- [ ] `go build ./...` passes with no errors.
- [ ] `go vet ./...` passes with no warnings.
- [ ] The new/changed route returns the expected status codes and response shape.
- [ ] Errors (not found, bad input, unauthorized) return the correct HTTP status + `{"error":"..."}` body via `response.Error`.
- [ ] No new linter errors (`make lint`).
- [ ] `docs/API.md` reflects any contract changes.

---

## Project Layer Rules (Quick Reference)

| Layer | Package | Rule |
|---|---|---|
| Transport | `internal/handlers` | Decode, validate, call service, encode. No DB, no business logic. |
| Business | `internal/services` | Orchestrate repo calls + side effects. No HTTP types. |
| Data | `internal/repository` | Ent queries/mutations only. No side effects, no HTTP types. |
| Middleware | `internal/middleware` | Cross-cutting concerns. Attach values to context; don't return business data. |
| Auth | `internal/auth` | PocketBase wrapper only. |

## Response Helpers

```go
response.JSON(w, http.StatusOK, payload)          // 200 with JSON body
response.Error(w, http.StatusBadRequest, "msg")   // error JSON: {"error":"msg"}
```

## Common Ent Patterns

```go
// Not-found check
if ent.IsNotFound(err) { response.Error(w, 404, "..."); return }

// UUID from URL param
id, err := uuid.Parse(chi.URLParam(r, "id"))

// Context-aware query
entity, err := h.repo.GetByID(r.Context(), id)
```

## Final Summary Template

After completing all phases, provide:

```
What was changed:
  - <file>: <one-line description>

Why it was changed:
  - <brief rationale tied to the requirement>

What to test next:
  - <specific endpoint or behavior to exercise>
```
