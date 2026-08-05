package tests

import (
	"testing"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/router/mock"
	"github.com/tinywasm/storage/mem"
	businesshours "github.com/veltylabs/business_hours"
)

type fakeIDs struct{ n int }

func (f *fakeIDs) NewID() string {
	f.n++
	return "test-id-" + fmt.Convert(f.n).String() // github.com/tinywasm/fmt — never stdlib strconv
}

// setup builds a Module over storage/mem and returns the SAME *orm.DB handle it was given —
// seedWeek below uses it directly (orm.DB.Create is exported; there is no need to reach into
// Module's unexported db field, and there is no write Op yet to seed through instead — see
// AGENTS.md's domain-specific notes on why this module's one Op is read-only).
func setup(t *testing.T) (*businesshours.Module, *orm.DB, *fakeIDs) {
	t.Helper()
	db := orm.New(mem.New())
	ids := &fakeIDs{}
	m, err := businesshours.New(db, businesshours.Deps{IDs: ids})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m, db, ids
}

// seedWeek inserts 7 rows through the same *orm.DB the test already holds — the composition-root
// app would seed the same way, since this module has no write Op (see AGENTS.md).
func seedWeek(t *testing.T, db *orm.DB, ids *fakeIDs) {
	t.Helper()
	for day := 0; day < 7; day++ {
		bh := &businesshours.BusinessHours{
			Id:        ids.NewID(),
			DayOfWeek: int64(day),
			OpenTime:  "08:00",
			CloseTime: "18:00",
			IsOpen:    day != 0 && day != 6,
			UpdatedAt: 123456789,
		}
		if !bh.IsOpen {
			bh.OpenTime, bh.CloseTime = "", ""
			bh.Notes = "Cerrado"
		}
		if err := db.Create(bh); err != nil {
			t.Fatalf("seed day %d: %v", day, err)
		}
	}
}

func TestGetSchedule_FullWeek(t *testing.T) {
	m, db, ids := setup(t)
	seedWeek(t, db, ids)

	rows, err := m.GetSchedule()
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("expected 7 rows, got %d", len(rows))
	}
	if !rows[1].IsOpen || rows[1].OpenTime != "08:00" || rows[1].CloseTime != "18:00" {
		t.Errorf("Monday incorrect: %+v", rows[1])
	}
	if rows[0].IsOpen || rows[0].Notes == "" {
		t.Errorf("Sunday incorrect: %+v", rows[0])
	}
}

func TestGetSchedule_Empty(t *testing.T) {
	m, _, _ := setup(t)
	_, err := m.GetSchedule()
	if err != businesshours.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMountOps_GetBusinessHours(t *testing.T) {
	m, db, ids := setup(t)
	seedWeek(t, db, ids)

	// mock.Router's zero value is legal (no Authn, no Authorize) but DENIES every guarded
	// route (model.Allowed(nil, ...) == false) — Configure an Authorize that allows, so the
	// success path is actually exercised, not accidentally passing via a 403 nobody checked.
	reg := &mock.Router{}
	reg.Configure(mock.Config{
		Authorize: func(userID string, r model.Resource, a model.Action) bool { return true },
	})
	m.MountOps(reg)

	routes := reg.Routes()
	if len(routes) != 1 {
		t.Fatalf("expected 1 registered op, got %d", len(routes))
	}
	info := routes[0]
	wantPath := "/" + businesshours.OpGetBusinessHours
	if info.Path != wantPath {
		t.Errorf("expected op path %q, got %q", wantPath, info.Path)
	}
	if info.Resource != "business_hours" || info.Action != model.Read {
		t.Errorf("expected Requires(business_hours, Read), got %q/%v", info.Resource, info.Action)
	}

	ctx := &mock.Context{}      // no body needed — the op declares Accepts(nil) and never decodes
	ctx.SetUserID("u1")         // AccessGuarded needs an identity before the gate runs
	reg.Invoke("OP", wantPath, ctx)

	if ctx.Status != 0 {
		t.Fatalf("expected no error status, got %d; body=%s", ctx.Status, ctx.ResponseBody())
	}
	if len(ctx.ResponseBody()) == 0 {
		t.Error("expected a non-empty encoded response body")
	}
}
