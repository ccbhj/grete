package rete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	. "github.com/ccbhj/grete/rete"
)

func getTestFacts() []Fact {
	return []Fact{
		{ID: "B1", Field: "on", Value: TVIdentity("B2")},
		{ID: "B1", Field: "on", Value: TVIdentity("B3")},
		{ID: "B1", Field: "color", Value: TVString("red")},
		{ID: "B2", Field: "on", Value: TVString("table")},
		{ID: "B2", Field: "left-of", Value: TVIdentity("B3")},
		{ID: "B2", Field: "color", Value: TVString("blue")},
		{ID: "B3", Field: "left-of", Value: TVIdentity("B4")},
		{ID: "B3", Field: "on", Value: TVString("table")},
		{ID: "B3", Field: "color", Value: TVString("red")},
	}
}

var testConds = []Cond{
	{
		Name:   TVIdentity("x"),
		Attr:   "on",
		Value:  TVIdentity("y"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("y"),
		Attr:   "left-of",
		Value:  TVIdentity("z"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("z"),
		Attr:   "color",
		Value:  TVString("red"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("z"),
		Attr:   "color",
		Value:  TVString("maize"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("b"),
		Attr:   "color",
		Value:  TVString("blue"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("c"),
		Attr:   "color",
		Value:  TVString("green"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("d"),
		Attr:   "color",
		Value:  TVString("white"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("s"),
		Attr:   "on",
		Value:  TVString("table"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("y"),
		Attr:   "a",
		Value:  TVString("b"),
		TestOp: TestOpEqual,
	},
	{
		Name:   TVIdentity("a"),
		Attr:   "left-of",
		Value:  TVString("d"),
		TestOp: TestOpEqual,
	},
}

var _ = Describe("BetaNet", func() {
	var (
		bn *BetaNetwork
		an *AlphaNetwork
	)

	BeforeEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
		bn = NewBetaNetwork(an)
		Expect(bn).NotTo(BeNil())
	})

	Describe("when building BetaNetwork", func() {
		AfterEach(func() {
			an = NewAlphaNetwork()
			Expect(an).NotTo(BeNil())
			bn = NewBetaNetwork(an)
			Expect(bn).NotTo(BeNil())
		})

		Context("allowed to add production", func() {
			It("can add wme before production added", func() {
				bn.AddFact(getTestFacts()...)
				pNode := bn.AddProduction("test", Cond{
					Name:   "x",
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})
				Expect(pNode.AnyMatches()).To(BeTrue())

				matches := pNode.Matches()
				redFact := lo.Filter(getTestFacts(), func(f Fact, idx int) bool {
					return f.Field == "color" && f.Value == TVString("red")
				})
				Expect(matches).To(HaveLen(len(redFact)))
				for _, fact := range redFact {
					Expect(matches).To(ContainElement(map[TVIdentity]Fact{
						"x": fact,
					}))
				}
			})

			It("can add production before wmes added", func() {
				pNode := bn.AddProduction("test", Cond{
					Name:   "x",
					Attr:   "on",
					Value:  TVString("table"),
					TestOp: TestOpEqual,
				})

				Expect(pNode.AnyMatches()).To(BeFalse())
				bn.AddFact(getTestFacts()...)
				Expect(pNode.AnyMatches()).To(BeTrue())

				matches := pNode.Matches()
				onTableFact := lo.Filter(getTestFacts(), func(f Fact, idx int) bool {
					return f.Field == "on" && f.Value == TVString("table")
				})
				Expect(matches).To(HaveLen(len(onTableFact)))
				for _, fact := range onTableFact {
					Expect(matches).To(ContainElement(map[TVIdentity]Fact{
						"x": fact,
					}))
				}
			})

			It("can add production with more than one condition", func() {
				pNode := bn.AddProduction("test case from Fig2.2 in paper",
					Cond{
						Name:   TVIdentity("x"),
						Attr:   "on",
						Value:  TVIdentity("y"),
						TestOp: TestOpEqual,
					},
					Cond{
						Name:   TVIdentity("y"),
						Attr:   "left-of",
						Value:  TVIdentity("z"),
						TestOp: TestOpEqual,
					},
					Cond{
						Name:   TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})

				Expect(pNode.AnyMatches()).To(BeFalse())
				bn.AddFact(getTestFacts()...)
				Expect(pNode.AnyMatches()).To(BeTrue())

				matches := pNode.Matches()
				Expect(matches).To(HaveLen(1))
				Expect(matches[0]).Should(And(
					HaveKeyWithValue(TVIdentity("x"), Fact{ID: "B1", Field: "on", Value: TVIdentity("B2")}),
					HaveKeyWithValue(TVIdentity("y"), Fact{ID: "B2", Field: "left-of", Value: TVIdentity("B3")}),
					HaveKeyWithValue(TVIdentity("z"), Fact{ID: "B3", Field: "color", Value: TVString("red")}),
				))
			})

			It("can add the same production for more than one time", func() {
				// test case from Fig2.2
				const ruleID = "single rule"
				px := bn.AddProduction(ruleID,
					Cond{
						Name:   TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})

				py := bn.AddProduction(ruleID,
					Cond{
						Name:   TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})
				Expect(px).To(BeEquivalentTo(py))

				pNode := bn.GetPNode(ruleID)
				Expect(pNode).To(BeEquivalentTo(px))
			})
		})
	})
})
