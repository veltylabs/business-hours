package businesshours

import (
	"webtyp.com/model"
	"webtyp.com/orm"
)

type lister struct {
	db *orm.DB
}

func (l lister) List() ([]model.Model, error) {
	rows, err := ReadAllBusinessHours(l.db.Query(&BusinessHours{}).OrderBy(BusinessHours_.DayOfWeek).Asc())
	if err != nil {
		return nil, err
	}
	out := make([]model.Model, len(rows))
	for i, r := range rows {
		out[i] = r
	}
	return out, nil
}
