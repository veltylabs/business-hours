# business-hours — Enhancement Plan (ToolProvider Self-Registration)

> **Goal:** Implement `mcp.ToolProvider` so the module self-registers its MCP tools
> via `srv.RegisterProvider(m)`. Move schema migration into `New(db)`.
>
> **Depends on:** `tinywasm/mcp` `RegisterProvider` + fixed `ToolExecutor` (see `tinywasm/mcp/docs/PLAN.md`).
> **Status:** Pending execution

---

## Development Rules

- **Testing Runner:** `go install github.com/tinywasm/devflow/cmd/gotest@latest`
- **Build Tag:** All backend files must use `//go:build !wasm`.
- **No log injection:** The module receives only `db *orm.DB`. No log parameter in `New()`.

---

## Step 1 — Move Schema Migration into `New(db)`

**Target File:** `mcp.go`

`businesshours.New(db)` must call `db.CreateTable(&BusinessHours{})` before returning.
The CMS no longer owns migration — the module does.

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

---

## Step 2 — Implement `ToolProvider`

**Target File:** `mcp.go`

Add `GetMCPToolsMetadata()` to make `*Module` implement `mcp.ToolProvider`.
The `Execute` field points directly to the existing handler method — no adapter needed.

```go
func (m *Module) GetMCPToolsMetadata() []mcp.ToolMetadata {
    return []mcp.ToolMetadata{
        {
            Name:        "get_business_hours",
            Description: "Returns the weekly operating schedule.",
            Execute:     m.GetBusinessHours,
            // No parameters — this tool takes no arguments.
        },
    }
}
```

---

## Step 3 — Add `RegisterTools`

**Target File:** `mcp.go`

```go
// RegisterTools registers all business-hours MCP tools on the given server.
// Call once during application startup after New(db).
func (m *Module) RegisterTools(srv *mcp.MCPServer) {
    srv.RegisterProvider(m)
}
```

---

## Step 4 — Update Tests

- Update `mcp_test.go` to verify `GetMCPToolsMetadata()` returns the expected tool names.
- Add a test that `New(db)` creates the `business_hours` table in a test DB.
- Run `gotest` — 100% pass required.

---

## Step 5 — Verify & Submit

1. Run `gotest` from project root.
2. Run `gopush 'feat: ToolProvider self-registration, migrate schema in New()'`
