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
	f, _, found := findFrame(3)
	return f, found
}

// FrameSkip returns the first stack frame outside the log package tree along
// with how many logical frames above the caller lie inside the log tree.
func FrameSkip() (frame runtime.Frame, skip int, ok bool) {
	return findFrame(3)
}

func findFrame(callersSkip int) (frame runtime.Frame, skip int, ok bool) {
	var stackBuf [16]uintptr
	n := runtime.Callers(callersSkip, stackBuf[:])
	frames := runtime.CallersFrames(stackBuf[:n])
	for {
		f, more := frames.Next()
		if !inTree(f.Function) {
			return f, skip, true
		}
		skip++
		if !more {
			break
		}
	}
	if n < len(stackBuf) {
		return runtime.Frame{}, 0, false
	}

	pcs := make([]uintptr, 32)
	for {
		skip = 0
		n := runtime.Callers(callersSkip, pcs)
		frames := runtime.CallersFrames(pcs[:n])
		for {
			f, more := frames.Next()
			if !inTree(f.Function) {
				return f, skip, true
			}
			skip++
			if !more {
				break
			}
		}
		if n < len(pcs) {
			return runtime.Frame{}, 0, false
		}
		pcs = make([]uintptr, 2*len(pcs))
	}
}

// inTree reports whether function, a fully qualified name such as
// "example.com/pkg.(*T).M.func1", belongs to a package in the log tree.
func inTree(function string) bool {
	pkg := function
	if bracket := strings.IndexByte(pkg, '['); bracket >= 0 {
		pkg = pkg[:bracket]
	}
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
