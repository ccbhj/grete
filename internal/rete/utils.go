package rete

import (
	"encoding/binary"
	"hash/fnv"
	"math/bits"

	H "github.com/mitchellh/hashstructure/v2"
	"github.com/zyedidia/generic/list"
)

type set[T comparable] map[T]struct{}

func newSet[T comparable]() set[T] {
	return make(set[T])
}

func (s set[T]) Add(t T) {
	s[t] = struct{}{}
}

func (s set[T]) Del(t T) {
	delete(s, t)
}

func (s set[T]) Clear() {
	clear(s)
}

func (s set[T]) Contains(t T) bool {
	_, in := s[t]
	return in
}

func (s set[T]) Len() int {
	return len(s)
}

func (s set[T]) ForEach(fn func(T)) {
	for e := range s {
		fn(e)
	}
}

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

func hashAny(v any) uint64 {
	h, err := H.Hash(v, H.FormatV2, &H.HashOptions{})
	if err != nil {
		panic(err)
	}
	return h
}

// hash32 generate hash of an uint64
func hash32(v uint64) uint32 {
	h := fnv.New32a()
	if err := binary.Write(h, binary.LittleEndian, v); err != nil {
		panic(err)
	}

	return h.Sum32()
}

// mix32 mixes two uint32 into one
func mix32(x, y uint32) uint32 {
	// 0x53c5ca59 and 0x74743c1b are magic numbers from wyhash32(see https://github.com/wangyi-fudan/wyhash/blob/master/wyhash32.h)
	c := uint64(x ^ 0x53c5ca59)
	c *= uint64(y ^ 0x74743c1b)
	return hash32(c)
}

// mix64 mixes two uint64 into one
func mix64(x, y uint64) uint64 {
	hi, lo := bits.Mul64(x, y)
	return hi ^ lo
}
