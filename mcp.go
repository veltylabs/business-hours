//go:build !wasm

package businesshours

import (
	"encoding/json"

	"github.com/tinywasm/context"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/mcp"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/unixid"
)

// Module holds the DB dependency for business_hours handlers.
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
