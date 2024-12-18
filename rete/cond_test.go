package rete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/ccbhj/grete/rete"
	. "github.com/ccbhj/grete/types"
)

var _ = Describe("Cond", func() {
	var cond *Guard
	BeforeEach(func() {
		cond = &Guard{
			AliasAttr: "field",
			Value:     GVString("bar"),
			Negative:  false,
			TestOp:    TestOpLess,
		}
	})

	It("can generate hash by Attr/Value at lowest 32 bit", func() {
		hash := cond.Hash()
		Expect(hash).NotTo(BeZero())
		Expect(hash).Should(BeNumerically(">", 0))
	})

	It("can set highest bit for Negative condition", func() {
		cond.Negative = true
		hash := cond.Hash()
		Expect(hash).NotTo(BeZero())
		Expect(int(hash)).Should(BeNumerically("<", 0))
	})

	It("can generate same hash for other Conds with same Attr and same value", func() {
		hash := cond.Hash()
		otherCond := cond
		Expect(otherCond).NotTo(BeZero())
		Expect(hash).Should(BeEquivalentTo(otherCond.Hash()))
	})
})
