package businesshours

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/router"
	"github.com/tinywasm/view"
)

var dayNames = [7]string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}

// Item implements view.Itemizer — the ONLY view-specific code this record carries. The Presenter
// indexes rows by ID from this during Reload; there is no manual byID/WithFill lookup anymore.
func (r *BusinessHours) Item() view.Item {
	return view.Item{
		ID:          r.Id,
		Label:       dayNames[r.DayOfWeek],
		Description: scheduleLabel(r),
	}
}

// NewView builds the weekly-schedule Presenter — the tech-agnostic engine a renderer (crudview, or
// any other) wraps. It is THIS module's job to build it (importing only view+model+router); the
// app decides which renderer draws it. Read-only: no WithSaveOp/WithDeleteOp — there is no
// create/update/delete Op yet (see AGENTS.md "Domain-specific notes").
func NewView(caller router.Caller) view.Presenter {
	record := &BusinessHours{}

	return view.New(
		caller,
		record,
		OpGetBusinessHours,
		func() model.ModelSlice { return &BusinessHoursList{} },
		view.WithTitle("Horario de atención"),
	)
}

// scheduleLabel formats one row's hours for display — "08:00–18:00" when open, "Cerrado" (plus
// any notes) when closed. Presentation-only; the raw Op (Stage 3) never applies this formatting.
func scheduleLabel(r *BusinessHours) string {
	if !r.IsOpen {
		if r.Notes != "" {
			return r.Notes
		}
		return "Cerrado"
	}
	return r.OpenTime + "–" + r.CloseTime
}
