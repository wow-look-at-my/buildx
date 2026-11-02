// Package userfunc implements a HCL extension that allows user-defined
// functions in HCL configuration.
//
// Using this extension requires some integration effort on the part of the
// calling application, to pass any declared functions into a HCL evaluation
// context. This happens after processing.
//
// The function declaration syntax looks like this:.
//
//	function "foo" {
//	  params = ["name"]
//	  result = "Hello, ${name}!"
//	}
//
// When a user-defined function is called, the expression given for the "result"
// attribute is evaluated in an isolated evaluation context. That context defines
// variables named after the given parameter names.
package userfunc
