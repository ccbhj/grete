package rete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	. "github.com/ccbhj/grete/internal/rete"
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
		ID:     TVIdentity("x"),
		Attr:   "on",
		Value:  TVIdentity("y"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("y"),
		Attr:   "left-of",
		Value:  TVIdentity("z"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("z"),
		Attr:   "color",
		Value:  TVString("red"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("z"),
		Attr:   "color",
		Value:  TVString("maize"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("b"),
		Attr:   "color",
		Value:  TVString("blue"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("c"),
		Attr:   "color",
		Value:  TVString("green"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("d"),
		Attr:   "color",
		Value:  TVString("white"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("s"),
		Attr:   "on",
		Value:  TVString("table"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("y"),
		Attr:   "a",
		Value:  TVString("b"),
		TestOp: TestOpEqual,
	},
	{
		ID:     TVIdentity("a"),
		Attr:   "left-of",
		Value:  TVString("d"),
		TestOp: TestOpEqual,
	},
}

func collect[T any](forEachFn func(func(T) (stop bool))) []T {
	arr := make([]T, 0, 2)
	forEachFn(func(t T) bool {
		arr = append(arr, t)
		return false
	})
	return arr
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

	AfterEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
		bn = NewBetaNetwork(an)
		Expect(bn).NotTo(BeNil())
	})

	Describe("construction", func() {
		When("adding production", func() {
			It("can add an single production", func() {
				pNode := bn.AddProduction("test", Cond{
					ID:     "x",
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})
				Expect(pNode.AnyMatches()).To(BeFalse())
				if Expect(pNode.Parent()).To(BeAssignableToTypeOf(&JoinNode{})) {
					if Expect(pNode.Parent().Parent()).To(BeAssignableToTypeOf(&BetaMem{})) {
						dummyBm := pNode.Parent().Parent().(*BetaMem)
						Expect(dummyBm.Parent()).To(BeNil())
					}
				}
			})

			It("can add production with more than one condition", func() {
				pNode := bn.AddProduction("test case from Fig2.2 in paper",
					Cond{
						ID:     TVIdentity("x"),
						Attr:   "on",
						Value:  TVIdentity("y"),
						TestOp: TestOpEqual,
					},
					Cond{
						ID:     TVIdentity("y"),
						Attr:   "left-of",
						Value:  TVIdentity("z"),
						TestOp: TestOpEqual,
					},
					Cond{
						ID:     TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})

				Expect(pNode.AnyMatches()).To(BeFalse())

				testFn := func(rete ReteNode) {
					if Expect(rete.Parent()).To(BeAssignableToTypeOf(&JoinNode{})) {
						Expect(pNode.Parent().Parent()).To(BeAssignableToTypeOf(&BetaMem{}))
					}
				}

				testFn(pNode)                   // condition 3
				testFn(pNode.Parent().Parent()) // condition 2

				// condition 1
				upNode := pNode.Parent().Parent().Parent().Parent().Parent()
				if Expect(upNode).To(BeAssignableToTypeOf(&JoinNode{})) {
					if Expect(upNode.Parent()).To(BeAssignableToTypeOf(&BetaMem{})) {
						dummyBm := upNode.Parent().(*BetaMem)
						Expect(dummyBm.Parent()).To(BeNil())
					}
				}
			})

			It("can add the same production for more than one time", func() {
				// test case from Fig2.2
				const ruleID = "single rule"
				px := bn.AddProduction(ruleID,
					Cond{
						ID:     TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})

				py := bn.AddProduction(ruleID,
					Cond{
						ID:     TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})
				Expect(px).To(BeEquivalentTo(py))

				pNode := bn.GetProduction(ruleID)
				Expect(pNode).To(BeIdenticalTo(px))
			})

			It("can share same beta memory", func() {
				pNodeX := bn.AddProduction("X",
					Cond{
						ID:     TVIdentity("x"),
						Attr:   "on",
						Value:  TVIdentity("y"),
						TestOp: TestOpEqual,
					},
					Cond{
						ID:     TVIdentity("y"),
						Attr:   "left-of",
						Value:  TVIdentity("z"),
						TestOp: TestOpEqual,
					},
					Cond{
						ID:     TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					})

				pNodeY := bn.AddProduction("Y",
					Cond{
						ID:     TVIdentity("x"),
						Attr:   "on",
						Value:  TVIdentity("y"),
						TestOp: TestOpEqual,
					},
					Cond{
						ID:     TVIdentity("y"),
						Attr:   "left-of",
						Value:  TVIdentity("z"),
						TestOp: TestOpEqual,
					},
					Cond{ // diferent rule here
						ID:     TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("blue"),
						TestOp: TestOpEqual,
					})

				Expect(pNodeX.Parent().Parent()).To(BeIdenticalTo(pNodeY.Parent().Parent()))
			})
		})

		When("removing production", func() {
			var redFact []Fact
			BeforeEach(func() {
				redFact = lo.Filter(getTestFacts(), func(item Fact, index int) bool {
					return item.Field == "color" && item.Value == TVString("red")
				})
			})

			It("can remove a single rule", func() {
				const ruleName = "single_rule"
				singleRule := bn.AddProduction(ruleName, Cond{
					ID:     "x",
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})
				lo.ForEach(redFact, func(item Fact, _ int) { bn.AddFact(item) })
				p := singleRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())
				Expect(singleRule.Matches()).ShouldNot(BeEmpty())

				// now remove it
				Expect(bn.RemoveProduction(ruleName)).Should(BeNil())
				Expect(p.AnyChild()).Should(BeFalse())
				Expect(collect(p.ForEachChild)).Should(BeEmpty())
				Expect(singleRule.Matches()).Should(BeEmpty())
				Expect(collect(an.AlphaRoot().ForEachChild)).Should(BeEmpty())

				// now add it again...
				singleRule = bn.AddProduction(ruleName, Cond{
					ID:     "x",
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})
				Expect(singleRule).ShouldNot(BeNil())
				p = singleRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())

				lo.ForEach(redFact, func(item Fact, _ int) { bn.AddFact(item) })
				Expect(singleRule.Matches()).ShouldNot(BeEmpty())
			})

			It("can remove a multi-rule production and add again", func() {
				const ruleName = "multi_rule"
				rules := []Cond{
					{
						ID:     TVIdentity("x"),
						Attr:   "on",
						Value:  TVIdentity("y"),
						TestOp: TestOpEqual,
					},
					{
						ID:     TVIdentity("y"),
						Attr:   "left-of",
						Value:  TVIdentity("z"),
						TestOp: TestOpEqual,
					},
					{
						ID:     TVIdentity("z"),
						Attr:   "color",
						Value:  TVString("red"),
						TestOp: TestOpEqual,
					}}
				multiRule := bn.AddProduction(ruleName, rules...)

				p := multiRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())
				lo.ForEach(getTestFacts(), func(item Fact, _ int) { bn.AddFact(item) })
				Expect(multiRule.Matches()).ShouldNot(BeEmpty())

				// now remove it
				Expect(bn.RemoveProduction(ruleName)).Should(BeNil())
				Expect(p.AnyChild()).Should(BeFalse())
				Expect(collect(p.ForEachChild)).Should(BeEmpty())
				Expect(multiRule.Matches()).Should(BeEmpty())

				// now add it again...
				multiRule = bn.AddProduction(ruleName, Cond{
					ID:     "x",
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})
				Expect(multiRule).ShouldNot(BeNil())
				p = multiRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())

				lo.ForEach(getTestFacts(), func(item Fact, _ int) { bn.AddFact(item) })
				Expect(multiRule.Matches()).ShouldNot(BeEmpty())
			})

		})

	})

	When("adding/removing facts", func() {
		var (
			singleRule *PNode
			multiRule  *PNode
		)
		BeforeEach(func() {
			singleRule = bn.AddProduction("single_rule", Cond{
				ID:     "x",
				Attr:   "color",
				Value:  TVString("red"),
				TestOp: TestOpEqual,
			})

			multiRule = bn.AddProduction("test case from Fig2.2 in paper",
				Cond{
					ID:     TVIdentity("x"),
					Attr:   "on",
					Value:  TVIdentity("y"),
					TestOp: TestOpEqual,
				},
				Cond{
					ID:     TVIdentity("y"),
					Attr:   "left-of",
					Value:  TVIdentity("z"),
					TestOp: TestOpEqual,
				},
				Cond{
					ID:     TVIdentity("z"),
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				})

			lo.ForEach(getTestFacts(), func(f Fact, _ int) { bn.AddFact(f) })
		})

		Context("matching facts with single rule", func() {
			var (
				redFact []Fact
				matches []map[TVIdentity]Fact
			)
			BeforeEach(func() {
				redFact = lo.Filter(getTestFacts(), func(f Fact, idx int) bool {
					return f.Field == "color" && f.Value == TVString("red")
				})
				matches = singleRule.Matches()
			})

			It("should match some facts", func() {
				Expect(matches).Should(HaveLen(len(redFact)))
				for _, fact := range redFact {
					Expect(matches).To(ContainElement(map[TVIdentity]Fact{
						"x": fact,
					}))
				}
			})

			It("should still match some facts after removing one", func() {
				bn.RemoveFact(redFact[0])
				for _, fact := range redFact[1:] {
					Expect(matches).To(ContainElement(map[TVIdentity]Fact{
						"x": fact,
					}))
				}
			})

			It("should not match any facts after removing all", func() {
				for _, fact := range redFact {
					bn.RemoveFact(fact)
				}
				Expect(singleRule.AnyMatches()).Should(BeFalse())
			})
		})

		Context("can match facts with multi rules", func() {
			It("should match some facts", func() {
				Expect(multiRule.AnyMatches()).To(BeTrue())
				matches := multiRule.Matches()
				Expect(matches).To(HaveLen(1))
				Expect(matches[0]).Should(And(
					HaveKeyWithValue(TVIdentity("x"), Fact{ID: "B1", Field: "on", Value: TVIdentity("B2")}),
					HaveKeyWithValue(TVIdentity("y"), Fact{ID: "B2", Field: "left-of", Value: TVIdentity("B3")}),
					HaveKeyWithValue(TVIdentity("z"), Fact{ID: "B3", Field: "color", Value: TVString("red")}),
				))
			})

			It("should not match any facts after removing one of the matched fact", func() {
				f := Fact{ID: "B1", Field: "on", Value: TVIdentity("B2")}
				bn.RemoveFact(f)
				Expect(multiRule.AnyMatches()).Should(BeFalse())

				bn.AddFact(f)
				Expect(multiRule.AnyMatches()).Should(BeTrue())
				matches := multiRule.Matches()
				Expect(matches).To(HaveLen(1))
				Expect(matches[0]).Should(And(
					HaveKeyWithValue(TVIdentity("x"), Fact{ID: "B1", Field: "on", Value: TVIdentity("B2")}),
					HaveKeyWithValue(TVIdentity("y"), Fact{ID: "B2", Field: "left-of", Value: TVIdentity("B3")}),
					HaveKeyWithValue(TVIdentity("z"), Fact{ID: "B3", Field: "color", Value: TVString("red")}),
				))
			})
		})
	})

})
