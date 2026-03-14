# business-hours — LLM Skill Summary

## Purpose
Manages the clinic's weekly operating schedule. One row per day (0–6).
The `get_business_hours` MCP tool is the single read endpoint.

## Key Files
| File | Role |
|------|------|
| `model.go` | `BusinessHours` struct + `TableName()` |
| `model_orm.go` | Auto-generated ORM helpers — DO NOT EDIT |
| `mcp.go` | `Module`, `New(db)`, `GetMCPTools()`, `RegisterTools()`, `GetBusinessHours()`, `buildScheduleResponse()` |
| `mcp_test.go` | All tests (`!wasm` build tag, `:memory:` SQLite) |

## Constraints
- `New(db)` calls `db.CreateTable(&BusinessHours{})` — module owns migration.
- `day_of_week` has a UNIQUE constraint; expect one row per day.
- `open`/`close` fields omitted from response when `is_open = false`.
- Spanish day names: Domingo, Lunes, Martes, Miércoles, Jueves, Viernes, Sábado.

## MCP Registration
```go
m, _ := businesshours.New(db)
m.RegisterTools(srv) // srv is *mcp.MCPServer
```