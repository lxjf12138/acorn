// Package workspace contains sandbox-service workspace domain boundaries.
//
// A workspace belongs to sandbox-service; it is not a global Acorn filesystem.
// A sandbox may mount a workspace, and files become workspace files only after
// explicit ResourceRef import or sandbox execution.
package workspace
