# business-hours Architecture

## 1. Domain Scope
Manages the weekly operating schedule of the clinic. Provides a single source of truth for
which days the clinic is open and during which hours.

## 2. Core Entity
- **BusinessHours:** One row per day of the week (UNIQUE constraint on `day_of_week`).
  Stores open/close times, open flag, optional notes, and the timestamp of the last update.

## 3. Patterns
This module is coupled only to published `tinywasm/*` contracts, never to concrete
infrastructure — see `AGENTS.md` (this repo's root) for the full whitelist/blacklist:

- **`orm.DB` for storage** — backend-agnostic; wraps whatever `storage.Conn` the composition-root
  app injects (`storage/mem` in this module's own tests, a real SQL backend in production).
- **`ddl.CreateTable`** (over `db.RawConn()`, type-asserted against the optional `ddl.Compiler`
  capability) for the module's own schema migration in `New()` — one table, `business_hours`.
  Replaces the removed `orm.DB.CreateTable`.
- **`router.OpModule`** (`ModelName()` + `MountOps(reg router.OpRegistry)`) for transport — the
  module never sees `router.Router`/`router.APIModule`, and never imports `tinywasm/mcp`.
- **`model.IDGenerator`** for identity (`Deps.IDs`, required — the module never builds its own).
- **`events.Publisher`** (`Deps.Publisher`, optional) is present in `Deps`/`Module` for
  shape-parity with every other module in this ecosystem, but currently unused: this module has
  no mutation Op yet (see below), so nothing ever calls `Publish(...)`.
- **`view.Presenter`** (`NewView(caller router.Caller) view.Presenter`) for UI, built with only
  `view`+`model`+`router` — the app chooses the renderer. Read-only (no `WithSaveOp`/
  `WithDeleteOp`): there is no write Op to wire them to.

## 4. Ops (via `MountOps`)
| Op | Action | Resource | Description |
|----|--------|----------|--------------|
| `get_business_hours` | `r` (`model.Read`) | `business_hours` | Returns all 7 rows, sorted by `day_of_week` |

There is exactly one Op — read-only. `get_business_hours` returns the raw `BusinessHoursList`
(literal `day_of_week`/`open_time`/`close_time`/`is_open`/`notes` fields per row) through the
codec-agnostic transport (`model.Encodable`/`Decodable`) — it does not hand-shape a bespoke JSON
response. Human-readable Spanish day names (Domingo … Sábado) and hiding `open_time`/`close_time`
for a closed day are presentation concerns, applied in `NewView`'s item-mapping function (`view.go`),
not in the Op itself. There is no create/update/delete Op: nothing in this module writes a row
today; seeding/editing the 7 rows is the composition-root app's job (direct `orm.DB` access), or a
future write Op (a separate plan — see `AGENTS.md`'s domain-specific notes).

## 5. Composition Root Example
```go
hours, _ := businesshours.New(db, businesshours.Deps{
    IDs: idGenerator, // model.IDGenerator
})
hours.MountOps(opRegistry) // router.OpRegistry
view := hours.NewView(caller) // router.Caller -> view.Presenter
```

## 6. Schema
See [`docs/diagrams/database.md`](diagrams/database.md).