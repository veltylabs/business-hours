//go:build !wasm

package businesshours

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tinywasm/context"
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
	tools := m.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "get_business_hours" {
		t.Errorf("expected tool name 'get_business_hours', got %q", tools[0].Name)
	}
}

func TestRegisterTools(t *testing.T) {
	m := setupTestModule(t)
	// New API: register by passing providers to mcp.NewServer
	cfg := mcp.Config{Name: "test", Version: "1.0.0"}
	_, _ = mcp.NewServer(cfg, []mcp.ToolProvider{m})
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

	res, err := m.GetBusinessHours(&context.Context{}, mcp.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Result: %+v", res)

	if res.IsError {
		t.Fatalf("expected success result, got error: %s", res.Content)
	}

	var content string = res.Content
	var envelope struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(res.Content), &envelope); err == nil && envelope.Type == "text" {
		content = envelope.Text
	}

	var schedResp scheduleResponse
	err = json.Unmarshal([]byte(content), &schedResp)
	if err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
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

	res, err := m.GetBusinessHours(&context.Context{}, mcp.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result, got success")
	}
	if !strings.Contains(res.Content, "schedule not found") {
		t.Errorf("expected 'schedule not found', got %q", res.Content)
	}
}

func TestGetBusinessHours_DBFailure(t *testing.T) {
	m := setupTestModule(t)

	// Drop table to simulate DB error
	err := m.db.DropTable(&BusinessHours{})
	if err != nil {
		t.Fatalf("failed to drop table: %v", err)
	}

	res, err := m.GetBusinessHours(&context.Context{}, mcp.Request{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result, got success")
	}
	if !strings.Contains(res.Content, "database unavailable") {
		t.Errorf("expected 'database unavailable', got %q", res.Content)
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
