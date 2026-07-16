// Package sdk provides the compatibility facade for the public RPL SDK.
//
// New code should prefer focused subpackages such as:
//   - sdk/catalog for editor type catalogs
//   - sdk/codegen for generation-facing helpers
//   - sdk/schema for structural model data
//   - sdk/target for language target authoring
//
// The root package remains stable so existing attrs and tools continue to work.
package sdk
