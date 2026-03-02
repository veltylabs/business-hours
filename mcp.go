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
