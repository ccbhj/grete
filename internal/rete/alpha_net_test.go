package rete_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	. "github.com/ccbhj/grete/internal/rete"
)

var _ = Describe("AlphaNet", func() {
	var (
		an *AlphaNetwork
	)
	BeforeEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
	})

	Describe("constructing AlphaNetwork", func() {
		It("allowed the same condition to be added for more than one time", func() {
			am := an.MakeAlphaMem(Cond{
				ID:     TVIdentity("x"),
				Attr:   "on",
				Value:  TVString("y"),
				TestOp: TestOpEqual,
			})
			Expect(am).NotTo(BeNil())
			Expect(an.AlphaRoot()).NotTo(BeNil())
		})

		It("allowed the conditions with same Attr but different Values that are an Identity to generate same AlphaMem", func() {
			am0 := an.MakeAlphaMem(Cond{
				ID:     TVIdentity("x"),
				Attr:   "on",
				Value:  TVIdentity("y"),
				TestOp: TestOpEqual,
			})
			Expect(am0).NotTo(BeNil())
			am1 := an.MakeAlphaMem(Cond{
				ID:     TVIdentity("x"),
				Attr:   "on",
				Value:  TVIdentity("z"),
				TestOp: TestOpEqual,
			})
			Expect(am1).NotTo(BeNil())
			Expect(fmt.Sprintf("%p", am0)).To(BeEquivalentTo(fmt.Sprintf("%p", am1)))
		})

		When("testing condition with value of Identity type", func() {
			var child *ConstantTestNode
			var am *AlphaMem
			BeforeEach(func() {
				am = an.MakeAlphaMem(Cond{
					ID:     TVIdentity("x"),
					Attr:   "on",
					Value:  TVIdentity("y"),
					TestOp: TestOpEqual,
				})
				an.AlphaRoot().ForEachChild(func(tn *ConstantTestNode) (stop bool) {
					child = tn
					return true
				})
			})

			It("should only test the Attr of a WME", func() {
				Expect(child.OutputMem()).To(Equal(am))
				Expect(child.Activate(NewWME("X", "on", TVString("Y")))).To(Equal(1))
			})
		})

		Context("testing condition with value of non-Identity type as tested Value", func() {
			var child, grandChild *ConstantTestNode
			var am *AlphaMem
			BeforeEach(func() {
				am = an.MakeAlphaMem(Cond{
					ID:     TVIdentity("x"),
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})

				an.AlphaRoot().ForEachChild(func(tn *ConstantTestNode) (stop bool) {
					child = tn
					return true
				})
				child.ForEachChild(func(tn *ConstantTestNode) (stop bool) {
					grandChild = tn
					return true
				})
			})

			It("should construct a tree with depth of 3 ", func() {
				Expect(an.AlphaRoot()).NotTo(BeNil())
				if Expect(child).NotTo(BeNil()) {
					Expect(grandChild).NotTo(BeNil())
				}
			})

			It("should test the Attr and Value of a WME", func() {
				Expect(grandChild.OutputMem()).To(Equal(am))
				// only activate by Value
				Expect(grandChild.Activate(NewWME("X", "background_color", TVString("red")))).To(Equal(1))
				// activate by both Attr and Value
				Expect(child.Activate(NewWME("X", TVString("color"), TVString("red")))).To(Equal(1))
			})
		})
	})

	Describe("Adding fact", func() {
		var (
			testFacts []Fact
			ams       []*AlphaMem
			conds     = []Cond{
				{
					ID:     TVIdentity("x"),
					Attr:   "on",
					Value:  TVIdentity("y"),
					TestOp: TestOpEqual,
				},
				{
					ID:     TVIdentity("x"),
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				},
			}
		)

		BeforeEach(func() {
			ams = make([]*AlphaMem, 0, len(conds))
			for _, c := range conds {
				am := an.MakeAlphaMem(c)
				Expect(am).NotTo(BeNil())
				ams = append(ams, am)
			}

			testFacts = getTestFacts()
			lo.ForEach(testFacts, func(item Fact, _ int) { an.AddFact(item) })
		})

		AfterEach(func() {
			testFacts = testFacts[:0]
		})

		It("should matched WMEs with Identity value", func() {
			onCondFact := lo.Filter(testFacts, func(f Fact, idx int) bool {
				return f.Field == "on"
			})
			onFact := ams[0]
			Expect(onCondFact).To(HaveLen(onFact.NItems()))
			onFact.ForEachItem(func(w *WME) (stop bool) {
				Expect(onCondFact).To(ContainElement(w.FactOfWME()))
				return false
			})
		})

		It("should matched Facts with contant value", func() {
			redFacts := lo.Filter(testFacts, func(f Fact, idx int) bool {
				return f.Field == "color" && f.Value == TVString("red")
			})
			redFact := ams[1]
			Expect(redFacts).To(HaveLen(redFact.NItems()))
			redFact.ForEachItem(func(w *WME) (stop bool) {
				GinkgoWriter.Printf("redCondAM => %p => %s", w, w)
				Expect(redFacts).To(ContainElement(w.FactOfWME()))
				return false
			})
		})
	})

	Describe("Remove facts", func() {
		var (
			testFacts []Fact
			ams       []*AlphaMem
			conds     = []Cond{
				{
					ID:     TVIdentity("x"),
					Attr:   "on",
					Value:  TVIdentity("y"),
					TestOp: TestOpEqual,
				},
				{
					ID:     TVIdentity("x"),
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				},
			}
		)

		BeforeEach(func() {
			ams = make([]*AlphaMem, 0, len(conds))
			for _, c := range conds {
				am := an.MakeAlphaMem(c)
				Expect(am).NotTo(BeNil())
				ams = append(ams, am)
			}

			testFacts = getTestFacts()
			lo.ForEach(testFacts, func(item Fact, _ int) { an.AddFact(item) })
		})

		AfterEach(func() {
			testFacts = testFacts[:0]
		})

		It("Remve facts should remove wme in alpha memory too", func() {
			factToRemoved := lo.Filter(testFacts, func(item Fact, index int) bool { return item.Field == "color" })
			lo.ForEach(
				factToRemoved,
				func(item Fact, _ int) { an.RemoveFact(item) },
			)

			Expect(ams[0].NItems()).NotTo(BeZero())
			Expect(ams[1].NItems()).To(BeZero())

			an.AddFact(Fact{"a", "color", TVString("red")})
			Expect(ams[1].NItems()).NotTo(BeZero())
		})
	})
})
