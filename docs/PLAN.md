---
PLAN: "feat: business_hours joins the reusable-module harness (OpModule, IDGenerator, ddl, storage/mem tests)"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

# PLAN — business_hours joins the reusable-module harness

You are an agent with **zero prior context** and **only this repository** (`business_hours`). This
plan is fully self-contained: every contract, rule, and code snippet you need is inline below. Read
`AGENTS.md` at this repo's root **first, in full** — it is the canonical whitelist/blacklist of
imports this plan applies, copied verbatim from
[`veltylabs/modules/AGENTS.md`](https://github.com/veltylabs/modules/blob/main/AGENTS.md) with this
module's own "Domain-specific notes" at the bottom. The reference implementation every module in
this ecosystem replicates is `github.com/veltylabs/item_catalog` — you do not have that repo
checked out, but every pattern you need from it is quoted inline here.

## 1. Why this plan exists

`business_hours` currently depends on concrete infrastructure the ecosystem is rectifying away from:
a concrete transport (`tinywasm/mcp`), a concrete ID generator (`tinywasm/unixid`), and a concrete
storage driver (`tinywasm/sqlite`, imported even in tests). The fix is to depend only on *ports*:
`tinywasm/model`, `tinywasm/router`, `tinywasm/view`, `tinywasm/events`, `tinywasm/orm`
(+transitively `tinywasm/storage`), and `tinywasm/ddl`. This is the same rectification already
applied as the reference pattern in `item_catalog`. Full ecosystem rationale (why `orm`/`storage`/
`ddl` are ports, not concrete deps): §2b of
[`REUSABLE_MODULES_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/REUSABLE_MODULES_MASTER_PLAN.md);
acceptance checklist: its §5.

**Do not use `docs/PLAN.md`'s previous content as a guide — this file replaces it entirely.** The
previous version of this file (a narrower "migrate `model.go` to `model.Definition`" plan) targeted
stale dependency versions (`model@v0.0.14`/`orm@v0.9.28`) and only covered Stage 1 below. This plan
supersedes it completely and retargets the current versions (§7).

## 2. Current state of this repo (read before touching anything)

- `model.go` — a plain Go struct with `db:"..."` tags:
  ```go
  //go:build !wasm

  package businesshours

  // BusinessHours represents a row in the business_hours table.
  type BusinessHours struct {
      ID        string `db:"pk"`               // set via unixid
      DayOfWeek int    `db:"unique,not_null"`  // 0=Sunday … 6=Saturday
      OpenTime  string `db:"not_null"`         // "HH:MM"
      CloseTime string `db:"not_null"`         // "HH:MM"
      IsOpen    bool   `db:"not_null"`
      Notes     string                         // nullable — empty string = no notes
      UpdatedAt int64  `db:"not_null"`
  }

  func (c *BusinessHours) TableName() string { return "business_hours" }
  ```
- `mcp.go` (repo root) — the *only* non-test file importing `tinywasm/mcp`/`tinywasm/unixid`. It
  defines `Module`, `New(db)` (calls the removed-API `db.CreateTable`, constructs a `unixid.UnixID`
  itself), the MCP tool `get_business_hours` via `mcp.ToolProvider`/`Tools()`, and hand-rolled JSON
  response shaping (`scheduleResponse`/`scheduleEntry`, Spanish day names, `encoding/json.Marshal`).
- `mcp_test.go` (repo root, package `businesshours`, **not** in `tests/`) — opens
  `tinywasm/sqlite` directly (`sqlite.Open(":memory:")`), uses stdlib `encoding/json` and `strings`.
- `go.mod` — requires `tinywasm/context v0.0.18`, `tinywasm/mcp v0.1.1`, `tinywasm/orm v0.6.0`,
  `tinywasm/sqlite v0.2.0`, `tinywasm/unixid v0.2.23`, `tinywasm/fmt v0.22.2` (all stale/forbidden
  except a much newer `orm`/`fmt`).
- `docs/ARCHITECTURE.md` describes the OLD pattern (module-owned `db.CreateTable`, MCP
  self-registration, `unixid` IDs) — rewritten by this plan's Stage 8.
- There is exactly **one** capability today: the read-only MCP tool `get_business_hours` (no
  params), returning all 7 rows sorted by `day_of_week`, enriched with Spanish day names, omitting
  `open`/`close` when the day is closed. There is **no** create/update/delete capability anywhere in
  this repo — nothing here ever writes a row today (seeding the 7 rows is the composition-root app's
  job). This plan does not add one; see "Out of scope" (§9).

## 3. The five-contract shape (inline reference — do not skip)

Every rectified module in this ecosystem takes this shape. Full detail lives in `AGENTS.md` at this
repo's root; the parts you need are quoted here.

```go
type Deps struct {
    IDs       model.IDGenerator // required — the module never builds its own
    Publisher events.Publisher  // optional — nil disables publishing silently
}

type Module struct {
    db  *orm.DB
    ids model.IDGenerator
    pub events.Publisher
}
```

- **Identity**: `Deps.IDs model.IDGenerator`, required. The module calls `m.ids.NewID()`; it never
  constructs a generator (`unixid.NewUnixID()` is forbidden in this module from now on).
- **Persistence**: `New(db *orm.DB, deps Deps)` owns its own schema migration via
  `github.com/tinywasm/ddl`. `orm.DB.CreateTable` no longer exists.
- **Transport**: the module implements `router.OpModule` (`ModelName() string` +
  `MountOps(reg router.OpRegistry)`), never `mcp.ToolProvider`/`Tools()`.
- **View**: `NewView(caller router.Caller) view.Presenter`, built with `view.New(...)` — only
  `view`+`model`+`router` imports.
- **Events**: `Deps.Publisher events.Publisher`, optional, `nil` disables publishing silently.
- For the full `model.Definition`/`Kind` contract (constructors, `FieldDB`, why `Field.Type` is
  never a bare enum literal), see `AGENTS.md` (this repo's root) and
  [`github.com/tinywasm/model`](https://github.com/tinywasm/model) — not repeated here; only this
  module's own concrete before/after code is shown below.

---

## Stage 1 — `model.go`: struct+tags → `model.Definition`

**File: `model.go`.** Replace the whole file:

**Before** (quoted in full in §2 above) — a plain struct with `db:"..."` tags.

**After:**

```go
package businesshours

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
)

// BusinessHoursModel: one row per day of the week. `day_of_week` is unique — see AGENTS.md
// "Domain-specific notes" for why this module never writes more than 7 rows.
// open_time/close_time carry a Permitted floor ("HH:MM": digits + ':' only, exactly 5 chars) —
// a free-text time column with the format documented only in a comment is the magic-string
// anti-pattern CONSTRUCTION_HARNESS forbids; the constraint belongs in the Definition.
var BusinessHoursModel = model.Definition{
	Name: "business_hours",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "day_of_week", Type: model.Int(), NotNull: true, DB: &model.FieldDB{Unique: true}},
		{Name: "open_time", Type: model.Text(), NotNull: true,
			Permitted: model.Permitted{Numbers: true, Extra: []rune{':'}, Minimum: 5, Maximum: 5}},
		{Name: "close_time", Type: model.Text(), NotNull: true,
			Permitted: model.Permitted{Numbers: true, Extra: []rune{':'}, Minimum: 5, Maximum: 5}},
		{Name: "is_open", Type: model.Bool(), NotNull: true},
		{Name: "notes", Type: model.Text()},
		{Name: "updated_at", Type: model.Int(), NotNull: true},
	},
}

var ErrNotFound = fmt.Err("schedule not found")
```

**No `GetBusinessHoursArgsModel`, no empty args struct.** The one Op takes no parameters and
`router.Route.Accepts(nil)` is the documented way to say so ("`nil` means 'no args'", `route.go`
in `tinywasm/router@v0.1.19`, still current) — do not invent an empty `Definition` + empty struct +
mandatory `{}` body just to have something to `Decode` into. See Stage 3.

**No hand-written `TableName()` either — the old method is deleted, not carried over.** Verified
against `model@v0.1.2`/`orm@v0.11.4` source: `Definition.Name` IS the model identity (`ormc`
generates `ModelName()` returning it, and `orm.DB` keys every operation off `ModelName()`); a
separate `TableName()` concept no longer exists in the ORM, so keeping the method would be dead
code that merely happens to agree with the generated one. (This supersedes any earlier draft note
that said to keep it.)

Notes on this rewrite:

- **Drop the `//go:build !wasm` tag entirely** — the old struct+tags file had one, but `model.go`
  now imports only `github.com/tinywasm/model`, which is isomorphic (compiles under
  `GOOS=js GOARCH=wasm`/TinyGo). This matches `item_catalog/model.go`, which carries no build tag.
  See also the note at the end of Stage 3 about `mcp.go`'s tag — the two are related.
- `TableName()` is **deleted** (see the note above): `Definition{Name: "business_hours"}` already
  carries the table identity via the generated `ModelName()`. After the rewrite,
  `grep -rn "TableName" .` must be empty.
- The import block already includes `"github.com/tinywasm/fmt"` for the `ErrNotFound` line (domain
  error, `fmt.Err(...)` — never stdlib `errors`).
- `DayOfWeek` is `int` today; `model.Int()`'s fixed mapping generates `int64`, so the regenerated
  struct's `DayOfWeek` field becomes `int64`. Check every comparison/arithmetic on `DayOfWeek`
  elsewhere in the repo after regenerating (there should be none left outside tests, and Stage 6
  rewrites the tests).
- `id` generates `Id` (pure casing, never `ID`) — check every consumer of `.ID` becomes `.Id`. The
  only place today is `mcp_test.go` (`bh.ID = ...`), rewritten wholesale in Stage 6.
- No `tinywasm/form/input` needed — this module has no UI-widget-bound field, base `model.Kind`
  constructors (`model.Text()`, `model.Int()`, `model.Bool()`) are enough.

**Regenerate `model_orm.go`:**

```bash
go install github.com/tinywasm/ormc/cmd/ormc@latest
ormc   # run from module root
```

This produces the concrete `BusinessHours` struct (`Id string`, `DayOfWeek int64`, `OpenTime
string`, `CloseTime string`, `IsOpen bool`, `Notes string`, `UpdatedAt int64`), `Schema()`/
`Pointers()`, `EncodeFields`/`DecodeFields`, `ModelName()`, `Validate(action byte) error`,
`ReadOneBusinessHours`/`ReadAllBusinessHours`, the `BusinessHours_` column-name struct, and (because
the model has a DB role) a `BusinessHoursList` type satisfying `model.ModelSlice` (`FielderSlice` +
`Decodable`) — used by Stage 3 and Stage 4. **Never hand-edit `model_orm.go`.**

---

## Stage 2 — identity: injected `model.IDGenerator`, never constructed locally

**File: `mcp.go`** (kept — see the file-naming note in Stage 3; do not rename it to `module.go`).

Today:

```go
import "github.com/tinywasm/unixid"

type Module struct {
	db  *orm.DB
	uid *unixid.UnixID
}

func New(db *orm.DB) (*Module, error) {
	if err := db.CreateTable(&BusinessHours{}); err != nil {
		return nil, err
	}
	u, err := unixid.NewUnixID()
	if err != nil {
		return nil, err
	}
	return &Module{db: db, uid: u}, nil
}
```

Replace with the standard `Deps`/`Module` shape from §3 (`events` is included for shape-parity with
every other module in this ecosystem — see the note at the end of this stage):

```go
type Deps struct {
	IDs       model.IDGenerator // required — the module never builds its own
	Publisher events.Publisher  // optional — nil disables publishing silently
}

type Module struct {
	db  *orm.DB
	ids model.IDGenerator
	pub events.Publisher
}
```

`New()`'s new body is written in Stage 5 (it also changes shape there, for the `ddl` migration —
don't write it twice). For this stage, the only required change is: `Deps.IDs` is validated as
required —

```go
if deps.IDs == nil {
	return nil, fmt.Err("business_hours: Deps.IDs is required")
}
```

— and `unixid.NewUnixID()` is deleted, full stop; nothing in this module constructs an ID generator
again. (Nothing in this repo currently calls `m.uid.NewID()` outside tests — there is no write path
yet, per §2 — so this stage is pure removal + the `Deps.IDs` contract, no call-site rewrite needed in
non-test code.)

Update `mcp.go`'s import block: remove `"github.com/tinywasm/unixid"`; add `"github.com/tinywasm/model"`
and `"github.com/tinywasm/events"` (needed for `model.IDGenerator`/`events.Publisher` above — `model`
is also needed later in this same file for Stage 3's `model.Read`, so add it once here).

**Note on `events.Publisher`:** this module has no mutation Op (§2, §9) and therefore never calls
`m.pub.Publish(...)` anywhere yet. It is still included in `Deps`/`Module` for shape-parity with
every other module (`AGENTS.md`'s "the shape every module takes" — `Deps{IDs, Publisher}` is uniform
across the ecosystem so a future write Op needs no `Deps`-signature change). It is legitimately
unused dead weight until a write Op exists — that is expected, not a bug to "fix" by removing it.

---

## Stage 3 — transport: `mcp.ToolProvider` → `router.OpModule`

**File: `mcp.go`.**

Today:

```go
import "github.com/tinywasm/mcp"

// Tools implements mcp.ToolProvider.
func (m *Module) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "get_business_hours",
			Description: "Returns the weekly operating schedule.",
			Resource:    "business_hours",
			Action:      'r',
			Execute:     m.GetBusinessHours,
		},
	}
}

// GetBusinessHours returns the weekly operating schedule.
func (m *Module) GetBusinessHours(ctx *context.Context, req mcp.Request) (*mcp.Result, error) {
	rows, err := ReadAllBusinessHours(m.db.Query(&BusinessHours{}).OrderBy(BusinessHours_.DayOfWeek).Asc())
	if err != nil {
		return &mcp.Result{IsError: true, Content: fmt.Err("database", "unavailable").Error()}, nil
	}
	if len(rows) == 0 {
		return &mcp.Result{IsError: true, Content: fmt.Err("schedule", "not", "found").Error()}, nil
	}
	b, err := json.Marshal(buildScheduleResponse(rows))
	if err != nil {
		return &mcp.Result{IsError: true, Content: err.Error()}, nil
	}
	return mcp.Text(string(b)), nil
}

var dayNames = [7]string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}

type scheduleEntry struct { /* ... */ }
type scheduleResponse struct { /* ... */ }
func buildScheduleResponse(rows []*BusinessHours) scheduleResponse { /* ... */ }
```

Replace the entire block above (`Tools()`, `GetBusinessHours`, `dayNames`, `scheduleEntry`,
`scheduleResponse`, `buildScheduleResponse`, and the `encoding/json` import) with:

```go
const OpGetBusinessHours = "get_business_hours"

func (m *Module) ModelName() string { return "business_hours" }

func (m *Module) MountOps(reg router.OpRegistry) {
	// This op takes no parameters — Accepts(nil) is the documented "no args" declaration
	// (router.Route's doc comment). Never invent an empty args struct for a no-args op.
	reg.Op(OpGetBusinessHours, m.opGetBusinessHours).
		Requires("business_hours", model.Read).
		Accepts(nil)
}

var _ router.OpModule = (*Module)(nil)

// GetSchedule returns all 7 rows sorted by day_of_week, or ErrNotFound if the table is empty
// (unseeded — the composition-root app is responsible for seeding the 7 rows; see AGENTS.md).
func (m *Module) GetSchedule() ([]BusinessHours, error) {
	rows, err := ReadAllBusinessHours(m.db.Query(&BusinessHours{}).OrderBy(BusinessHours_.DayOfWeek).Asc())
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	items := make([]BusinessHours, len(rows))
	for i, r := range rows {
		items[i] = *r
	}
	return items, nil
}

func (m *Module) opGetBusinessHours(ctx router.Context) {
	rows, err := m.GetSchedule()
	if err != nil {
		// Status convention (ecosystem-wide): 404 for not-found, 500 only for genuine internal
		// errors — never collapse both into 500 (the "runtime mystery" the harness forbids).
		if err == ErrNotFound {
			ctx.WriteStatus(404)
			return
		}
		ctx.WriteStatus(500)
		return
	}
	list := make(BusinessHoursList, len(rows))
	for i := range rows {
		list[i] = &rows[i]
	}
	if err := ctx.Encode(&list); err != nil {
		ctx.WriteStatus(500)
	}
}
```

Add `"github.com/tinywasm/router"` to the import block; drop `"github.com/tinywasm/context"`,
`"github.com/tinywasm/mcp"`, and `"encoding/json"` entirely — nothing in this file needs them
anymore.

**On the Spanish day names and "omit open/close when closed" formatting:** that logic does not move
into the Op handler. Under the new codec-agnostic transport (`AGENTS.md` → "Encoding"), an Op
encodes the typed `model.Encodable` record as-is (`BusinessHoursList` — 7 rows with their literal
`day_of_week`/`open_time`/`close_time`/`is_open` fields) — a module never hand-shapes a bespoke JSON
response inside an Op. Presentation formatting (a human-readable day name, hiding `open`/`close` for
a closed day) is a UI concern, and belongs in `NewView`'s item-mapping function instead (Stage 4),
where the ecosystem's `view.Presenter` pattern already expects exactly this kind of per-row label
formatting. This is a deliberate behavior change in *where* the formatting happens, not in what it
produces: any client of the raw Op sees plain field values; any client of the view sees the
formatted `Label`/`Description`.

**File-naming note:** keep the file named `mcp.go`, do not rename it to `module.go`/`ops.go`. This
matches `item_catalog`'s actual current file (`item_catalog/mcp.go`), which bundles `Module`, `New`,
service methods, `ModelName`, `MountOps`, and every Op handler in one file — despite no longer
importing `tinywasm/mcp` — precisely to avoid pure-rename churn once a historical name stops being
literally accurate. Do the same here.

**Build-tag note — drop `//go:build !wasm` from the top of `mcp.go`.** The original file (and
`item_catalog`'s current `mcp.go`) carries this tag, but by the end of Stage 5 this file imports only
`orm`, `router`, `model`, `events`, `ddl`, and `fmt` — every one isomorphic per `AGENTS.md`'s "Build
Tags Rule" — so the tag is no longer needed, and leaving it in place would actively break the wasm
build: `view.go` (Stage 4, untagged, so it always compiles) references the `OpGetBusinessHours`
constant declared in `mcp.go`; if `mcp.go` keeps `//go:build !wasm`, that constant would not exist
under `GOOS=js GOARCH=wasm`, and `view.go` would fail to compile there. Remove the tag now, in this
stage (`item_catalog@main`'s `mcp.go` carries no tag either — the reference already matches).

---

## Stage 4 — view: `NewView(caller router.Caller) view.Presenter`

**File: `view.go`** (new file, no build tag — `view`+`model`+`router` are all isomorphic).

A weekly schedule of 7 rows is a natural fit for a small read-only list view — build one:

**API note (`tinywasm/view@v0.1.12`, verified against the real published module — this plan
originally targeted the now-stale `v0.1.1`):** `view.New` dropped its 4th positional arg (the
`func(list model.FielderSlice) []view.Item` projector) and `view.WithFill`. `newList` is now
`func() model.ModelSlice` (not `FielderSlice`), and list-row projection is a method the record
itself implements — `view.Itemizer` (`Item() view.Item`). Selection lookup (formerly `WithFill`) is
now automatic: the `Presenter` builds its own id→record index from `Itemizer` during `Reload`.

```go
package businesshours

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

var dayNames = [7]string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}

// Item implements view.Itemizer — the ONLY view-specific code this record carries. The Presenter
// indexes rows by ID from this during Reload; there is no manual byID/WithFill lookup anymore.
func (r *BusinessHours) Item() view.Item {
	return view.Item{
		ID:          r.Id,
		Label:       dayNames[r.DayOfWeek],
		Description: scheduleLabel(r),
	}
}

// NewView builds the weekly-schedule Presenter — the tech-agnostic engine a renderer (crudview, or
// any other) wraps. It is THIS module's job to build it (importing only view+model+router); the
// app decides which renderer draws it. Read-only: no WithSaveOp/WithDeleteOp — there is no
// create/update/delete Op yet (see AGENTS.md "Domain-specific notes").
func NewView(caller router.Caller) view.Presenter {
	record := &BusinessHours{}

	return view.New(
		caller,
		record,
		OpGetBusinessHours,
		func() model.ModelSlice { return &BusinessHoursList{} },
		view.WithTitle("Horario de atención"),
	)
}

// scheduleLabel formats one row's hours for display — "08:00–18:00" when open, "Cerrado" (plus
// any notes) when closed. Presentation-only; the raw Op (Stage 3) never applies this formatting.
func scheduleLabel(r *BusinessHours) string {
	if !r.IsOpen {
		if r.Notes != "" {
			return r.Notes
		}
		return "Cerrado"
	}
	return r.OpenTime + "–" + r.CloseTime
}
```

(`scheduleLabel`'s `+` string concatenation is fine here — it isn't the stdlib `strings`/`strconv`
package the blacklist forbids, just the builtin `+` operator on two `string` values.)

---

## Stage 5 — persistence: `db.CreateTable` → `ddl.CreateTable` + type assertion

**File: `mcp.go`**, function `New`. Today (already partially rewritten by Stage 2 — this stage
finishes `New`'s body):

```go
func New(db *orm.DB) (*Module, error) {
	if err := db.CreateTable(&BusinessHours{}); err != nil {
		return nil, err
	}
	u, err := unixid.NewUnixID()
	if err != nil {
		return nil, err
	}
	return &Module{db: db, uid: u}, nil
}
```

Final form, combining Stage 2's `Deps` validation with the `ddl` migration:

```go
func New(db *orm.DB, deps Deps) (*Module, error) {
	if deps.IDs == nil {
		return nil, fmt.Err("business_hours: Deps.IDs is required")
	}
	// ddl.Compiler is an optional capability — only SQL backends (sqlt, postgres) implement it.
	// storage/mem (this module's own tests, Stage 6) creates tables lazily and needs no DDL at
	// all, so a type assertion, not an unconditional call, is how the module stays agnostic here.
	if ddlCompiler, ok := db.RawConn().(ddl.Compiler); ok {
		if err := ddl.New(db.RawConn(), ddlCompiler).CreateTable(&BusinessHours{}); err != nil {
			return nil, err
		}
	}
	return &Module{db: db, ids: deps.IDs, pub: deps.Publisher}, nil
}
```

Add `"github.com/tinywasm/ddl"` to the import block. `db.CreateTable` no longer exists on `*orm.DB`
at all — do not reach for it.

---

## Stage 6 — tests: move to `tests/`, drop every concrete backend

**Delete** `mcp_test.go` from the repo root — replace it with the files below under `tests/`
(package `tests`, external — exercises only the exported API, per `AGENTS.md`'s testing contract).
This mirrors `item_catalog`'s current `tests/` layout: a plain directory **inside the root module**.
**Do NOT create a `tests/go.mod`** — the earlier nested-module-with-`replace` pattern was removed
from `item_catalog` as a defect (`AGENTS.md`: a local-path `replace` is always a defect to close).
Test-only dependencies (`storage/mem`, `router/mock`, `view/conformance`) are resolved by one
`go mod tidy` at the module root (Stage 7).

**New file: `tests/business_hours_test.go`**, package `tests`. Every call below was verified against
the actual source of the pinned versions (§7) in the local module cache — not guessed. The
`TestNewView_ListsWeek` conformance-style test goes in its own file `tests/conformance_test.go`
(ecosystem convention: conformance suites live in a separate, same-named file — keeps test files
small and modular):

```go
package tests

import (
	"testing"

	businesshours "github.com/veltylabs/business_hours"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/storage/mem"
	"github.com/tinywasm/view/conformance"
)

type fakeIDs struct{ n int }

func (f *fakeIDs) NewID() string {
	f.n++
	return "test-id-" + fmt.Convert(f.n).String() // github.com/tinywasm/fmt — never stdlib strconv
}

// setup builds a Module over storage/mem and returns the SAME *orm.DB handle it was given —
// seedWeek below uses it directly (orm.DB.Create is exported; there is no need to reach into
// Module's unexported db field, and there is no write Op yet to seed through instead — see
// AGENTS.md's domain-specific notes on why this module's one Op is read-only).
func setup(t *testing.T) (*businesshours.Module, *orm.DB, *fakeIDs) {
	t.Helper()
	db := orm.New(mem.New())
	ids := &fakeIDs{}
	m, err := businesshours.New(db, businesshours.Deps{IDs: ids})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m, db, ids
}

// seedWeek inserts 7 rows through the same *orm.DB the test already holds — the composition-root
// app would seed the same way, since this module has no write Op (see AGENTS.md).
func seedWeek(t *testing.T, db *orm.DB, ids *fakeIDs) {
	t.Helper()
	for day := 0; day < 7; day++ {
		bh := &businesshours.BusinessHours{
			Id:        ids.NewID(),
			DayOfWeek: int64(day),
			OpenTime:  "08:00",
			CloseTime: "18:00",
			IsOpen:    day != 0 && day != 6,
			UpdatedAt: 123456789,
		}
		if !bh.IsOpen {
			bh.OpenTime, bh.CloseTime = "", ""
			bh.Notes = "Cerrado"
		}
		if err := db.Create(bh); err != nil {
			t.Fatalf("seed day %d: %v", day, err)
		}
	}
}

func TestGetSchedule_FullWeek(t *testing.T) {
	m, db, ids := setup(t)
	seedWeek(t, db, ids)

	rows, err := m.GetSchedule()
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("expected 7 rows, got %d", len(rows))
	}
	if !rows[1].IsOpen || rows[1].OpenTime != "08:00" || rows[1].CloseTime != "18:00" {
		t.Errorf("Monday incorrect: %+v", rows[1])
	}
	if rows[0].IsOpen || rows[0].Notes == "" {
		t.Errorf("Sunday incorrect: %+v", rows[0])
	}
}

func TestGetSchedule_Empty(t *testing.T) {
	m, _, _ := setup(t)
	_, err := m.GetSchedule()
	if err != businesshours.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMountOps_GetBusinessHours(t *testing.T) {
	m, db, ids := setup(t)
	seedWeek(t, db, ids)

	// mock.Router's zero value is legal (no Authn, no Authorize) but DENIES every guarded
	// route (model.Allowed(nil, ...) == false) — Configure an Authorize that allows, so the
	// success path is actually exercised, not accidentally passing via a 403 nobody checked.
	reg := &mock.Router{}
	reg.Configure(mock.Config{
		Authorize: func(userID string, r model.Resource, a model.Action) bool { return true },
	})
	m.MountOps(reg)

	routes := reg.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 registered op, got %d", len(routes))
	}
	info := routes[0]
	wantPath := "/" + businesshours.OpGetBusinessHours
	if info.Path != wantPath {
		t.Errorf("expected op path %q, got %q", wantPath, info.Path)
	}
	if info.Resource != "business_hours" || info.Action != model.Read {
		t.Errorf("expected Requires(business_hours, Read), got %q/%v", info.Resource, info.Action)
	}

	ctx := &mock.Context{}      // no body needed — the op declares Accepts(nil) and never decodes
	ctx.SetUserID("u1")         // AccessGuarded needs an identity before the gate runs
	reg.Invoke("OP", wantPath, ctx)

	if ctx.Status != 0 {
		t.Fatalf("expected no error status, got %d; body=%s", ctx.Status, ctx.ResponseBody())
	}
	if len(ctx.ResponseBody()) == 0 {
		t.Error("expected a non-empty encoded response body")
	}
}
```

**New file: `tests/conformance_test.go`**, package `tests`, its own full import block (do not guess
at "same as the previous file" — only these four packages are actually used here):

```go
package tests

import (
	"testing"

	businesshours "github.com/veltylabs/business_hours"
	"github.com/tinywasm/model"
	"github.com/tinywasm/view"
	"github.com/tinywasm/view/conformance"
)

func TestNewView_ListsWeek(t *testing.T) {
	caller := &conformance.FakeCaller{
		Reply: func(op string, into model.Decodable) {
			if op != businesshours.OpGetBusinessHours {
				return
			}
			dst := into.(*businesshours.BusinessHoursList)
			week := make(businesshours.BusinessHoursList, 7)
			for day := 0; day < 7; day++ {
				bh := &businesshours.BusinessHours{DayOfWeek: int64(day), IsOpen: day != 0 && day != 6}
				if bh.IsOpen {
					bh.OpenTime, bh.CloseTime = "08:00", "18:00"
				} else {
					bh.Notes = "Cerrado"
				}
				week[day] = bh
			}
			*dst = week
		},
	}

	p := businesshours.NewView(caller)
	if err := p.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	items := p.Items()
	if len(items) != 7 {
		t.Fatalf("expected 7 items, got %d", len(items))
	}
	if items[1].Label != "Lunes" {
		t.Errorf("expected Lunes label, got %q", items[1].Label)
	}
	// Presenter has no CanSave()/CanDelete() — Saver/Deleter are capabilities the renderer
	// discovers by type assertion (view.Presenter doc comment). Absent WithSaveOp/WithDeleteOp,
	// view.New returns a bare core that implements neither.
	if _, ok := p.(view.Saver); ok {
		t.Error("expected a read-only presenter: no SaveOp configured")
	}
	if _, ok := p.(view.Deleter); ok {
		t.Error("expected a read-only presenter: no DeleteOp configured")
	}
}
```

`m` and `db` in every test above **must** come from the same `setup(t)` call — `m` wraps that exact
`db` internally, so seeding through a second, unrelated `db` would leave `m`'s own store empty.

**Delete** entirely, do not port: `TestNew_TableCreationError` and `TestGetBusinessHours_DBFailure`
(both simulated a DB failure by closing/dropping a concrete `sqlite` connection — `storage/mem` has
no equivalent deterministic failure mode, and `AGENTS.md`'s testing checklist does not require one).
Also delete `TestBuildSchedule` (tested `buildScheduleResponse`, which Stage 3 removes entirely) and
`TestGetMCPToolsMetadata`/`TestRegisterTools` (tested `mcp.ToolProvider`, which no longer exists —
`TestMountOps_GetBusinessHours` above is its replacement).

Every test in `tests/` uses `errors.Is`/direct comparison against typed errors (`ErrNotFound`), never
substring matching on error text (`strings.Contains`) — the old tests matched on hand-rolled MCP
error strings that no longer exist; Op handlers now return a status code, not free text.

---

## Stage 7 — `go.mod`: end state

**Remove entirely** (must not appear, even indirectly — `AGENTS.md` blacklist, no exceptions for
tests):

```
github.com/tinywasm/context
github.com/tinywasm/mcp
github.com/tinywasm/sqlite
github.com/tinywasm/unixid
```

**Add as direct dependencies**, at the versions currently used by `item_catalog`'s `go.mod` — check
`https://github.com/veltylabs/item_catalog/blob/main/go.mod` for the current numbers before running
`go get`, since these move fast and a stale pin here is exactly the kind of drift this
rectification closes. As of this revision (re-verified by actually building this stage's code
against the real published modules, not guessed), `item_catalog`'s `go.mod` pins:

```
github.com/tinywasm/events v0.0.2
github.com/tinywasm/fmt    v0.25.5
github.com/tinywasm/model  v0.1.2
github.com/tinywasm/orm    v0.11.4
github.com/tinywasm/router v0.1.19
github.com/tinywasm/view   v0.1.12
```

Plus, made **direct** (not left `// indirect`) because `mcp.go` now imports it explicitly:

```
github.com/tinywasm/ddl v0.0.7
```

Run `go get <module>@<version>` for each line above (or `go get -u ./...` then `go mod tidy`), never
hand-edit version numbers into `go.mod` — a typo'd pseudo-version there does not resolve. Expect
`go mod tidy` to also add `github.com/tinywasm/form` and `github.com/tinywasm/json` as new
**indirect** entries automatically (pulled in transitively by `view/conformance`, which uses
`form/input` for its `MockRecord`, and by `router/mock`) — this is correct and expected; do not
remove them, and do not add either as a *direct* dependency (§7's earlier "do not add form" note
still holds — this module still has no form-widget `Kind`).

`github.com/tinywasm/storage` becomes a **direct** dependency too — `tests/business_hours_test.go`
imports `github.com/tinywasm/storage/mem` directly, and `tests/` belongs to this same module (no
nested `go.mod`, Stage 6). One `go mod tidy` at the module root after making the edits above — do
not hand-edit the indirect block; let `tidy` resolve it. Add `github.com/tinywasm/view` as a direct
require as well (`view.go`, Stage 4, plus `view/conformance` in tests).

Do not add `github.com/tinywasm/form` or `github.com/tinywasm/time` — this module uses no
form-widget `Kind`s and has no write path that would call `time.Now()` (see AGENTS.md's
domain-specific note on the one read-only Op).

---

## 8. `docs/ARCHITECTURE.md` — verify, don't skip

Once Stages 1–7 are done, `docs/ARCHITECTURE.md` must describe the resulting end state, not the old
`db.CreateTable`/MCP-self-registration pattern. Its "Domain Scope" and "Core Entity" sections stay as
they are (still accurate) — rewrite only "Architectural Patterns" (→ a "Patterns" list, `item_catalog`
style: `orm`+`ddl` / `router.OpModule` / `model.IDGenerator` / `view.Presenter`, noting
`events.Publisher` is present but currently unused) and "MCP Tools" (→ an "Ops" table with one row,
`get_business_hours` / `r` / `business_hours`). This has very likely already been done by the same
change that authored this plan — if you find `docs/ARCHITECTURE.md` still describing the old pattern
when you reach this stage, update it; do not leave it stale.

## 9. Out of scope — do not add

- **No create/update/delete Op.** This module has never had a write path (§2); adding one is a
  domain-behavior change, not a harness-adoption change, and belongs in a separate, future plan.
- **No `tenant_id`.** See `AGENTS.md`'s domain-specific notes — this module is deliberately not
  multi-tenant.
- **No renaming of `mcp.go`.** See the file-naming note at the end of Stage 3.
- **No change to the table name, column names, or the `day_of_week` UNIQUE constraint.**

## 10. Acceptance criteria

Run every check from the module root unless noted:

- `grep -rn "tinywasm/mcp\|tinywasm/unixid\|tinywasm/sqlite\|tinywasm/context" .` → empty (repo-wide,
  `tests/` included).
- `grep -rn "encoding/json" .` → empty.
- `grep -rn "\"strings\"\|\"strconv\"\|\"errors\"" .` → empty.
- `grep -rn "db.CreateTable\|db.DropTable" .` → empty.
- `grep -rn "unixid.NewUnixID\|mcp.ToolProvider\|mcp.Tool{" .` → empty.
- `grep -rn "go:build !wasm" model.go mcp.go view.go` → empty (none of the three carry the tag).
- `go build ./...` clean.
- `GOOS=js GOARCH=wasm go build ./...` clean (model.go/mcp.go/view.go should need no `!wasm` build
  tag at all once they only import isomorphic packages — per `AGENTS.md`'s "Build Tags Rule"; if you
  find yourself keeping `//go:build !wasm` anywhere, first check whether a concrete import snuck
  back in).
- `gotest ./...` green from the module root (covers `tests/` — same module; `find tests -name
  go.mod` → empty).
- `grep -rn "TableName" *.go` → empty (dead method deleted, Stage 1).
- `go.mod`: none of `tinywasm/context`, `tinywasm/mcp`, `tinywasm/sqlite`, `tinywasm/unixid` appear,
  directly or indirectly. `tinywasm/ddl` and `tinywasm/storage` are direct.
- `git status` clean after committing (no stray `mcp_test.go` left at the repo root once Stage 6
  deletes it).

## 11. Stages

| # | Stage | Output | Criterion |
|---|-------|--------|-----------|
| 1 | `model.go`: struct+tags → `model.Definition` | new `model.go`, regenerated `model_orm.go` | compiles; `Id`/`DayOfWeek int64` updated everywhere |
| 2 | Identity: `Deps.IDs model.IDGenerator` | `Deps`/`Module` shape in `mcp.go`; `unixid` gone | `grep unixid` empty |
| 3 | Transport: `router.OpModule` | `ModelName`/`MountOps`/`opGetBusinessHours` in `mcp.go`; `mcp.ToolProvider` gone | `grep tinywasm/mcp` empty; `var _ router.OpModule` compiles |
| 4 | View: `NewView` | new `view.go` | compiles; list has 7 items with day-name labels |
| 5 | Persistence: `ddl.CreateTable` + type assertion | `New()` rewritten in `mcp.go` | `grep db.CreateTable` empty |
| 6 | Tests: move to `tests/`, `storage/mem` only | `tests/business_hours_test.go` + `tests/conformance_test.go` (same module, NO `tests/go.mod`); root `mcp_test.go` deleted | `gotest ./...` green; `grep tinywasm/sqlite` empty |
| 7 | `go.mod` end state | deps added/removed per §7 | `go build ./...` clean; blacklisted deps absent even indirectly |
| 8 | `docs/ARCHITECTURE.md` verified/updated | Patterns + Ops sections match end state | matches what Stages 1–7 actually produced |
