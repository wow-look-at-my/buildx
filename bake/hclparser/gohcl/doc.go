// Copyright (c) HashiCorp, Inc. SPDX-License-Identifier: MPL-2.0

// Package gohcl allows decoding HCL configurations into Go data structures.
//
// It provides a convenient and concise way of describing the schema for
// configuration and then accessing the resulting data via native Go types.
//
// A struct field tag scheme is used, similar to other decoding and
// unmarshalling libraries. The tags are formatted as in the following
// example:.
//
//	ThingType string `hcl:"thing_type,attr"`
//
// The first is the name of the corresponding construct in configuration,
// while the second is a keyword giving the kind of construct expected.
package gohcl
