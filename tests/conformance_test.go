package tests

import (
	"testing"

	"webtyp.com/model"
	"webtyp.com/view"
	"webtyp.com/view/conformance"
	businesshours "github.com/veltylabs/business_hours"
)

func TestNewView_ListsWeek(t *testing.T) {
	caller := &conformance.FakeCaller{
		Reply: func(op string, into model.Decodable) {
			if op != businesshours.OpGetBusinessHours {
				return
			}
			dst := into.(*businesshours.BusinessHoursList)
			week := make(businesshours.BusinessHoursList, 7)
			for day := 0; day < 7; day++ {
				bh := &businesshours.BusinessHours{DayOfWeek: int64(day), IsOpen: day != 0 && day != 6}
				if bh.IsOpen {
					bh.OpenTime, bh.CloseTime = "08:00", "18:00"
				} else {
					bh.Notes = "Cerrado"
				}
				week[day] = bh
			}
			*dst = week
		},
	}

	p := businesshours.NewView(caller)
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
	// El Presenter no tiene CanSave()/CanDelete() — Saver/Deleter son capacidades que el renderizador
	// descubre mediante aserción de tipo (comentario de documentación de view.Presenter). En ausencia de WithSaveOp/WithDeleteOp,
	// view.New devuelve un núcleo básico que no implementa ninguno de los dos.
	if _, ok := p.(view.Saver); ok {
		t.Error("expected a read-only presenter: no SaveOp configured")
	}
	if _, ok := p.(view.Deleter); ok {
		t.Error("expected a read-only presenter: no DeleteOp configured")
	}
}
