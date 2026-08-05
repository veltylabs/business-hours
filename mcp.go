package businesshours

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/events"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router"
)

type Deps struct {
	IDs       model.IDGenerator // required — the module never builds its own
	Publisher events.Publisher  // optional — nil disables publishing silently
}

type Module struct {
	db  *orm.DB
	ids model.IDGenerator
	pub events.Publisher
}

func New(db *orm.DB, deps Deps) (*Module, error) {
	if deps.IDs == nil {
		return nil, fmt.Err("business_hours: Deps.IDs is required")
	}
	// ddl.Compiler is an optional capability — only SQL backends (sqlt, postgres) implement it.
	// storage/mem (this module's own tests, Stage 6) creates tables lazily and needs no DDL at
	// all, so a type assertion, not an unconditional call, is how the module stays agnostic here.
	if ddlCompiler, ok := db.RawConn().(ddl.Compiler); ok {
		if err := ddl.New(db.RawConn(), ddlCompiler).CreateTable(&BusinessHours{}); err != nil {
			return nil, err
		}
	}
	return &Module{db: db, ids: deps.IDs, pub: deps.Publisher}, nil
}

const OpGetBusinessHours = "get_business_hours"

func (m *Module) ModelName() string { return "business_hours" }

func (m *Module) MountOps(reg router.OpRegistry) {
	// This op takes no parameters — Accepts(nil) is the documented "no args" declaration
	// (router.Route's doc comment). Never invent an empty args struct for a no-args op.
	reg.Op(OpGetBusinessHours, m.opGetBusinessHours).
		Requires("business_hours", model.Read).
		Accepts(nil)
}

var _ router.OpModule = (*Module)(nil)

// GetSchedule returns all 7 rows sorted by day_of_week, or ErrNotFound if the table is empty
// (unseeded — the composition-root app is responsible for seeding the 7 rows; see AGENTS.md).
func (m *Module) GetSchedule() ([]BusinessHours, error) {
	rows, err := ReadAllBusinessHours(m.db.Query(&BusinessHours{}).OrderBy(BusinessHours_.DayOfWeek).Asc())
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	items := make([]BusinessHours, len(rows))
	for i, r := range rows {
		items[i] = *r
	}
	return items, nil
}

func (m *Module) opGetBusinessHours(ctx router.Context) {
	rows, err := m.GetSchedule()
	if err != nil {
		// Status convention (ecosystem-wide): 404 for not-found, 500 only for genuine internal
		// errors — never collapse both into 500 (the "runtime mystery" the harness forbids).
		if err == ErrNotFound {
			ctx.WriteStatus(404)
			return
		}
		ctx.WriteStatus(500)
		return
	}
	list := make(BusinessHoursList, len(rows))
	for i := range rows {
		list[i] = &rows[i]
	}
	if err := ctx.Encode(&list); err != nil {
		ctx.WriteStatus(500)
	}
}
