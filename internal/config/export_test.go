package config

import "reflect"

// FiniteFloats exposes the finite walk so the schema sweep in the external
// test package can prove its own route machinery against a fixture type.
func FiniteFloats(val reflect.Value) error {
	return finiteFloats(val, "")
}
