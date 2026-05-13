// Package workspace contains sandbox-side workspace state.
//
// A file belongs to the sandbox workspace only after it has been imported into
// the sandbox or created by sandbox execution. Control-plane resources are not
// workspace files until explicitly imported.
package workspace
