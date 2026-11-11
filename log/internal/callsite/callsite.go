// Package callsite finds the code that called into the log package tree, past
// any number of facade, wrapper and backend frames.
package callsite

import (
	"reflect"
	"runtime"
	"strings"
)

type marker struct{}

// root is the import path of the log package; the tree is root and every
// package below it except external test packages.
var root = strings.TrimSuffix(reflect.TypeFor[marker]().PkgPath(), "/internal/callsite")

// Frame returns the first stack frame whose function lies outside the log
// package tree, or ok=false when the whole stack is inside it.
func Frame() (frame runtime.Frame, ok bool) {
	pcs := make([]uintptr, 32)
	for {
		// Skip runtime.Callers and Frame itself.
		n := runtime.Callers(2, pcs)
		frames := runtime.CallersFrames(pcs[:n])
		for {
			f, more := frames.Next()
			if !inTree(f.Function) {
				return f, true
			}
			if !more {
				break
			}
		}
		if n < len(pcs) {
			return runtime.Frame{}, false
		}
		pcs = make([]uintptr, 2*len(pcs))
	}
}

// inTree reports whether function, a fully qualified name such as
// "example.com/pkg.(*T).M.func1", belongs to a package in the log tree.
func inTree(function string) bool {
	pkg := function
	if slash := strings.LastIndexByte(pkg, '/'); slash >= 0 {
		if dot := strings.IndexByte(pkg[slash:], '.'); dot >= 0 {
			pkg = pkg[:slash+dot]
		}
	}
	if pkg == root {
		return true
	}
	return strings.HasPrefix(pkg, root+"/") && !strings.HasSuffix(pkg, "_test")
}
