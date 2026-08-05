---
PLAN: "test: business_hours raise coverage to >=80% with real-value tests only"
TAG: v0.1.1
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

# PLAN — business_hours: subir cobertura a >=80% (solo pruebas de valor real)

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`business_hours`). El
módulo ya adoptó el patrón del arnés reutilizable (`router.OpModule`, `ddl`, `storage/mem`) en una
ronda anterior; este plan es un ajuste pequeño y autocontenido sobre ese trabajo ya mergeado.

## 1. Por qué existe este plan

La cobertura actual es 55.6% (medida con `go test ./tests/... -coverpkg=github.com/veltylabs/business_hours
-cover`), por debajo del objetivo del ecosistema (>=80%). **No se trata de perseguir el número —
varios de los caminos sin cubrir son plumbing generado por `ormc` que ningún camino real de
producción invoca jamás; forzar cobertura ahí sería relleno, no valor.** Este plan agrega
exactamente 3 pruebas nuevas, cada una demostrando un comportamiento real que hoy nadie verifica, y
dice explícitamente cuáles huecos se dejan sin cubrir a propósito.

## 2. Analizado y descartado — no agregar pruebas para esto

- `BusinessHoursList.EncodeFields`/`DecodeFields` (los stubs vacíos `{}` generados para el tipo
  lista) — el codec real serializa listas vía `Len()`/`At()`/`Append()`, nunca llama a estos dos
  métodos directamente. Están en 0% de cobertura de forma estructural, no por falta de una prueba.
- `BusinessHours.Validate` — el generado por `ormc` para el registro individual. Este módulo es
  **solo lectura** (un único Op, `get_business_hours`); ningún camino de producción llama jamás a
  `Validate` porque nunca hay un `Create`/`Update` a través de este módulo (sembrar las 7 filas es
  trabajo de la app de composición raíz — ver `AGENTS.md`). Escribir una prueba que llame a
  `Validate` directamente probaría el comportamiento de `ormc`, no el de este módulo.
- `ReadOneBusinessHours` — generado automáticamente para todo modelo con rol de DB, pero este módulo
  nunca lo usa (solo `ReadAllBusinessHours`, ordenado). Cubrirlo sería probar código muerto desde la
  perspectiva de este módulo.

Si una ronda de revisión futura decide que estos SÍ deben cubrirse, es una decisión de alcance
distinta — no la implementes especulativamente aquí.

## 3. Las 3 pruebas a agregar — todas en `tests/business_hours_test.go` (archivo existente)

### 3.1 — `TestNew_RequiresIDs` (cubre la rama de validación de `New`, hoy sin probar)

Insertar **antes** de `TestGetSchedule_Empty`:

```go
func TestNew_RequiresIDs(t *testing.T) {
	db := orm.New(mem.New())
	if _, err := businesshours.New(db, businesshours.Deps{}); err == nil {
		t.Fatal("expected an error when Deps.IDs is nil")
	}
}
```

### 3.2 — Extender `TestMountOps_GetBusinessHours` con una aserción de `ModelName` y un roundtrip real de decodificación

**Al inicio de la función**, justo después de `seedWeek(t, db, ids)`, agregar:

```go
	if m.ModelName() != "business_hours" {
		t.Fatalf("expected ModelName %q, got %q", "business_hours", m.ModelName())
	}
```

**Al final de la función**, justo después del bloque que verifica `len(ctx.ResponseBody()) == 0`,
agregar (esto exige importar `"github.com/tinywasm/json"` — ver §4):

```go
	// Decodifica la respuesta a través del codec real (no solo revisa que el body no esté vacío) —
	// prueba que el contrato de wire realmente funciona de punta a punta, y ejercita
	// BusinessHoursList.Append/DecodeFields, que ningún otro test recorre hoy.
	var got businesshours.BusinessHoursList
	if err := json.Decode(ctx.ResponseBody(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 7 {
		t.Fatalf("expected 7 decoded rows, got %d", len(got))
	}
	if !got[1].IsOpen || got[1].OpenTime != "08:00" {
		t.Errorf("Monday decoded incorrectly: %+v", got[1])
	}
```

`"github.com/tinywasm/json"` en un archivo de test es la excepción documentada en `AGENTS.md` ("in
tests, use `github.com/tinywasm/json` for codec verification") — no es una violación del blacklist.

### 3.3 — `TestMountOps_GetBusinessHours_Empty` (cubre la rama 404 del Op handler, hoy solo probada a nivel de servicio, nunca a través del Op)

Agregar como función nueva, después de `TestMountOps_GetBusinessHours`:

```go
func TestMountOps_GetBusinessHours_Empty(t *testing.T) {
	m, _, _ := setup(t) // sin seedWeek — la tabla queda vacía

	reg := &mock.Router{}
	reg.Configure(mock.Config{
		Authorize: func(userID string, r model.Resource, a model.Action) bool { return true },
	})
	m.MountOps(reg)

	ctx := &mock.Context{}
	ctx.SetUserID("u1")
	reg.Invoke("OP", "/"+businesshours.OpGetBusinessHours, ctx)

	if ctx.Status != 404 {
		t.Fatalf("expected 404 for an empty schedule, got %d", ctx.Status)
	}
}
```

## 4. Import a agregar

En `tests/business_hours_test.go`, agrega `"github.com/tinywasm/json"` al bloque de imports
existente (junto a `fmt`, `model`, `orm`, `router/mock`, `storage/mem`). Corre `go mod tidy` después
— `tinywasm/json` ya está resuelto transitivamente en el grafo de dependencias, pero pasa a ser
directo al importarlo explícitamente en un archivo de test.

## 5. Fuera de alcance

- No tocar `mcp.go`, `model.go`, `view.go`, `model_orm.go` — este plan es solo de pruebas.
- No agregar pruebas para los tres puntos descartados en §2.
- No perseguir 100% — 82.7% es el resultado esperado de aplicar exactamente §3; suficiente para
  cruzar el umbral de >=80% sin relleno.

## 6. Criterio de aceptación

- `go build ./...` y `GOOS=js GOARCH=wasm go build ./...` limpios.
- `gotest ./...` verde.
- `go test ./tests/... -coverpkg=github.com/veltylabs/business_hours -cover` reporta >=80%.
- `git status` limpio tras el commit.
