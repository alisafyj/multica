package designdocument

import (
	"fmt"
	"reflect"
	"strings"
)

// SchemaOutline renders the exact JSON shape a package file is decoded into.
//
// brief.json and coverage.json are required, are decoded with
// DisallowUnknownFields, and — until this existed — were described to the
// agent only in prose: "the semantic layer", "requirement coverage and honest
// gaps". An agent cannot infer `requirement_coverage` from "coverage", so every
// run invented its own field names and every finished package was rejected at
// the gate. critique.json, which is OPTIONAL, was the only one whose schema the
// prompt actually carried.
//
// The outline is generated from the structs rather than written beside them, so
// renaming a field cannot leave the instructions describing a shape the parser
// no longer accepts. Prose still says what belongs in each field; this says
// what each field is called.
func SchemaOutline(value any) string {
	return outlineType(reflect.TypeOf(value), 0)
}

// outlineType renders one type. Structs at the top level go one field per line;
// nested structs collapse inline, which keeps a page or a flow readable as a
// single shape instead of a tree the reader has to reassemble.
func outlineType(t reflect.Type, depth int) string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		fields := structFields(t, depth)
		if depth == 0 {
			return "{\n  " + strings.Join(fields, ",\n  ") + "\n}"
		}
		return "{" + strings.Join(fields, ", ") + "}"
	case reflect.Slice, reflect.Array:
		return "[" + outlineType(t.Elem(), depth+1) + "]"
	case reflect.Map:
		return fmt.Sprintf("{%s: %s}", outlineType(t.Key(), depth+1), outlineType(t.Elem(), depth+1))
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Float32, reflect.Float64:
		return "number"
	default:
		if isIntegerKind(t.Kind()) {
			return "integer"
		}
		return t.Kind().String()
	}
}

func structFields(t reflect.Type, depth int) []string {
	fields := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		name, optional, ok := jsonFieldName(field)
		if !ok {
			continue
		}
		// An optional field is marked rather than omitted: the agent has to be
		// able to tell "you may leave this out" from "this name is not accepted".
		if optional {
			name += "?"
		}
		fields = append(fields, name+": "+outlineType(field.Type, depth+1))
	}
	return fields
}

// jsonFieldName reports the wire name of a struct field, whether it may be
// omitted, and whether it is serialised at all.
func jsonFieldName(field reflect.StructField) (name string, optional bool, ok bool) {
	tag, tagged := field.Tag.Lookup("json")
	if tag == "-" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	if !tagged || name == "" {
		name = field.Name
	}
	for _, option := range parts[1:] {
		if option == "omitempty" {
			optional = true
		}
	}
	return name, optional, true
}

func isIntegerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}
