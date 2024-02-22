package rete

import (
	"github.com/zyedidia/generic/list"
)
func isListEmpty[T any](l *list.List[T]) bool {
	return l == nil || (l.Back == nil && l.Front == nil)
}

func removeOneFromListTailWhen[T any](l *list.List[T], cmp func(T) bool) bool {
	if l == nil {
		return false
	}
	p := l.Back
	for p != nil && !cmp(p.Value) {
		p = p.Prev
	}

	if p == nil || !cmp(p.Value) {
		return false
	}

	l.Remove(p)
	return true
}

func listHeadForEach[T any](l *list.List[T], fn func(T) (stop bool)) {
	node := l.Front
	for node != nil {
		if fn(node.Value) {
			return
		}
		node = node.Next
	}
}

func listToSlice[T any](l *list.List[T]) []T {
	arr := make([]T, 0)
	l.Front.Each(func(val T) {
		arr = append(arr, val)
	})

	return arr
}

func isIdentity(tv TestValue) bool {
	return tv.Type() == TestValueTypeIdentity
}
