# Migrate to tinywasm/orm v2 API (fmt.Field)

## Context

The ORM code generator (`ormc`) now produces `Schema() []fmt.Field` (from `tinywasm/fmt`) with individual bool constraint fields instead of the old `[]orm.Field` with bitmask constraints. The `Values()` method is removed; consumers use `fmt.ReadValues(schema, ptrs)` instead.

### Key API Changes

| Old (current) | New (target) |
|---|---|
| `[]orm.Field{...Constraints: orm.ConstraintPK}` | `[]fmt.Field{...PK: true}` |
| `orm.TypeText`, `orm.TypeInt64`, `orm.TypeBool` | `fmt.FieldText`, `fmt.FieldInt`, `fmt.FieldBool` |
| `m.Values() []any` | `fmt.ReadValues(m.Schema(), m.Pointers())` |
| `var BusinessHoursMeta = struct{...}` | `var BusinessHours_ = struct{...}` (standardized `_` suffix) |

### Models in scope

- `BusinessHours`

### Target fmt.Field Struct (`tinywasm/fmt`)

```go
type Field struct {
    Name    string
    Type    FieldType // FieldText, FieldInt, FieldFloat, FieldBool, FieldBlob, FieldStruct
    PK      bool
    Unique  bool
    NotNull bool
    AutoInc bool
    Input   string
    JSON    string
}
```

### Generated Code per Struct (`ormc`)

- `TableName() string`, `FormName() string`
- `Schema() []fmt.Field`, `Pointers() []any`
- `T_` metadata struct with typed column constants
- `ReadOneT(qb *orm.QB, model *T)`, `ReadAllT(qb *orm.QB)`

---

## Stage 1 — Regenerate ORM Code

**File**: `model_orm.go` (auto-generated)

1. Update `ormc`: `go install github.com/tinywasm/orm/cmd/ormc@latest`
2. Run `ormc` from project root
3. Verify `BusinessHours_` meta struct (not `BusinessHoursMeta`)

---

## Stage 2 — Update Handwritten Code

**File**: `mcp.go`

1. Replace `BusinessHoursMeta.DayOfWeek` → `BusinessHours_.DayOfWeek` (and all meta references)
2. Search for `.Values()` calls → replace with `fmt.ReadValues(m.Schema(), m.Pointers())`

> **Note**: `db.Query()`, `.OrderBy()`, `.Asc()`, `ReadAllBusinessHours()` — all unchanged.

---

## Stage 3 — Update go.mod

1. Run `go mod tidy`

---

## Verification

```bash
gotest
```

## Linked Documents

- [ARCHITECTURE.md](ARCHITECTURE.md)
- [SKILL.md](SKILL.md)
