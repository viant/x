package model

// TypeParam represents a single type parameter with an optional constraint.
// Constraint is expressed as a Node; for identifiers like "any" or
// "comparable" it will be a Basic with that name; for interface or
// union constraints it will reflect the parsed structure best‑effort.
type TypeParam struct {
	Name       string
	Constraint Node // may be nil
}
