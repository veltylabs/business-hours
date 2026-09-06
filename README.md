# business-hours
<img src="docs/img/badges.svg">

Weekly business operating hours module for the Velty ecosystem. This module provides a single source of truth for the clinic's operating schedule via MCP (Model Context Protocol).

## 🎯 Purpose
Manages the clinic's weekly operating schedule with one entry per day of the week (0–6). It expose the `get_business_hours` MCP tool as the primary read endpoint for LLMs and other consumers.

## 🛠 MCP Tools

| Tool | Parameters | Returns | Description |
|------|-----------|---------|-------------|
| `get_business_hours` | — | `schedule[]` | Returns the 7-day weekly schedule sorted by `day_of_week`. Includes Spanish day names and handles open/closed status. |

## 🚀 Quick Start

```go
import (
    "github.com/webtyp/mcp"
    businesshours "github.com/veltylabs/business-hours"
)

// 1. Initialize the module (handles DB schema migration automatically)
m, err := businesshours.New(db)
if err != nil {
    log.Fatal(err)
}

// 2. Register with an MCP Server
srv := mcp.NewServer(mcp.Config{
    Name:    "Clinic Tools",
    Version: "1.0.0",
}, []mcp.ToolProvider{m})
```

## 📂 Key Files & Structure

| File | Role |
|------|------|
| `model.go` | `BusinessHours` struct + `TableName()` |
| `model_orm.go` | Auto-generated ORM helpers — **DO NOT EDIT** |
| `mcp.go` | `Module` definition, `New(db)`, and `Tools()` provider |
| `mcp_test.go` | Unit and Integration tests (using `:memory:` SQLite) |

## 📝 Constraints & Details
- **Self-Migrating**: `New(db)` calls `db.CreateTable(&BusinessHours{})`.
- **Unique Schedule**: `day_of_week` has a UNIQUE constraint; only one row per day is allowed.
- **Dynamic Response**: `open` and `close` fields are omitted from the MCP response when `is_open = false`.
- **Localization**: Uses Spanish day names (Domingo, Lunes, Martes, Miércoles, Jueves, Viernes, Sábado).

## 📖 Documentation

- [ARCHITECTURE.md](docs/ARCHITECTURE.md) — Detailed domain scope, patterns, and MCP tool reference.
- [Database Diagram](docs/diagrams/database.md) — Visual schema representation.

---
*This module is part of the Velty Labs modules collection.*