package rete

import (
	"encoding/binary"
	"hash/fnv"
	"math/bits"
)

// const goldenRatio32 uint32 = 0x61C88647 // phi = (sqrt(5)-1)/2
//
// func hash32(v uint32) uint32 {
// 	return uint32(v) * goldenRatio32
// }
//
// func hash64(v uint64) uint32 {
// 	return hash32(uint32(v) ^ hash32(uint32(v>>32)))
// }
//
// func hashCombine32(x, y uint32) uint32 {
// 	return hash32(hash32(x) ^ hash32(y))
// }

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
