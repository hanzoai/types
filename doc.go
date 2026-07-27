// Package types holds the values Hanzo services share.
//
// It imports the standard library only, so a service can depend on a value
// without inheriting the machinery of whoever else uses it.
//
// A value earns a place here when two or more services must agree on it
// exactly. Fetching, storing and recording are effects, and an effect belongs
// with the service that owns it, next to the schema or transport it acts on.
package types
