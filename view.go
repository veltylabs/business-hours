package businesshours

import (
	"webtyp.com/orm"
	"webtyp.com/view"
)

var dayNames = [7]string{"Domingo", "Lunes", "Martes", "Miércoles", "Jueves", "Viernes", "Sábado"}

// Item implementa view.Itemizer — el ÚNICO código específico de la vista que lleva este registro. El Presenter
// indexa las filas por ID a partir de esto durante la recarga (Reload); ya no hay búsqueda manual por ID o WithFill.
func (r *BusinessHours) Item() view.Item {
	return view.Item{
		ID:          r.Id,
		Label:       dayNames[r.DayOfWeek],
		Description: scheduleLabel(r),
	}
}

// NewView construye el Presenter del horario semanal — el motor agnóstico de la tecnología que un renderizador (crudview o
// cualquier otro) envuelve. Es tarea de ESTE módulo construirlo (importando solo view+model+orm); la
// aplicación decide qué renderizador lo dibuja. Solo lectura: sin WithSaveOp/WithDeleteOp — no hay una
// operación de creación/actualización/eliminación todavía (ver AGENTS.md "Domain-specific notes").
func NewView(db *orm.DB) view.Presenter {
	return view.New(
		lister{db: db},
		&BusinessHours{},
		view.WithTitle("Horario de atención"),
	)
}

// scheduleLabel formatea las horas de una fila para su visualización — "08:00–18:00" cuando está abierto, "Cerrado" (más
// cualquier nota) cuando está cerrado. Solo para fines de presentación; la operación cruda nunca aplica este formato.
func scheduleLabel(r *BusinessHours) string {
	if !r.IsOpen {
		if r.Notes != "" {
			return r.Notes
		}
		return "Cerrado"
	}
	return r.OpenTime + "–" + r.CloseTime
}
