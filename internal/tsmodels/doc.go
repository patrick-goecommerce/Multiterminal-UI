// Package tsmodels checks frontend/wailsjs/go/models.ts against the Go structs
// it mirrors.
//
// Wails v3 does not regenerate that file, so every field added to a Go struct
// that reaches the frontend has to be added to models.ts by hand, in two
// places: the class declaration and the constructor. Miss the constructor and
// the field is silently dropped on deserialization. Nothing fails, nothing
// warns, the frontend simply always reads undefined. CLAUDE.md calls it a
// recurring bug, and it is: status_line shipped that way once.
//
// The check here is deliberately not a generator. Generating the file would
// mean reproducing Wails' own convertValues semantics for every nested struct,
// slice and map across 47 classes, and a generator that gets that subtly wrong
// breaks the whole frontend at once and just as silently. A check has no such
// failure mode: it either names the missing field or says nothing.
//
// It compares in one direction only. Every class in models.ts must carry every
// json field of the Go struct it mirrors; a Go struct that the frontend never
// sees is not required to appear. That is the direction the bug travels.
package tsmodels
