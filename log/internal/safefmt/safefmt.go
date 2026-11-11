// Package safefmt formats arbitrary values with fmt without risking the
// unbounded recursion fmt has on cyclic values.
package safefmt

import (
	"fmt"
	"reflect"
)

// maxDepth bounds how deep Sprint will look before refusing to format a value.
// It counts reflection levels, and each %w layer of an error chain costs three
// of them (interface, pointer, struct), so it leaves room for real error
// chains. Cycles are caught by identity, not by this bound.
const maxDepth = 128

// Sprint returns fmt.Sprint(v), or a "<unformattable T: ...>" placeholder when
// v is cyclic or deeper than maxDepth reflection levels. The structural check
// covers every value, including those with Error, String or Format methods,
// because such a method can format its own cycle.
func Sprint(v any) string {
	// fmt.Sprint has no cycle detection, so a self-referential container recurses
	// until the goroutine stack is exhausted. That failure is fatal and cannot be
	// recovered — unlike a panic from a MarshalJSON, which the backend catches —
	// so it has to be prevented rather than handled. Values that are cyclic or
	// implausibly deep are reported instead of formatted.
	w := formatWalk{
		onPath:   make(map[containerKey]struct{}),
		verified: make(map[containerKey]int),
	}
	if _, ok := w.walk(reflect.ValueOf(v), 0); !ok {
		return fmt.Sprintf("<unformattable %T: cyclic or deeper than %d levels>", v, maxDepth)
	}
	return fmt.Sprint(v)
}

// containerKey identifies a container. For slices the length is part of the
// identity: rv.Pointer() is the backing array's base address, which an
// aliasing sub-slice (s[0:1] inside s) shares with its ancestor without forming
// a cycle, while a genuine self-reference has both the same base and the same
// length. Non-slice containers use -1. The type separates a pointer to a struct
// from a pointer to its first field, which share an address.
type containerKey struct {
	ptr    uintptr
	length int
	typ    reflect.Type
}

// formatWalk checks whether a value can be handed to fmt.Sprint without risking
// unbounded recursion. onPath holds the containers on the current path, so a
// value reachable from itself is rejected while the same value appearing twice
// side by side is not. verified maps each container already found acyclic to
// the number of levels below it, so a shared subtree is walked once instead of
// once per path that reaches it.
type formatWalk struct {
	onPath   map[containerKey]struct{}
	verified map[containerKey]int
}

// walk reports whether rv, found depth levels down, is formattable and, if so,
// how many levels lie below it.
func (w *formatWalk) walk(rv reflect.Value, depth int) (height int, ok bool) {
	if depth > maxDepth {
		return 0, false
	}
	switch rv.Kind() {
	case reflect.Invalid:
		return 0, true
	case reflect.Interface:
		if rv.IsNil() {
			return 0, true
		}
		return w.child(rv.Elem(), depth)
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if rv.IsNil() {
			return 0, true
		}
		key := containerKey{ptr: rv.Pointer(), length: -1, typ: rv.Type()}
		if rv.Kind() == reflect.Slice {
			key.length = rv.Len()
		}
		if _, ok := w.onPath[key]; ok {
			return 0, false
		}
		if h, ok := w.verified[key]; ok {
			return h, depth+h <= maxDepth
		}
		w.onPath[key] = struct{}{}
		defer delete(w.onPath, key)

		switch rv.Kind() {
		case reflect.Pointer:
			height, ok = w.child(rv.Elem(), depth)
		case reflect.Map:
			ok = true
			iter := rv.MapRange()
			for ok && iter.Next() {
				height, ok = w.maxChild(height, iter.Key(), depth)
				if ok {
					height, ok = w.maxChild(height, iter.Value(), depth)
				}
			}
		default: // slice
			height, ok = w.elems(rv, depth)
		}
		if ok {
			w.verified[key] = height
		}
		return height, ok
	case reflect.Array:
		return w.elems(rv, depth)
	case reflect.Struct:
		ok = true
		for i := 0; ok && i < rv.NumField(); i++ {
			height, ok = w.maxChild(height, rv.Field(i), depth)
		}
		return height, ok
	default:
		return 0, true
	}
}

// child walks rv as the child of a value at depth and returns its height as
// seen from that parent.
func (w *formatWalk) child(rv reflect.Value, depth int) (int, bool) {
	h, ok := w.walk(rv, depth+1)
	return h + 1, ok
}

// maxChild walks rv as a child of a value at depth and folds its height into
// height.
func (w *formatWalk) maxChild(height int, rv reflect.Value, depth int) (int, bool) {
	h, ok := w.child(rv, depth)
	return max(height, h), ok
}

func (w *formatWalk) elems(rv reflect.Value, depth int) (height int, ok bool) {
	ok = true
	for i := 0; ok && i < rv.Len(); i++ {
		height, ok = w.maxChild(height, rv.Index(i), depth)
	}
	return height, ok
}
