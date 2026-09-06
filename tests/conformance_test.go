package tests

import (
	"testing"

	"webtyp.com/orm"
	"webtyp.com/storage/mem"
	"webtyp.com/view"
	businesshours "github.com/veltylabs/business_hours"
)

func TestNewView_ListsWeek(t *testing.T) {
	db := orm.New(mem.New())
	ids := &fakeIDs{}
	_, err := businesshours.New(db, businesshours.Deps{IDs: ids})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	seedWeek(t, db, ids)

	p := businesshours.NewView(db)
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
	if _, ok := p.(view.Saver); ok {
		t.Error("expected a read-only presenter: no SaveOp configured")
	}
	if _, ok := p.(view.Deleter); ok {
		t.Error("expected a read-only presenter: no DeleteOp configured")
	}
}
