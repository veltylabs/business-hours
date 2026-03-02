# business-hours — Implementation Plan.

> **Module:** `github.com/veltylabs/business-hours`
> **Package:** `businesshours`
> **Goal:** Implement the `GetBusinessHours` handler. Weekly operating hours per day of week. New table auto-created via ORM on startup.
>
> This module is a standalone Lego piece. It has no knowledge of which application will use it.

---

## Development Rules

- **SRP:** Every file has a single, well-defined purpose.
- **DI:** DB injected via `*orm.DB`. No global state.
- **Flat structure:** All files in repo root — no subdirectories.
- **Max 500 lines per file.**
- **Testing:** `gotest` (not `go test`). Mock all external interfaces. DDT.
- **ORM:** `tinywasm/orm` + `ormc` code generator. Run `ormc` from repo root.
- **Time:** Use `github.com/tinywasm/time`. NEVER use standard `time` package.
- **Errors:** `tinywasm/fmt` only — Noun+Adjective word order.

### Installation Prerequisites

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
go install github.com/tinywasm/orm/cmd/ormc@latest
```

---

## go.mod Dependencies

```bash
go get github.com/tinywasm/orm@latest
go get github.com/tinywasm/fmt@latest
go get github.com/tinywasm/unixid@latest
go get github.com/tinywasm/sqlite@latest
```

---

## Database Schema

```sql
CREATE TABLE business_hours (
    id          VARCHAR(255) PRIMARY KEY, -- string PK via unixid
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    -- 0 = Sunday, 1 = Monday, ..., 6 = Saturday
    open_time   TIME NOT NULL,       -- stored as string "HH:MM" in ORM
    close_time  TIME NOT NULL,       -- stored as string "HH:MM" in ORM
    is_open     BOOLEAN NOT NULL DEFAULT TRUE,
    notes       VARCHAR(255),
    updated_at  BIGINT NOT NULL      -- Unix Nano timestamp
);

CREATE UNIQUE INDEX uq_business_hours_day ON business_hours(day_of_week);
```

---

## Files to Create

| File | Action | Purpose |
|---|---|---|
| `model.go` | Create | `BusinessHours` struct with `db:` tags |
| `model_orm.go` | Generate | Run `ormc` — do NOT hand-write |
| `mcp.go` | Create | `Module` struct + exported handler method |
| `mcp_test.go` | Create | Black-box tool handler tests with mock DB |

---

## Step 1 — Model (`model.go`)

```go
//go:build !wasm

package businesshours

import "github.com/tinywasm/orm"

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

**Generate ORM code:**
```bash
# From repo root:
ormc
# Creates: model_orm.go
```

---

## Step 2 — Tool Output Contract

```json
{
  "schedule": [
    { "day": 0, "day_name": "Domingo",   "is_open": false, "notes": "Cerrado" },
    { "day": 1, "day_name": "Lunes",     "is_open": true,  "open": "08:00", "close": "18:00" },
    { "day": 6, "day_name": "Sábado",    "is_open": false, "notes": "Cerrado" }
  ]
}
```

| Day | Name |
|---|---|
| 0 | Domingo |
| 1 | Lunes |
| 2 | Martes |
| 3 | Miércoles |
| 4 | Jueves |
| 5 | Viernes |
| 6 | Sábado |

---

## Step 3 — Module (`mcp.go`)

```go
//go:build !wasm

package businesshours

import (
    "context"

    "github.com/tinywasm/fmt"
    "github.com/tinywasm/orm"
    "github.com/tinywasm/unixid"
)

// Module holds the DB dependency for business_hours handlers.
type Module struct {
    db  *orm.DB
    uid *unixid.UnixID
}

func New(db *orm.DB) (*Module, error) {
    u, err := unixid.NewUnixID()
    if err != nil {
        return nil, err
    }
    return &Module{db: db, uid: u}, nil
}

// GetBusinessHours returns the weekly operating schedule.
// Signature matches ToolHandler: func(context.Context, map[string]any) (any, error)
func (m *Module) GetBusinessHours(ctx context.Context, args map[string]any) (any, error) {
    rows, err := ReadAllBusinessHours(m.db.Query(&BusinessHours{}).OrderBy(BusinessHoursMeta.DayOfWeek).Asc())
    if err != nil {
        return nil, fmt.Err("database", "unavailable") // EN: Database Unavailable
    }
    if len(rows) == 0 {
        return nil, fmt.Err("schedule", "not", "found") // EN: Schedule Not Found
    }
    return buildScheduleResponse(rows), nil
}

var dayNames = [7]string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}

type scheduleEntry struct {
    Day     int    `json:"day"`
    DayName string `json:"day_name"`
    IsOpen  bool   `json:"is_open"`
    Open    string `json:"open,omitempty"`
    Close   string `json:"close,omitempty"`
    Notes   string `json:"notes,omitempty"`
}

type scheduleResponse struct {
    Schedule []scheduleEntry `json:"schedule"`
}

func buildScheduleResponse(rows []*BusinessHours) scheduleResponse {
    entries := make([]scheduleEntry, len(rows))
    for i, r := range rows {
        e := scheduleEntry{
            Day:     r.DayOfWeek,
            DayName: dayNames[r.DayOfWeek],
            IsOpen:  r.IsOpen,
            Notes:   r.Notes,
        }
        if r.IsOpen {
            e.Open = r.OpenTime
            e.Close = r.CloseTime
        }
        entries[i] = e
    }
    return scheduleResponse{Schedule: entries}
}
```

---

## Step 4 — Tests (`mcp_test.go`)

Integration tests using `github.com/tinywasm/sqlite` with an in-memory database.

```go
func setupTestModule(t *testing.T) *Module {
    db, _ := sqlite.Open(":memory:")
    db.CreateTable(&BusinessHours{})
    m, _ := New(db)
    return m
}
```

```
TestGetBusinessHours_FullWeek
  - Seed 7 rows (all days configured)
  - Assert response has 7 entries
  - Assert Lunes (day=1) has is_open=true, open="08:00", close="18:00"
  - Assert Domingo (day=0) has is_open=false, notes non-empty

TestGetBusinessHours_Empty
  - Start with empty table
  - Assert handler returns "schedule not found"

TestGetBusinessHours_DBFailure
  - Simulate DB error (e.g. drop table before query)
  - Assert error contains "database unavailable"
```

```bash
gotest -run TestGetBusinessHours
gotest -run TestBuildSchedule
```

---

## Checklist

- [ ] `model.go` — `BusinessHours` struct with correct `db:` tags; `TableName()` returns `"business_hours"`
- [ ] `ormc` run from repo root — `model_orm.go` generated (contains `ReadAllBusinessHours`, `BusinessHoursMeta`)
- [ ] `mcp.go` — NO import of `mjosefa-cms` or any application package
- [ ] `mcp.go` — `GetBusinessHours` is exported and matches signature `func(context.Context, map[string]any) (any, error)`
- [ ] `open`/`close` fields are OMITTED (omitempty) when `is_open=false`
- [ ] All 4 test cases pass: `gotest -run TestGetBusinessHours`
- [ ] `go build ./...` succeeds
- [ ] `gopush 'implement GetBusinessHours handler'`
