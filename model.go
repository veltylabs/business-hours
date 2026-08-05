package businesshours

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
)

// BusinessHoursModel: one row per day of the week. `day_of_week` is unique — see AGENTS.md
// "Domain-specific notes" for why this module never writes more than 7 rows.
// open_time/close_time carry a Permitted floor ("HH:MM": digits + ':' only, exactly 5 chars) —
// a free-text time column with the format documented only in a comment is the magic-string
// anti-pattern CONSTRUCTION_HARNESS forbids; the constraint belongs in the Definition.
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
