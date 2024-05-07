package rete

import (
	. "github.com/ccbhj/grete/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var testChesses = getTestChesses()
var testPlayer = getTestPlayer(testChesses)

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
		playerType = TypeInfo{
			T: GValueTypeStruct,
			Fields: map[string]GValueType{
				"Chess0": GValueTypeStruct,
				"Chess1": GValueTypeStruct,
			},
		}
		addChesses  func()
		addPlayers  func()
		removeFacts func(ids ...GVIdentity)
	)

	BeforeEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
		bn = NewBetaNetwork(an)
		Expect(bn).NotTo(BeNil())

		addChesses = func() {
			lo.ForEach(testChesses, func(item *Chess, _ int) {
				bn.AddFact(Fact{
					ID:    item.ID,
					Value: NewGVStruct(item),
				})
			})
		}

		addPlayers = func() {
			lo.ForEach(testPlayer, func(item *Player, _ int) {
				bn.AddFact(Fact{
					ID:    item.ID,
					Value: NewGVStruct(item),
				})
			})
		}

		removeFacts = func(ids ...GVIdentity) {
			facts := testChesses
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
						JoinTests: []JoinTest{
							{
								XAttr:  "Rank",
								Y:      Selector{"X", "Rank"},
								TestOp: TestOpGreater,
							},
						},
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
						JoinTests: []JoinTest{
							{
								XAttr:  "Rank",
								Y:      Selector{"a", "Rank"},
								TestOp: TestOpGreater,
							},
						},
					},
				}})

			p2 = bn.AddProduction(Production{
				ID: "p2",
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
						JoinTests: []JoinTest{
							{
								XAttr:  "Rank",
								Y:      Selector{"a", "Rank"},
								TestOp: TestOpLess,
							},
						},
					},
				},
			})
		})

		It("can share alpha memory", func() {
			pj0x := p0.Parent().(*JoinNode).amem
			pj0y := p0.Parent().Parent().Parent().(*JoinNode).amem

			pj1x := p1.Parent().(*JoinNode).amem
			pj1y := p1.Parent().Parent().Parent().(*JoinNode).amem

			// alias order is different from p0 in p2
			pj2y := p2.Parent().(*JoinNode).amem
			pj2x := p2.Parent().Parent().Parent().(*JoinNode).amem

			Expect(pj1x).Should(BeIdenticalTo(pj0x), "even though the alias is not the same")
			Expect(pj1y).Should(BeIdenticalTo(pj0y), "even though the alias is not the same")

			Expect(pj2x).Should(BeIdenticalTo(pj0x), "even though the alias is not the same")
			Expect(pj2y).Should(BeIdenticalTo(pj0y), "even though the alias is not the same")

			Expect(pj2x).Should(BeIdenticalTo(pj0x), "even though the alias' declaration order is not the same")
			Expect(pj2y).Should(BeIdenticalTo(pj0y), "even though the alias' declaration order is not the same")

			Expect(p0.Parent()).Should(BeIdenticalTo(p1.Parent()), "join node should be shared too, since they generate the same TestAtJoinNode")
		})

		It("can share beta memory with similiar join test", func() {
			p0BM := p0.Parent().Parent().(*BetaMem)
			p1BM := p1.Parent().Parent().(*BetaMem)
			Expect(p0BM).Should(BeIdenticalTo(p1BM))
		})

		It("can match the right wmes", func() {
			chesses := getTestChesses()
			addChesses()
			Expect(p0.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X": chesses[0], // B1
				"Y": chesses[1], // B2
			}))
			Expect(p1.Matches()).To(ConsistOf(map[GVIdentity]any{
				"a": chesses[0], // B1
				"b": chesses[1], // B2
			}))
			Expect(p2.Matches()).To(ConsistOf(map[GVIdentity]any{
				"a": chesses[1], // B1
				"b": chesses[0], // B2
			}))
		})
	})

	When("adding/removing production and facts", func() {
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
			}
			pNode := bn.AddProduction(p)
			Expect(pNode.AnyMatches()).To(BeFalse())
			if Expect(pNode.Parent()).To(BeAssignableToTypeOf(&JoinNode{})) {
				if Expect(pNode.Parent().Parent()).To(BeAssignableToTypeOf(&BetaMem{})) {
					dummyBm := pNode.Parent().Parent().(*BetaMem)
					Expect(dummyBm.Parent()).To(BeNil())
				}
			}

			addChesses()
			Expect(pNode.AnyMatches()).Should(BeTrue())
			pNode.items.ForEach(func(t *Token) {
				Expect(t.node).Should(BeIdenticalTo(pNode))
			})

			hit := lo.Must(lo.Find(getTestChesses(), func(item *Chess) bool {
				return item.Rank == 1 && item.Color == "red"
			}))
			Expect(pNode.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X": hit,
			}))

			// now remove facts
			removeFacts(hit.ID)
			Expect(pNode.AnyMatches()).Should(BeFalse())

			// add it back
			addChesses()
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
						JoinTests: []JoinTest{
							{
								XAttr:  FieldSelf,
								Y:      Selector{"X", "On"},
								TestOp: TestOpEqual,
							},
						},
					},
					{
						Alias: "Winner",
						Type:  playerType,
						JoinTests: []JoinTest{
							{
								XAttr: "Chess0",
								Y: Selector{
									Alias:     "X",
									AliasAttr: FieldSelf,
								},
								TestOp: TestOpEqual,
							},
							{
								XAttr: "Chess1",
								Y: Selector{
									Alias:     "Y",
									AliasAttr: FieldSelf,
								},
								TestOp: TestOpEqual,
							},
						},
					},
				},
			}
			pNode := bn.AddProduction(p)

			Expect(pNode.AnyMatches()).To(BeFalse())
			if Expect(pNode.Parent()).To(BeAssignableToTypeOf(&JoinNode{})) {
				// node of join test
				joinTestNode := pNode.Parent().(*JoinNode)
				Expect(joinTestNode.amem).ShouldNot(BeNil())
				Expect(joinTestNode.Parent()).To(BeAssignableToTypeOf(&BetaMem{}))
				// node of alias "Y"
				Expect(joinTestNode.Parent().Parent()).To(BeAssignableToTypeOf(&JoinNode{}))
			}

			addChesses()
			addPlayers()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			hit := lo.Must(lo.Find(testChesses, func(item *Chess) bool {
				return item.Rank == 1 && item.Color == "red"
			}))
			winner := lo.Must(lo.Find(testPlayer, func(item *Player) bool {
				return item.Chess0 == hit
			}))
			Expect(pNode.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X":      hit,
				"Y":      hit.On,
				"Winner": winner,
			}))

			removeFacts(hit.ID)
			Expect(pNode.AnyMatches()).ShouldNot(BeTrue())

			addChesses()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			// now remove production
			bn.RemoveProduction(joinTestsPrd)
			Expect(pNode.AnyMatches()).Should(BeFalse())

			// add production back
			pNode = bn.AddProduction(p)
			addChesses()
			addPlayers()
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
						JoinTests: []JoinTest{
							{
								XAttr:  FieldSelf,
								Y:      Selector{"X", "On"},
								TestOp: TestOpEqual,
							},
						},
					},
				},
			}
			pNode := bn.AddProduction(p)
			addChesses()
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

	When("adding negative join test", Label("NegativeNode"), func() {

		BeforeEach(func() {
			an = NewAlphaNetwork()
			Expect(an).NotTo(BeNil())
			bn = NewBetaNetwork(an)
			Expect(bn).NotTo(BeNil())
		})

		It("can add an production with negative join tests", func() {
			const joinTestsPrd = "no chesses with Rank==1 is on table"
			p := Production{
				ID: joinTestsPrd,
				When: []AliasDeclaration{
					{
						Alias: "TABLE",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Color",
								Value:     GVString(""),
								TestOp:    TestOpEqual,
							},
						},
					},
					{
						Alias: "X",
						Type:  tf,
						Guards: []Guard{
							{
								AliasAttr: "Rank",
								Value:     GVInt(1),
								TestOp:    TestOpEqual,
							},
						},
						NJoinTest: []JoinTest{
							{
								XAttr:  "On",
								Y:      Selector{"TABLE", FieldSelf},
								TestOp: TestOpEqual,
							},
						},
					},
				},
			}
			pNode := bn.AddProduction(p)

			Expect(pNode.AnyMatches()).To(BeFalse())
			if Expect(pNode.Parent()).To(BeAssignableToTypeOf(&NegativeNode{})) {
				// node of join test
				nnode := pNode.Parent().(*NegativeNode)
				Expect(nnode.amem).ShouldNot(BeNil())
				Expect(nnode.Parent()).To(BeAssignableToTypeOf(&JoinNode{}))
				Expect(nnode.Parent().Parent()).To(BeAssignableToTypeOf(&BetaMem{}))
			}

			addChesses()
			addPlayers()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			hit := lo.Must(lo.Find(testChesses, func(item *Chess) bool {
				return item.Rank == 1
			}))
			table := testChesses[3]
			Expect(pNode.Matches()).To(ConsistOf(map[GVIdentity]any{
				"X":     hit,
				"TABLE": table,
			}))

			removeFacts(hit.ID)
			Expect(pNode.AnyMatches()).ShouldNot(BeTrue())

			addChesses()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			// now remove production
			bn.RemoveProduction(joinTestsPrd)
			Expect(pNode.AnyMatches()).Should(BeFalse())

			// add production back
			pNode = bn.AddProduction(p)
			addChesses()
			addPlayers()
			Expect(pNode.AnyMatches()).Should(BeTrue())

			bn.AddFact(Fact{
				ID: "B4",
				Value: NewGVStruct(&Chess{
					ID:    "B4",
					Color: "red",
					On:    table,
					Rank:  1,
				}),
			})
			Expect(pNode.AnyMatches()).Should(BeFalse())
		})
	})

})
