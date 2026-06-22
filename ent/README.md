# Ent ORM — Code Generation

⚠️ **DO NOT EDIT** generated files (unless you enjoy debugging 😅)

---

## What to Edit

✏️ **ONLY edit:** `ent/schema/*.go`

These are your entity definitions (Order, Product, Customer, etc.). Everything else gets regenerated.

---

## What Gets Generated (Hands Off!)

When you run `go generate ./ent`, Ent creates:

| Files | Purpose |
|-------|---------|
| `<entity>.go` | Entity struct definition |
| `<entity>_create.go` | Create builder |
| `<entity>_update.go` | Update builder |
| `<entity>_delete.go` | Delete builder |
| `<entity>_query.go` | Query builder |
| `<entity>/<entity>.go` | Predicates (for WHERE clauses) |
| `<entity>/predicate.go` | Predicate interface |
| `client.go`, `tx.go` | Connection management |
| `hook.go`, `mixin.go` | Lifecycle hooks (advanced) |

**Total:** ~10-12 files per entity, all auto-generated.

---

## Quick Start: 3 Steps

### 1. Add/Edit Schema

```bash
touch ent/schema/newentity.go
```

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"time"
)

type NewEntity struct {
	ent.Schema
}

func (NewEntity) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New).Immutable(),
		field.String("name").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (NewEntity) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name"),
		index.Fields("created_at"),
	}
}
```

### 2. Generate Code

```bash
go generate ./ent
```

### 3. Run Migration

```bash
make migrate
# or
go run ./cmd/api  # Auto-migrates on startup
```

---

## Working with Ent

### Create

```go
entity, err := client.NewEntity.Create().
	SetName("example").
	Save(ctx)
```

### Read

```go
// By ID
entity, err := client.NewEntity.Get(ctx, id)

// Query with filters
entities, err := client.NewEntity.Query().
	Where(newentity.NameEQ("example")).
	All(ctx)
```

### Update

```go
entity, err := client.NewEntity.Update().
	Where(newentity.IDEQ(id)).
	SetName("updated").
	Save(ctx)
```

### Delete

```go
err := client.NewEntity.DeleteOneID(ctx, id)
```

---

## Common Field Types

```go
field.String("name")                          // string
field.Int("count")                            // int
field.Float("price")                          // float64
field.Bool("is_active")                       // bool
field.Time("created_at")                      // time.Time
field.UUID("id", uuid.UUID{})                 // uuid.UUID
field.Bytes("data")                           // []byte
field.Enum("status").Values("pending", "done") // Typed enum
```

---

## Indexes

```go
// Single column
index.Fields("name")

// Multi-column
index.Fields("user_id", "created_at")

// Unique
index.Fields("email").Unique()

// Partial (PostgreSQL only)
index.Fields("status").Where(eq.StatusEQ("active"))
```

---

## Edges (Relationships)

```go
// One-to-many: Order has many Items
edge.To("items", OrderItem.Type)

// Many-to-one: OrderItem belongs to Order
edge.From("order", Order.Type).Ref("items")

// Many-to-many: Product has many Categories
edge.To("categories", Category.Type)
```

---

## VSCode File Nesting

Check `.vscode/settings.json` — this groups generated files under their source:
- `category.go` (source)
- ├─ `category_create.go`
- ├─ `category_update.go`
- ├─ `category/` (predicates)
- └─ ...

Keeps the explorer clean!

---

## After Schema Changes

Always:
1. Update `ent/schema/*.go`
2. Run `go generate ./ent`
3. Run `make migrate` (or `go run ./cmd/api`)
4. Commit generated files (they're deterministic)

---

## Troubleshooting

**"undefined reference to ent/newentity"**
```bash
go generate ./ent
```

**Database migration fails**
```bash
# Check your schema syntax
# Ensure PostgreSQL is running
# Check logs: go run ./cmd/api
```

**Type mismatch error**
- Ent generates `string` for `field.String()`
- Ent generates `*string` for `field.String().Optional()`
- Check the generated `<entity>.go` for actual types

---

**More help:** See [docs/DEVELOPER.md → Ent ORM Guide](../docs/DEVELOPER.md#working-with-ent-orm)
