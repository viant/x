// Package ast provides a thin façade over the syntetic/model AST helpers so
// you can use a consistent "builder" entrypoint for generating declarations and
// rendered files. It intentionally delegates to model.Type/GoFile/Package
// methods and does not duplicate AST construction logic.
package ast
