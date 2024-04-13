package rete

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	. "github.com/ccbhj/grete/internal/types"
)

var testFacts = getTestFacts()

func collect[T any](fn func(func(item T) (stop bool))) []T {
	ret := make([]T, 0, 4)
	fn(func(item T) bool {
		ret = append(ret, item)
		return false
	})
	return ret
}

func ignoreIndex[T any](fn func(T)) func(T, int) {
	return func(t T, _ int) {
		fn(t)
	}
}

var _ = Describe("BetaNet", func() {
	var (
		bn *BetaNetwork
		an *AlphaNetwork
		tf = TypeInfo{
			T: GValueTypeStruct,
			Fields: map[string]GValueType{
				"Color": GValueTypeString,
				"On":    GValueTypeStruct,
			},
		}
		addFacts    func()
		removeFacts func(ids ...GVIdentity)
	)

	BeforeEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
		bn = NewBetaNetwork(an)
		Expect(bn).NotTo(BeNil())

		addFacts = func() {
			lo.ForEach(testFacts, func(item *Chess, _ int) {
				bn.AddFact(Fact{
					ID:    item.ID,
					Value: NewGVStruct(item),
				})
			})
		}

		removeFacts = func(ids ...GVIdentity) {
			facts := testFacts
			if len(ids) > 0 {
				facts = lo.Filter(facts, func(item *Chess, _ int) bool {
					for _, id := range ids {
						if item.ID == id {
							return true
						}
					}
					return false
				})
			}

			lo.ForEach(facts, func(item *Chess, _ int) {
				bn.RemoveFact(Fact{
					ID:    item.ID,
					Value: NewGVStruct(item),
				})
			})
		}
	})

	Describe("node sharing", func() {
		var p0, p1, p2 *PNode

		BeforeEach(func() {
			p0 = bn.AddProduction(Production{
				ID: "p0",
				When: []AliasDeclaration{
					{
						Alias: "X",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("red"),
								TestOp:    TestOpEqual,
							},
						},
					},
					{
						Alias: "Y",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("blue"),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
				Match: []JoinTest{
					{
						Alias:  []Selector{{"X", "Rank"}, {"Y", "Rank"}},
						TestOp: TestOpLess,
					},
				},
			})

			p1 = bn.AddProduction(Production{
				ID: "p1",
				When: []AliasDeclaration{
					{
						Alias: "a",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("red"),
								TestOp:    TestOpEqual,
							},
						},
					},
					{
						Alias: "b",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("blue"),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
				Match: []JoinTest{
					{
						Alias:  []Selector{{"b", "Rank"}, {"a", "Rank"}},
						TestOp: TestOpLess,
					},
				}})

			p2 = bn.AddProduction(Production{
				ID: "p1",
				When: []AliasDeclaration{
					{
						Alias: "a",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("blue"),
								TestOp:    TestOpEqual,
							},
						},
					},
					{
						Alias: "b",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("red"),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
				Match: []JoinTest{
					{
						Alias:  []Selector{{"b", "Rank"}, {"a", "Rank"}},
						TestOp: TestOpLess,
					},
				},
			})
		})

		It("can share alpha memory even though the alias is not the same", func() {
			pj0x := p0.Parent().Parent().Parent().(*JoinNode).amem
			pj0y := p0.Parent().Parent().Parent().Parent().Parent().(*JoinNode).amem

			pj1x := p1.Parent().Parent().Parent().(*JoinNode).amem
			pj1y := p1.Parent().Parent().Parent().Parent().Parent().(*JoinNode).amem

			Expect(pj1x).Should(BeIdenticalTo(pj0x))
			Expect(pj1y).Should(BeIdenticalTo(pj0y))

			// join node not shared since join test is not the same
			Expect(p0.Parent()).ShouldNot(BeIdenticalTo(p1.Parent()))
			// but beta memory for guards are shared
			Expect(p0.Parent().Parent()).Should(BeIdenticalTo(p1.Parent().Parent()))

			chesses := getTestFacts()
			addFacts()
			Expect(p0.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X": chesses[0], // B1
				"Y": chesses[1], // B2
			}))
			Expect(p1.Matches()).To(ConsistOf(map[GVIdentity]any{
				"a": chesses[2], // B2
				"b": chesses[1], // B3
			}))
		})

		It("can share BetaMem memory if for the same JoinTest and similiar AliasDeclaration", func() {
			pj1 := p1.Parent()
			pj2 := p2.Parent()

			Expect(pj1).Should(BeIdenticalTo(pj2))

			chesses := getTestFacts()
			addFacts()
			Expect(p1.Matches()).To(ConsistOf(map[GVIdentity]any{
				"a": chesses[2], // B2
				"b": chesses[1], // B3
			}))
			Expect(p2.Matches()).To(ConsistOf(map[GVIdentity]any{
				"a": chesses[2], // B1
				"b": chesses[1], // B2
			}))
		})

	})

	When("adding production and facts", func() {
		BeforeEach(func() {
			an = NewAlphaNetwork()
			Expect(an).NotTo(BeNil())
			bn = NewBetaNetwork(an)
			Expect(bn).NotTo(BeNil())
		})

		It("can add an production with guards only", func() {
			const guardsPrd = "production with guards only"
			p := Production{
				ID: guardsPrd,
				When: []AliasDeclaration{
					{
						Alias: "X",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("red"),
								TestOp:    TestOpEqual,
							},
							{
								AliasAttr: "Rank",
								Value:     GVInt(1),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
				Match: []JoinTest{},
			}
			pNode := bn.AddProduction(p)
			Expect(pNode.AnyMatches()).To(BeFalse())
			if Expect(pNode.Parent()).To(BeAssignableToTypeOf(&JoinNode{})) {
				if Expect(pNode.Parent().Parent()).To(BeAssignableToTypeOf(&BetaMem{})) {
					dummyBm := pNode.Parent().Parent().(*BetaMem)
					Expect(dummyBm.Parent()).To(BeNil())
				}
			}

			addFacts()
			Expect(pNode.AnyMatches()).Should(BeTrue())
			pNode.items.ForEach(func(t *Token) {
				Expect(t.nodes.Contains(pNode)).Should(BeTrue())
			})

			hit := lo.Must(lo.Find(getTestFacts(), func(item *Chess) bool {
				return item.Rank == 1 && item.Color == "red"
			}))
			Expect(pNode.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X": hit,
			}))

			// now remove facts
			removeFacts(hit.ID)
			Expect(pNode.AnyMatches()).Should(BeFalse())

			// add it back
			addFacts()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			// now remove production
			bn.RemoveProduction(guardsPrd)
			Expect(pNode.AnyMatches()).Should(BeFalse())
			Expect(pNode.Parent()).Should(BeNil())
		})

		It("can add an production with join tests", func() {
			const joinTestsPrd = "production with join tests"
			p := Production{
				ID: joinTestsPrd,
				When: []AliasDeclaration{
					{
						Alias: "X",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("red"),
								TestOp:    TestOpEqual,
							},
						},
					},
					{
						Alias: "Y",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("blue"),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
				Match: []JoinTest{
					{
						Alias:  []Selector{{"X", "On"}, {"Y", FieldSelf}},
						TestOp: TestOpEqual,
					},
				},
			}
			pNode := bn.AddProduction(p)

			Expect(pNode.AnyMatches()).To(BeFalse())
			if Expect(pNode.Parent()).To(BeAssignableToTypeOf(&JoinNode{})) {
				// node of join test
				joinTestNode := pNode.Parent().(*JoinNode)
				Expect(joinTestNode.amem).Should(BeNil())
				Expect(joinTestNode.Parent()).To(BeAssignableToTypeOf(&BetaMem{}))
				// node of alias "Y"
				Expect(joinTestNode.Parent().Parent()).To(BeAssignableToTypeOf(&JoinNode{}))
			}

			addFacts()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			hit := lo.Must(lo.Find(getTestFacts(), func(item *Chess) bool {
				return item.Rank == 1 && item.Color == "red"
			}))
			Expect(pNode.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X": hit,
				"Y": hit.On,
			}))

			removeFacts(hit.ID)
			Expect(pNode.AnyMatches()).ShouldNot(BeTrue())

			addFacts()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			// now remove production
			bn.RemoveProduction(joinTestsPrd)
			Expect(pNode.AnyMatches()).Should(BeFalse())

			// add production back
			pNode = bn.AddProduction(p)
			addFacts()
			Expect(pNode.AnyMatches()).Should(BeTrue())
		})

		It("can add an production on the fly", func() {
			const joinTestsPrd = "production with join tests"
			p := Production{
				ID: joinTestsPrd,
				When: []AliasDeclaration{
					{
						Alias: "X",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("red"),
								TestOp:    TestOpEqual,
							},
						},
					},
					{
						Alias: "Y",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString("blue"),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
				Match: []JoinTest{
					{
						Alias:  []Selector{{"X", "On"}, {"Y", FieldSelf}},
						TestOp: TestOpEqual,
					},
				},
			}
			pNode := bn.AddProduction(p)
			addFacts()
			Expect(pNode.AnyMatches()).To(BeTrue())

			// add another production
			tablepNode := bn.AddProduction(Production{
				ID: "match table",
				When: []AliasDeclaration{
					{
						Alias: "T",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString(""),
								TestOp:    TestOpEqual,
							},
						},
					},
				},
			})
			Expect(tablepNode.AnyMatches()).Should(BeTrue())
		})

	})

})
