# PLAN — business_hours: migrar model.go a model.Definition

> This plan is dispatched via the CodeJob workflow. See skill: **agents-workflow**.

✅ **Desbloqueado.** `github.com/tinywasm/model@v0.0.14` (con `orm@v0.9.28`) ya lee `model.Definition`.
`go get github.com/tinywasm/model@v0.0.14 github.com/tinywasm/orm@v0.9.28` antes de regenerar.
⚠️ **Casing puro:** el campo `id` genera `Id` (no `ID`); actualiza referencias `.ID`→`.Id` en
consumidores (ver §5).

Eres un agente **sin contexto previo** y **solo tienes este repositorio** (`business_hours`). Plan
autocontenido: todo contrato, regla y ejemplo está inline.

---

## 1. Qué cambia y por qué

El ecosistema tinywasm invirtió la generación de modelos: en vez de struct Go + tags string, se
escribe una definición **tipada** (`model.Definition`) a mano, y `ormc` genera el struct concreto +
plomería. Migración **mecánica**: mismo comportamiento, misma tabla, mismas columnas.

## 2. Contrato de `github.com/tinywasm/model` (inline)

`Field.Type` **no** es un literal de un enum — es la interfaz `Kind`. Se rellena llamando a un
constructor (`model.Text()`, `model.Int()`, …), nunca asignando `model.FieldText` directamente:

```go
package model

// FieldType es el mapeo determinista de almacenamiento/wire — lo devuelve Kind.Storage(),
// nunca se asigna directamente a Field.Type.
type FieldType int
const (
    FieldText FieldType = iota // string
    FieldInt                   // int64
    FieldFloat                 // float64
    FieldBool                  // bool
    FieldBlob                  // []byte
    FieldStruct                // struct anidado — Kind = model.Struct(ref)
    FieldIntSlice               // []int
    FieldStructSlice            // []T anidado — Kind = model.StructSlice(ref)
    FieldRaw                    // JSON pre-serializado
)

// Kind reemplaza el antiguo par Field.Type-enum + Field.Widget. Implementaciones
// sin estado, seguras para reuso concurrente.
type Kind interface {
    Storage() FieldType          // mapeo determinista a Go/DDL
    Name() string                // nombre semántico: "text", "int", "email", ...
    Validate(value string) error // SIEMPRE presente — fail-closed
}

// Constructores base — devuelven Kind, no un literal FieldType:
func Text() Kind  // storage FieldText
func Int() Kind   // storage FieldInt
func Float() Kind // storage FieldFloat
func Bool() Kind  // storage FieldBool
func Blob() Kind  // storage FieldBlob

type FieldDB struct { PK, Unique, AutoInc bool }

type Field struct {
    Name      string
    Type      Kind        // model.Text(), model.Int(), ... — NUNCA un literal FieldType
    NotNull   bool
    OmitEmpty bool
    DB        *FieldDB    // nil = sin persistencia
    Ref       *Definition // solo FK escalar; en composición (Struct/StructSlice) el ref va
                          // en el constructor del Kind, no aquí
    Exclude   bool
    Permitted
}

type Fields = []Field

type Definition struct {
    Name   string
    Fields Fields
}
```

Mapeo fijo: `model.Text()`→`string`, `model.Int()`→`int64`, `model.Bool()`→`bool`. Variable de
definición debe llamarse `<Struct>Model`.

**Ya no existe `Field.Widget`.** Un Kind con UI (cuando un módulo lo necesite) es un `Kind` de
`github.com/tinywasm/form/input` (p. ej. `input.Text()`) — este módulo no usa widgets, así que le
bastan los Kinds base.

---

## 3. Estado actual (`model.go`, a portar)

```go
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
```

**Nota de migración de tipo:** `DayOfWeek` es hoy `int` (32-bit). El nuevo mapeo fijo hace que
`FieldInt` genere **`int64`**. `DayOfWeek` pasará de `int` a `int64` en el struct generado. Revisa
comparaciones/aritmética con `DayOfWeek` en el resto del código (`0 <= DayOfWeek <= 6`) — funcionan
igual con `int64`, pero cualquier literal o variable `int` comparada directamente puede necesitar
conversión explícita.

## 4. Estado objetivo (`model.go` reescrito)

```go
//go:build !wasm

package businesshours

import "github.com/tinywasm/model"

var BusinessHoursModel = model.Definition{
	Name: "business_hours",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "day_of_week", Type: model.Int(), NotNull: true, DB: &model.FieldDB{Unique: true}},
		{Name: "open_time", Type: model.Text(), NotNull: true},
		{Name: "close_time", Type: model.Text(), NotNull: true},
		{Name: "is_open", Type: model.Bool(), NotNull: true},
		{Name: "notes", Type: model.Text()},
		{Name: "updated_at", Type: model.Int(), NotNull: true},
	},
}

func (c *BusinessHours) TableName() string { return "business_hours" }
```

`TableName()` es un método **escrito a mano** (no lo genera `ormc`) — Go permite declarar métodos de
un tipo generado en otro archivo del mismo paquete. **Consérvalo tal cual**, junto a la `Definition`,
en el `model.go` reescrito:

```go
func (c *BusinessHours) TableName() string { return "business_hours" }
```

No hace falta tocar ningún caller de `TableName()`.

## 5. Pasos

> **Dependencias:** `go get github.com/tinywasm/model@v0.0.14 github.com/tinywasm/orm@v0.9.28`
> (dependencia directa nueva de `model`; antes solo se llegaba a él transitivamente vía `orm`).

1. Reescribe `model.go` con el contenido de §4 (incluye conservar `TableName()`), **sin directivas**.
2. Regenera `model_orm.go` con `ormc` (instalado/actual). El struct `BusinessHours` resultante: ⚠️ **`Id string`**
   (el campo `id` genera `Id` con casing puro, **no** `ID`), `DayOfWeek int64` (antes `int` — ver nota de
   §3), `OpenTime string`, `CloseTime string`, `IsOpen bool`, `Notes string`, `UpdatedAt int64`.
3. Ajusta: referencias `.ID`→`.Id` en consumidores, y cualquier comparación/aritmética con `DayOfWeek`
   que asuma `int` en vez de `int64`. Las claves JSON/columnas del wire no cambian.

## 6. Fuera de alcance

- No cambiar el nombre de tabla ni columnas.
- No añadir reglas de validación nuevas.

## 7. Criterio de aceptación

- `gotest ./...` verde con `go.mod` en `model v0.0.14` / `orm v0.9.28`.
- `model_orm.go` regenerado compila; campo `Id` (casing puro, antes `ID`) y `DayOfWeek int64`
  actualizados en todo el código consumidor (sin conversiones rotas).
- No queda struct plano con tags `db:` en `model.go`.

## 8. Etapas

| # | Etapa | Salida | Criterio |
|---|---|---|---|
| 1 | Reescribir `model.go` | Definition de §4 | compila (con ormc actualizado) |
| 2 | Regenerar + ajustar `DayOfWeek int64` | struct + plomería | mismos datos, tipo actualizado |
| 3 | Ajustar comparaciones `DayOfWeek` | callers actualizados si asumían `int` | `gotest ./...` verde |
