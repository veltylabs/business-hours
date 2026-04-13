# business-hours
<img src="docs/img/badges.svg">

Weekly business operating hours MCP module. Provides a single `get_business_hours` MCP tool
that returns the clinic's full weekly schedule.

## MCP Tools

| Tool | Parameters | Description |
|------|-----------|-------------|
| `get_business_hours` | — | Returns the 7-day weekly schedule sorted by `day_of_week`. |

## Quick Start

```go
import businesshours "github.com/veltylabs/business-hours"

m, err := businesshours.New(db)  // creates table + initialises module
m.RegisterTools(srv)             // registers MCP tools on *mcp.MCPServer
```

## Documentation

| Document | Description |
|----------|-------------|
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Domain scope, patterns, and MCP tool reference |
| [Database Diagram](docs/diagrams/database.md) | Schema diagram |
| [SKILL.md](docs/SKILL.md) | LLM-friendly condensed summary |