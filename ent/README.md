# Ent ORM — Generated Code

⚠️ **DO NOT EDIT** files in this directory except `schema/`.

## What to Edit

- ✏️ `schema/*.go` — Your entity definitions (Product, Order, etc.)

## What Gets Generated (Don't Touch)

| Pattern | Purpose | Count |
|---------|---------|-------|
| `<entity>.go` | Entity struct | 5 files |
| `<entity>_create.go` | Create builder | 5 files |
| `<entity>_update.go` | Update builder | 5 files |
| `<entity>_delete.go` | Delete builder | 5 files |
| `<entity>_query.go` | Query builder | 5 files |
| `<entity>/` | Predicates, constants | 5 dirs |
| `client.go`, `tx.go` | Shared utilities | 8 files |

**Total: ~43 generated files for 5 entities**

## Regenerate After Schema Changes

```bash
go generate ./ent
```

## VSCode File Nesting

See `.vscode/settings.json` — this groups `category_*.go` under `category.go`.
