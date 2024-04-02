package rete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/ccbhj/grete/internal/rete"
	. "github.com/ccbhj/grete/internal/types"
)

var _ = Describe("Cond", func() {
	var cond *Cond
	BeforeEach(func() {
		cond = &Cond{
			Alias:     "Foo",
			AliasAttr: "field",
			Value:     GVString("bar"),
			Negative:  false,
			TestOp:    TestOpLess,
		}
	})

	It("can generate hash by Attr/Value at lowest 32 bit", func() {
		hash := cond.Hash(0)
		Expect(hash).NotTo(BeZero())
		Expect(hash).Should(BeNumerically(">", 0))
	})

	It("can set highest bit for Negative condition", func() {
		cond.Negative = true
		hash := cond.Hash(0)
		Expect(hash).NotTo(BeZero())
		Expect(int(hash)).Should(BeNumerically("<", 0))
	})

	It("can generate same hash for other Conds with same Attr and same value", func() {
		opt := CondHashOptMaskID
		hash := cond.Hash(opt)
		otherCond := cond
		otherCond.Alias = "Bar"
		Expect(otherCond).NotTo(BeZero())
		Expect(hash).Should(BeEquivalentTo(otherCond.Hash(opt)))
	})

	It("can generate same hash for other Conds with same Attr if Value is an Identity", func() {
		opt := CondHashOptMaskValue
		hash := cond.Hash(opt)
		otherCond := cond
		otherCond.Value = GVIdentity("X")
		Expect(otherCond).NotTo(BeZero())
		Expect(hash).Should(BeEquivalentTo(otherCond.Hash(opt)))
	})
})
