---
PLAN: "refactor!: webtyp.com rename + adopt the new view.New(Lister, record, opts) API"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 15809867557506129029
PR: https://github.com/veltylabs/business_hours/pull/7
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan — `business_hours`: finish the WebTyp rename + new `view.New` API

The framework migrated `github.com/tinywasm/*` → `webtyp.com/*` and every
framework module is published under the new path. This module's imports were
already rewritten and `go.mod` pinned to the current published tags (staged).
The blocker is that **`webtyp.com/view`'s `view.New` signature changed**.

## The `view.New` API change

`go build ./...` currently fails in `view.go`:

```
./view.go:29:3: cannot use caller (variable of interface type router.Caller) as view.Lister value in argument to view.New: router.Caller does not implement view.Lister (missing method List)
./view.go:31:3: cannot use OpGetBusinessHours (untyped string constant "get_business_hours") as view.Option value in argument to view.New
./view.go:32:3: cannot use func() model.ModelSlice {…} (value of type func() model.ModelSlice) as view.Option value in argument to view.New
```

### Old signature (what `view.go` uses today)

```go
view.New(
	caller,                                              // router.Caller
	record,                                              // *BusinessHours
	OpGetBusinessHours,                                  // string op constant
	func() model.ModelSlice { return &BusinessHoursList{} },
	view.WithTitle("Horario de atención"),
)
```

### New signature (current `webtyp.com/view`)

```go
// view.New builds the presenter over a Lister. Mandatory collaborators are
// positional; a nil/empty mandatory value panics.
func New(l Lister, record model.Model, opts ...Option) Presenter

type Lister interface {
	List() ([]model.Model, error)
}
```

`router.Caller`, the op-name string, and the `func() model.ModelSlice` slice
factory are **gone**. The Presenter now derives its capabilities purely from
what `l` implements (`Saver`/`Updater`/`Deleter` — none here, this view is
read-only). `Option` is still `view.WithTitle` / `view.WithSearchPlaceholder`.

### Reference implementations in the framework (read these first)

- `webtyp.com/auth` → `auth/view.go`: `return view.New(b, &User{}, view.WithTitle("Usuarios"))`
  where `b` is the module's own backend value implementing `Lister`.
- `webtyp.com/layout/crudview` → `crudview/crud.go` for how a renderer wraps the
  Presenter that a module builds.

### Do

This module already has the listing primitive:
`model_orm.go`: `func ReadAllBusinessHours(qb *orm.QB) (BusinessHoursList, error)`.

1. Add a small **Lister adapter** in this package (new file `lister.go`) that
   holds whatever it needs to run `ReadAllBusinessHours` and satisfies
   `view.Lister`:
   ```go
   type lister struct{ qb *orm.QB }

   func (l lister) List() ([]model.Model, error) {
       rows, err := ReadAllBusinessHours(l.qb)
       if err != nil {
           return nil, err
       }
       out := make([]model.Model, len(rows))
       for i, r := range rows {
           out[i] = r
       }
       return out, nil
   }
   ```
   (Confirm `*BusinessHours` implements `model.Model`; `model_orm.go` already
   defines `Item()` / `Schema()` / `Pointers()` on it.)
2. Change `NewView`'s parameter from `caller router.Caller` to whatever the
   adapter needs — most likely `qb *orm.QB` (import `webtyp.com/orm`). Drop the
   `webtyp.com/router` import from `view.go` if nothing else there uses it.
   ```go
   func NewView(qb *orm.QB) view.Presenter {
       return view.New(lister{qb: qb}, &BusinessHours{}, view.WithTitle("Horario de atención"))
   }
   ```
3. Update every caller of `NewView` in this module (grep `NewView(`), and in
   `mcp.go` if it wires the view.
4. `OpGetBusinessHours` — if it is no longer referenced anywhere after this
   change (`grep -rn OpGetBusinessHours .`), delete the constant and its
   declaration. If `mcp.go` still needs an op name for the raw MCP tool, keep it
   there only.
5. `go mod tidy`.

## Verify the rename is complete

```
grep -rn 'github.com/tinywasm' --include='*.go' --include='go.mod' .   # must be empty
```

## Acceptance

- `go build ./...` → clean.
- `gotest ./...` → all green.
- `grep -rn 'github.com/tinywasm' --include='*.go' --include='go.mod' .` → empty.
- `grep -rn 'router.Caller' view.go` → empty (the view no longer depends on the
  router transport).
- No `replace … => ../…` in `go.mod` pointing outside the module.

## Constraints

- **No hardcoded strings** — op names, titles, and any repeated literal are
  named constants in this package.
- Read-only view: do **not** add `Saver`/`Updater`/`Deleter` methods to the
  adapter. `view.New` must return the plain read-only `Presenter`.
- `Item()` on `*BusinessHours` (in `view.go`) stays exactly as it is — the
  Presenter still projects rows through `view.Itemizer`.
- Keep this module importing only `view` + `model` + `orm` (+ `router` only if
  genuinely still needed). The app, not this module, picks the renderer.
