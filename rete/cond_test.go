package rete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ccbhj/grete/rete"
)

var _ = Describe("Cond", func() {
	var cond *rete.Cond
	BeforeEach(func() {
		cond = &rete.Cond{
			Name:     "Foo",
			Attr:     "field",
			Value:    rete.TVString("bar"),
			Negative: false,
			TestOp:   rete.TestOpLess,
		}
	})
	It("can generate hash by Name/Attr/Value at lowest 32 bit", func() {
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
})
