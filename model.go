package businesshours

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// BusinessHoursModel: una fila por día de la semana. `day_of_week` es único — ver AGENTS.md
// "Domain-specific notes" para saber por qué este módulo nunca escribe más de 7 filas.
// open_time/close_time tienen un límite permitido ("HH:MM": solo dígitos + ':', exactamente 5 caracteres) —
// una columna de tiempo de texto libre con el formato documentado solo en un comentario es el patrón
// antipatrón de cadena mágica que CONSTRUCTION_HARNESS prohíbe; la restricción pertenece a la definición.
var BusinessHoursModel = model.Definition{
	Name: "business_hours",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "day_of_week", Type: model.Int(), NotNull: true, DB: &model.FieldDB{Unique: true}},
		{Name: "open_time", Type: model.Text(), NotNull: true,
			Permitted: model.Permitted{Numbers: true, Extra: []rune{':'}, Minimum: 5, Maximum: 5}},
		{Name: "close_time", Type: model.Text(), NotNull: true,
			Permitted: model.Permitted{Numbers: true, Extra: []rune{':'}, Minimum: 5, Maximum: 5}},
		{Name: "is_open", Type: model.Bool(), NotNull: true},
		{Name: "notes", Type: model.Text()},
		{Name: "updated_at", Type: model.Int(), NotNull: true},
	},
}

var ErrNotFound = fmt.Err("schedule not found")
