//go:build !wasm

package businesshours

import (
    "context"
    "strings"
    "testing"

    "github.com/tinywasm/mcp"
    "github.com/tinywasm/sqlite"
)

func setupTestModule(t *testing.T) *Module {
    db, _ := sqlite.Open(":memory:")
    m, err := New(db)
    if err != nil {
        t.Fatalf("failed to create module: %v", err)
    }
    return m
}

func TestNew_CreatesTable(t *testing.T) {
    db, _ := sqlite.Open(":memory:")
    _, err := New(db)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }

    // Verify table was created by attempting to query it
    rows, err := ReadAllBusinessHours(db.Query(&BusinessHours{}))
    if err != nil {
        t.Fatalf("expected no error querying business_hours table, got %v", err)
    }
    if len(rows) != 0 {
        t.Errorf("expected 0 rows, got %d", len(rows))
    }
}

func TestNew_TableCreationError(t *testing.T) {
    // A quick way to make CreateTable fail is to pass a nil DB, but since New(db) does db.CreateTable, that would panic
    // Instead, let's just make it fail by closing the database first
    db, _ := sqlite.Open(":memory:")
    db.Close()

    _, err := New(db)
    if err == nil {
        t.Fatal("expected error when table creation fails")
    }
}

func TestGetMCPToolsMetadata(t *testing.T) {
    m := setupTestModule(t)
    tools := m.GetMCPToolsMetadata()
    if len(tools) != 1 {
        t.Fatalf("expected 1 tool, got %d", len(tools))
    }
    if tools[0].Name != "get_business_hours" {
        t.Errorf("expected tool name 'get_business_hours', got %q", tools[0].Name)
    }
}

func TestRegisterTools(t *testing.T) {
    m := setupTestModule(t)
    // We just ensure it doesn't panic. To fully test RegisterTools we'd need to mock mcp.MCPServer
    // but RegisterTools delegates to srv.RegisterProvider, so passing a real *mcp.MCPServer is enough to cover the line
    srv := mcp.NewMCPServer("test", "1.0.0")
    m.RegisterTools(srv)
}

func TestGetBusinessHours_FullWeek(t *testing.T) {
    m := setupTestModule(t)

    // Seed 7 rows
    for i := 0; i < 7; i++ {
        isOpen := true
        open := "08:00"
        close := "18:00"
        notes := ""
        if i == 0 || i == 6 {
            isOpen = false
            open = ""
            close = ""
            notes = "Cerrado"
        }

        bh := &BusinessHours{
            ID:        m.uid.GetNewID(),
            DayOfWeek: i,
            OpenTime:  open,
            CloseTime: close,
            IsOpen:    isOpen,
            Notes:     notes,
            UpdatedAt: 123456789,
        }
        err := m.db.Create(bh)
        if err != nil {
            t.Fatalf("failed to insert: %v", err)
        }
    }

    res, err := m.GetBusinessHours(context.Background(), nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    schedResp, ok := res.(scheduleResponse)
    if !ok {
        t.Fatalf("expected scheduleResponse, got %T", res)
    }

    if len(schedResp.Schedule) != 7 {
        t.Fatalf("expected 7 entries, got %d", len(schedResp.Schedule))
    }

    lunes := schedResp.Schedule[1]
    if !lunes.IsOpen || lunes.Open != "08:00" || lunes.Close != "18:00" {
        t.Errorf("Lunes incorrect: %+v", lunes)
    }

    domingo := schedResp.Schedule[0]
    if domingo.IsOpen || domingo.Notes == "" {
        t.Errorf("Domingo incorrect: %+v", domingo)
    }
}

func TestGetBusinessHours_Empty(t *testing.T) {
    m := setupTestModule(t)

    _, err := m.GetBusinessHours(context.Background(), nil)
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "schedule not found") {
        t.Errorf("expected 'schedule not found', got %q", err.Error())
    }
}

func TestGetBusinessHours_DBFailure(t *testing.T) {
    m := setupTestModule(t)

    // Drop table to simulate DB error
    err := m.db.DropTable(&BusinessHours{})
    if err != nil {
        t.Fatalf("failed to drop table: %v", err)
    }

    _, err = m.GetBusinessHours(context.Background(), nil)
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    if !strings.Contains(err.Error(), "database unavailable") {
        t.Errorf("expected 'database unavailable', got %q", err.Error())
    }
}

func TestBuildSchedule(t *testing.T) {
    rows := []*BusinessHours{
        {DayOfWeek: 0, IsOpen: false, Notes: "Cerrado"},
        {DayOfWeek: 1, IsOpen: true, OpenTime: "08:00", CloseTime: "18:00"},
    }
    resp := buildScheduleResponse(rows)

    if len(resp.Schedule) != 2 {
        t.Fatalf("expected 2, got %d", len(resp.Schedule))
    }

    if resp.Schedule[0].DayName != "Domingo" || resp.Schedule[0].IsOpen {
        t.Errorf("expected Domingo to be closed")
    }

    if resp.Schedule[1].DayName != "Lunes" || !resp.Schedule[1].IsOpen || resp.Schedule[1].Open != "08:00" {
        t.Errorf("expected Lunes to be open 08:00")
    }
}
