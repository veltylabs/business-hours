//go:build !wasm

package businesshours

// BusinessHours represents a row in the business_hours table.
type BusinessHours struct {
    ID        string `db:"pk"`               // set via unixid
    DayOfWeek int    `db:"unique,not_null"`  // 0=Sunday … 6=Saturday
    OpenTime  string `db:"not_null"`         // "HH:MM"
    CloseTime string `db:"not_null"`         // "HH:MM"
    IsOpen    bool   `db:"not_null"`
    Notes     string                         // nullable — empty string = no notes
    UpdatedAt int64  `db:"not_null"`
}

func (c *BusinessHours) TableName() string { return "business_hours" }
