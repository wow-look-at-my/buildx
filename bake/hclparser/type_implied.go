// MIT License.
//
// Copyright (c) Martin Atkins.
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software. It associated documentation files (the "Software").
// It also associated to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense. And/or sell copies of the Software, and to permit
// persons to whom the Software is furnished to do so, subject to the
// following conditions:.
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// type_implied.go holds the type implied code.

package hclparser

import (
	"reflect"

	"github.com/zclconf/go-cty/cty"
)

// ImpliedType takes an arbitrary Go value (as an interface{}) and attempts to
// find a suitable cty.Type instance that could be used for a conversion with
// ToCtyValue.
func ImpliedType(gv any) (cty.Type, error) {
	rt := reflect.TypeOf(gv)
	var path cty.Path
	return impliedType(rt, path)
}

func impliedType(rt reflect.Type, path cty.Path) (cty.Type, error) {
	if ety, err := impliedTypeExt(rt, path); err == nil {
		return ety, nil
	}

	switch rt.Kind() {
	case reflect.Ptr:
		return impliedType(rt.Elem(), path)

	// Primitive types
	case reflect.Bool:
		return cty.Bool, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cty.Number, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return cty.Number, nil
	case reflect.Float32, reflect.Float64:
		return cty.Number, nil
	case reflect.String:
		return cty.String, nil

	// Collection types
	case reflect.Slice:
		path := append(path, cty.IndexStep{Key: cty.UnknownVal(cty.Number)})
		ety, err := impliedType(rt.Elem(), path)
		if err != nil {
			return cty.NilType, err
		}
		return cty.List(ety), nil
	case reflect.Map:
		if !stringType.AssignableTo(rt.Key()) {
			return cty.NilType, path.NewErrorf("no cty.Type for %s (must have string keys)", rt)
		}
		path := append(path, cty.IndexStep{Key: cty.UnknownVal(cty.String)})
		ety, err := impliedType(rt.Elem(), path)
		if err != nil {
			return cty.NilType, err
		}
		return cty.Map(ety), nil

	// Structural types
	case reflect.Struct:
		return impliedStructType(rt, path)

	default:
		return cty.NilType, path.NewErrorf("no cty.Type for %s", rt)
	}
}

func impliedStructType(rt reflect.Type, path cty.Path) (cty.Type, error) {
	if valueType.AssignableTo(rt) {
		// Special case: cty.Value represents cty.DynamicPseudoType, for type conformance checking.
		return cty.DynamicPseudoType, nil
	}

	fieldIdxs := structTagIndices(rt)
	if len(fieldIdxs) == 0 {
		return cty.NilType, path.NewErrorf("no cty.Type for %s (no cty field tags)", rt)
	}

	atys := make(map[string]cty.Type, len(fieldIdxs))

	{
		// Temporary extension of path for attributes
		path := append(path, nil)

		for k, fi := range fieldIdxs {
			path[len(path)-1] = cty.GetAttrStep{Name: k}

			ft := rt.Field(fi).Type
			aty, err := impliedType(ft, path)
			if err != nil {
				return cty.NilType, err
			}

			atys[k] = aty
		}
	}

	return cty.Object(atys), nil
}

var (
	valueType  = reflect.TypeFor[cty.Value]()
	stringType = reflect.TypeFor[string]()
)

// structTagIndices interrogates the fields of the given type (which must be a struct type. Otherwise, we'll panic) and returns a map from the cty attribute names declared via struct tags to the
// indices of the fields holding those tags.
func structTagIndices(st reflect.Type) map[string]int {
	ct := st.NumField()
	ret := make(map[string]int, ct)

	for i := range ct {
		field := st.Field(i)
		attrName := field.Tag.Get("cty")
		if attrName != "" {
			ret[attrName] = i
		}
	}

	return ret
}
