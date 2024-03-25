package rete

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

var _ = Describe("JoinTest", func() {
	var (
		chessTab *Chess
		chessX   *Chess
		toWME    = func(c *Chess) *WME {
			return &WME{
				ID:    c.ID,
				Value: NewTVStruct(c),
			}
		}
	)
	BeforeEach(func() {
		chessTab = &Chess{
			ID: "table",
		}
		chessX = &Chess{
			ID:    "x",
			Color: "red",
			On:    chessTab,
			Rank:  10,
		}
	})

	When("building join test", func() {
		It("won't generate join test for constant testing without duplicated alias", func() {
			c1 := Cond{
				Alias:     "X",
				AliasAttr: "Color",
				Value:     TVString("red"),
			}
			c2 := Cond{
				Alias:     "Y",
				AliasAttr: "On",
				Value:     TVString("table"),
			}

			Expect(buildJoinTestFromConds(c1, nil)).Should(BeEmpty())
			Expect(buildJoinTestFromConds(c2, []Cond{c1})).Should(BeEmpty())
		})

		It("can generate join tests for conds with same alias", func() {
			c1 := Cond{
				Alias:     "TABLE",
				AliasAttr: "Rank",
				Value:     TVInt(0),
			}
			c2 := Cond{
				Alias:     "TABLE",
				AliasAttr: "On",
				Value:     NewTVNil(),
			}
			c3 := Cond{
				Alias:     "TABLE",
				AliasAttr: "Color",
				Value:     TVString(""),
			}
			Expect(buildJoinTestFromConds(c1, nil)).Should(BeEmpty())
			jts := buildJoinTestFromConds(c2, []Cond{c1})
			if Expect(jts).Should(HaveLen(1)) {
				Expect(jts[0].performTest(toWME(chessTab), toWME(chessTab))).Should(BeTrue())
			}

			jts = buildJoinTestFromConds(c3, []Cond{c1, c2})
			if Expect(jts).Should(HaveLen(2)) {
				Expect(jts[0]).To(Equal(&TestAtJoinNode{
					LhsAttr:    FieldID,
					RhsAttr:    FieldID,
					CondOffRhs: 0,
					TestOp:     TestOpEqual,
				}))
				Expect(jts[0].performTest(toWME(chessTab), toWME(chessTab))).Should(BeTrue())
				Expect(jts[1]).To(Equal(&TestAtJoinNode{
					LhsAttr:    FieldID,
					RhsAttr:    FieldID,
					CondOffRhs: 1,
					TestOp:     TestOpEqual,
				}))
				Expect(jts[1].performTest(toWME(chessTab), toWME(chessTab))).Should(BeTrue())
			}
		})

		It("can generate join tests between one cond's ID and previous cond's Value", func() {
			c1 := Cond{
				Alias:     "X",
				AliasAttr: "On",
				Value:     TVIdentity("TABLE"),
			}
			c2 := Cond{
				Alias:     "TABLE",
				AliasAttr: "Color",
				Value:     TVString(""),
			}
			Expect(buildJoinTestFromConds(c1, nil)).Should(BeEmpty())
			jts := buildJoinTestFromConds(c2, []Cond{c1})
			if Expect(jts).Should(HaveLen(1)) {
				jt := jts[0]
				Expect(jt).To(Equal(&TestAtJoinNode{
					LhsAttr:      FieldSelf,
					RhsAttr:      "On",
					CondOffRhs:   0,
					ReverseOrder: true,
				}))
				Expect(jt.performTest(toWME(chessTab), toWME(chessX)))
			}
		})

		It("can generate join tests between one cond's Value and previous cond's ID", func() {
			c1 := Cond{
				Alias:     "TABLE",
				AliasAttr: "Color",
				Value:     TVString(""),
			}
			c2 := Cond{
				Alias:     "X",
				AliasAttr: "On",
				Value:     TVIdentity("TABLE"),
			}
			Expect(buildJoinTestFromConds(c1, nil)).Should(BeEmpty())
			jts := buildJoinTestFromConds(c2, []Cond{c1})
			if Expect(jts).Should(HaveLen(1)) {
				jt := jts[0]
				Expect(jt).To(Equal(&TestAtJoinNode{
					LhsAttr:    "On",
					RhsAttr:    FieldSelf,
					CondOffRhs: 0,
				}))
				Expect(jt.performTest(toWME(chessX), toWME(chessTab))).To(BeTrue())
			}
		})
	})
})

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

	Describe("construction", func() {
		AfterEach(func() {
			an = NewAlphaNetwork()
			Expect(an).NotTo(BeNil())
			bn = NewBetaNetwork(an)
			Expect(bn).NotTo(BeNil())
		})

		When("adding production", func() {
			It("can add an single production", func() {
				pNode := bn.AddProduction("test", Cond{
					Alias:     "x",
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
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
						Alias:     TVIdentity("x"),
						AliasAttr: "On",
						Value:     TVIdentity("y"),
						TestOp:    TestOpEqual,
					},
					Cond{
						Alias:     TVIdentity("y"),
						AliasAttr: "LeftOf",
						Value:     TVIdentity("z"),
						TestOp:    TestOpEqual,
					},
					Cond{
						Alias:     TVIdentity("z"),
						AliasAttr: "Color",
						Value:     TVString("red"),
						TestOp:    TestOpEqual,
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
						Alias:     TVIdentity("z"),
						AliasAttr: "Color",
						Value:     TVString("red"),
						TestOp:    TestOpEqual,
					})

				py := bn.AddProduction(ruleID,
					Cond{
						Alias:     TVIdentity("z"),
						AliasAttr: "Color",
						Value:     TVString("red"),
						TestOp:    TestOpEqual,
					})
				Expect(px).To(BeEquivalentTo(py))

				pNode := bn.GetProduction(ruleID)
				Expect(pNode).To(BeIdenticalTo(px))
			})

			It("can share same beta memory", func() {
				pNodeX := bn.AddProduction("X",
					Cond{
						Alias:     TVIdentity("x"),
						AliasAttr: "On",
						Value:     TVIdentity("y"),
						TestOp:    TestOpEqual,
					},
					Cond{
						Alias:     TVIdentity("y"),
						AliasAttr: "LeftOf",
						Value:     TVIdentity("z"),
						TestOp:    TestOpEqual,
					},
					Cond{
						Alias:     TVIdentity("z"),
						AliasAttr: "Color",
						Value:     TVString("red"),
						TestOp:    TestOpEqual,
					})

				pNodeY := bn.AddProduction("Y",
					Cond{
						Alias:     TVIdentity("x"),
						AliasAttr: "On",
						Value:     TVIdentity("y"),
						TestOp:    TestOpEqual,
					},
					Cond{
						Alias:     TVIdentity("y"),
						AliasAttr: "LeftOf",
						Value:     TVIdentity("z"),
						TestOp:    TestOpEqual,
					},
					Cond{ // diferent rule here
						Alias:     TVIdentity("z"),
						AliasAttr: "Color",
						Value:     TVString("blue"),
						TestOp:    TestOpEqual,
					})

				Expect(pNodeX.Parent().Parent()).To(BeIdenticalTo(pNodeY.Parent().Parent()))
			})
		})

		When("removing production", func() {
			var (
				testChesses    []*Chess
				testChessFacts []Fact
				redChesses     []*Chess
				redChessFacts  []Fact
			)
			BeforeEach(func() {
				testChesses = getTestFacts()
				testChessFacts = lo.Map(testChesses, func(item *Chess, index int) Fact {
					return Fact{
						ID:    item.ID,
						Value: NewTVStruct(item),
					}
				})
				redChesses = lo.Filter(testChesses, func(item *Chess, index int) bool {
					return item.Color == "red"
				})
				redChessFacts = lo.Map(redChesses, func(item *Chess, index int) Fact {
					return Fact{
						ID:    item.ID,
						Value: NewTVStruct(item),
					}
				})
			})

			AfterEach(func() {
				an = NewAlphaNetwork()
				Expect(an).NotTo(BeNil())
				bn = NewBetaNetwork(an)
				Expect(bn).NotTo(BeNil())
			})

			It("can remove a single rule", func() {
				const ruleName = "single_rule"
				singleRule := bn.AddProduction(ruleName, Cond{
					Alias:     "X",
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
				})
				lo.ForEach(redChessFacts, ignoreIndex(bn.AddFact))
				p := singleRule.Parent()
				Expect(p.AnyChild()).Should(BeTrue())
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
					Alias:     "x",
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
				})
				Expect(singleRule).ShouldNot(BeNil())
				p = singleRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())

				lo.ForEach(redChessFacts, ignoreIndex(bn.AddFact))
				Expect(singleRule.Matches()).ShouldNot(BeEmpty())
			})

			It("can remove a multi-rule production and add again", func() {
				const ruleName = "multi_rule"
				rules := []Cond{
					{
						Alias:     TVIdentity("x"),
						AliasAttr: "On",
						Value:     TVIdentity("y"),
						TestOp:    TestOpEqual,
					},
					{
						Alias:     TVIdentity("y"),
						AliasAttr: "LeftOf",
						Value:     TVIdentity("z"),
						TestOp:    TestOpEqual,
					},
					{
						Alias:     TVIdentity("z"),
						AliasAttr: "Color",
						Value:     TVString("red"),
						TestOp:    TestOpEqual,
					}}
				multiRule := bn.AddProduction(ruleName, rules...)

				p := multiRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())
				lo.ForEach(testChessFacts, ignoreIndex(bn.AddFact))
				Expect(multiRule.Matches()).ShouldNot(BeEmpty())

				// now remove it
				Expect(bn.RemoveProduction(ruleName)).Should(BeNil())
				Expect(p.AnyChild()).Should(BeFalse())
				Expect(collect(p.ForEachChild)).Should(BeEmpty())
				Expect(multiRule.Matches()).Should(BeEmpty())

				// now add it again...
				multiRule = bn.AddProduction(ruleName, Cond{
					Alias:     "X",
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
				})
				Expect(multiRule).ShouldNot(BeNil())
				p = multiRule.Parent()
				Expect(p.AnyChild()).ShouldNot(BeFalse())
				Expect(collect(p.ForEachChild)).ShouldNot(BeEmpty())

				lo.ForEach(testChessFacts, ignoreIndex(bn.AddFact))
				Expect(multiRule.Matches()).ShouldNot(BeEmpty())
			})

			When("removing negative production", func() {
				It("can remove a single rule negative production", func() {
					const singleNegativeRule = "single_negative"
					singleRule := bn.AddProduction(singleNegativeRule, Cond{
						Alias:     "X",
						AliasAttr: "Color",
						Value:     TVString("red"),
						Negative:  true,
					})
					lo.ForEach(testChessFacts, ignoreIndex(bn.AddFact))
					Expect(singleRule.AnyMatches()).To(BeTrue())
					matches, err := singleRule.Matches()
					Expect(err).To(BeNil())
					Expect(matches).To(HaveLen(len(testChesses) - len(redChesses)))

					err = bn.RemoveProduction(singleNegativeRule)
					Expect(err).To(BeNil())
					Expect(singleRule.AnyMatches()).To(BeFalse())
					Expect(singleRule.Parent()).To(BeNil())
				})

				It("can remove a multi rule negative production", func() {
					const multiNegativeRule = "multi_negative"
					chesses := getTestFacts()
					multiRules := bn.AddProduction(multiNegativeRule,
						Cond{
							Alias:     "X",
							AliasAttr: FieldSelf,
							Value:     NewTVStruct(chesses[0]),
						},
						Cond{
							Alias:     "X",
							AliasAttr: "Color",
							Value:     TVIdentity("c1"),
						},
						Cond{
							Alias:     "Y",
							AliasAttr: "Color",
							Value:     TVIdentity("c2"),
						},
						Cond{
							Alias:     "c1",
							AliasAttr: FieldID,
							Value:     TVIdentity("c2"),
							Negative:  true,
						},
					)

					lo.ForEach(
						lo.Map(chesses[:3], func(c *Chess, i int) Fact {
							return Fact{ID: c.ID, Value: NewTVStruct(c)}
						}),
						ignoreIndex(bn.AddFact))
					Expect(multiRules.AnyMatches()).To(BeTrue())
					matches, err := multiRules.Matches()
					Expect(err).To(BeNil())
					Expect(matches).To(HaveLen(3))

					err = bn.RemoveProduction(multiNegativeRule)
					Expect(err).To(BeNil())
					Expect(multiRules.AnyMatches()).To(BeFalse())
					Expect(multiRules.Parent()).To(BeNil())
					Expect(multiRules.items.Len()).To(BeZero())
				})

			})
		})
	})

	When("adding/removing facts", func() {
		var (
			singleRule         *PNode
			multiRule          *PNode
			testChesses        []*Chess
			signleNegativeRule *PNode
			multiNegativeRule  *PNode
			getTestChess       = func() []*Chess {
				var chess [4]*Chess
				for i := 0; i < len(chess); i++ {
					chess[i] = new(Chess)
				}

				*chess[0] = Chess{
					ID:    "B1",
					Color: "red",
					On:    chess[1],
				}
				*chess[1] = Chess{
					ID:     "B2",
					Color:  "blue",
					LeftOf: chess[2],
					On:     chess[2],
				}
				*chess[2] = Chess{
					ID:    "B3",
					On:    chess[3],
					Color: "red",
				}
				*chess[3] = Chess{
					ID: "table",
				}
				return chess[:]
			}
		)

		BeforeEach(func() {
			testChesses = getTestChess()
			singleRule = bn.AddProduction("single_rule", Cond{
				Alias:     "x",
				AliasAttr: "Color",
				Value:     TVString("red"),
				TestOp:    TestOpEqual,
			})
			signleNegativeRule = bn.AddProduction("single_negative_rule", Cond{
				Alias:     "x",
				AliasAttr: "Color",
				Value:     TVString("red"),
				TestOp:    TestOpEqual,
				Negative:  true,
			})

			multiRule = bn.AddProduction("multi_rule",
				Cond{
					Alias:     TVIdentity("table"),
					AliasAttr: "On",
					Value:     NewTVNil(),
					TestOp:    TestOpEqual,
				},
				Cond{
					Alias:     TVIdentity("x"),
					AliasAttr: "On",
					Value:     TVIdentity("y"),
					TestOp:    TestOpEqual,
				},
				Cond{
					Alias:     TVIdentity("y"),
					AliasAttr: "LeftOf",
					Value:     TVIdentity("z"),
					TestOp:    TestOpEqual,
				},
				Cond{
					Alias:     TVIdentity("z"),
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
				},
				Cond{
					Alias:     TVIdentity("z"),
					AliasAttr: "On",
					Value:     TVIdentity("table"),
					TestOp:    TestOpEqual,
				},
			)
			multiNegativeRule = bn.AddProduction("multi_negative_rule",
				Cond{
					Alias:     "x",
					AliasAttr: "On",
					Value:     TVIdentity("y"),
					TestOp:    TestOpEqual,
				},
				Cond{
					Alias:     "TABLE",
					AliasAttr: "Color",
					Value:     TVString(""),
					TestOp:    TestOpEqual,
				},
				Cond{
					Alias:     "x",
					AliasAttr: "Color",
					Value:     TVString(""),
					TestOp:    TestOpEqual,
					Negative:  true,
				},
				Cond{
					Alias:     "y",
					AliasAttr: FieldSelf,
					Value:     TVIdentity("TABLE"),
					TestOp:    TestOpEqual,
					Negative:  true,
				})

			lo.ForEach(testChesses, func(c *Chess, _ int) {
				bn.AddFact(Fact{
					ID:    TVIdentity(c.ID),
					Value: NewTVStruct(c),
				})
			})
		})

		AfterEach(func() {
			an = NewAlphaNetwork()
			Expect(an).NotTo(BeNil())
			bn = NewBetaNetwork(an)
			Expect(bn).NotTo(BeNil())
		})

		Context("matching facts with single rule", func() {
			var (
				redChess []*Chess
				matches  []map[TVIdentity]any
				err      error
			)
			BeforeEach(func() {
				redChess = lo.Filter(testChesses, func(f *Chess, idx int) bool {
					return f.Color == "red"
				})
				matches, err = singleRule.Matches()
				Expect(err).To(BeNil())
			})

			It("should match some facts", func() {
				Expect(matches).Should(HaveLen(len(redChess)))
				for _, fact := range redChess {
					Expect(matches).To(ContainElement(map[TVIdentity]any{
						"x": fact,
					}))
				}
			})

			It("should still match some facts after removing one", func() {
				bn.RemoveFact(Fact{
					ID:    TVIdentity(redChess[0].ID),
					Value: NewTVStruct(redChess[0]),
				})
				for _, chess := range redChess[:] {
					Expect(matches).To(ContainElement(map[TVIdentity]any{
						"x": chess,
					}))
				}
			})

			It("should not match any facts after removing all", func() {
				for _, fact := range redChess {
					bn.RemoveFact(Fact{
						ID:    TVIdentity(fact.ID),
						Value: NewTVStruct(fact),
					})
				}
				Expect(singleRule.AnyMatches()).Should(BeFalse())
			})
		})

		Context("can match facts with multi rules", func() {
			It("should match some facts", func() {
				Expect(multiRule.AnyMatches()).To(BeTrue())
				matches, err := multiRule.Matches()
				Expect(err).To(BeNil())
				Expect(matches).To(HaveLen(1))
				Expect(matches[0]).Should(And(
					HaveKeyWithValue(TVIdentity("x"), testChesses[0]),
					HaveKeyWithValue(TVIdentity("y"), testChesses[1]),
					HaveKeyWithValue(TVIdentity("z"), testChesses[2]),
					HaveKeyWithValue(TVIdentity("table"), testChesses[3]),
				))
			})

			It("should not match any facts after removing one of the matched fact", func() {
				f := Fact{ID: TVIdentity(testChesses[3].ID), Value: NewTVStruct(testChesses[3])}
				bn.RemoveFact(f)
				Expect(multiRule.AnyMatches()).Should(BeFalse())

				bn.AddFact(f)
				Expect(multiRule.AnyMatches()).Should(BeTrue())
				matches, err := multiRule.Matches()
				Expect(err).To(BeNil())
				Expect(matches).To(HaveLen(1))
				Expect(matches[0]).Should(And(
					HaveKeyWithValue(TVIdentity("x"), testChesses[0]),
					HaveKeyWithValue(TVIdentity("y"), testChesses[1]),
					HaveKeyWithValue(TVIdentity("z"), testChesses[2]),
					HaveKeyWithValue(TVIdentity("table"), testChesses[3]),
				))
			})
		})

		Context("can match facts with negative rules", func() {
			It("should match some facts for single rule", func() {
				matches, err := signleNegativeRule.Matches()
				Expect(err).To(BeNil())
				nonRedChesses := lo.Filter(testChesses, func(item *Chess, index int) bool {
					return item.Color != "red"
				})
				Expect(matches).Should(HaveLen(len(nonRedChesses)))
				for _, chess := range nonRedChesses {
					Expect(matches).To(ContainElement(map[TVIdentity]any{
						"x": chess,
					}))
				}
			})

			It("should match some facts for multi rules", func() {
				Expect(multiNegativeRule.AnyMatches()).To(BeTrue())
				matches, err := multiNegativeRule.Matches()
				Expect(err).To(BeNil())
				for item := range multiNegativeRule.items {
					GinkgoLogr.Info("match", item)
				}
				Expect(matches).To(HaveLen(2))
				Expect(matches[0]).Should(Or(
					And(
						HaveKeyWithValue(TVIdentity("x"), testChesses[0]),
						HaveKeyWithValue(TVIdentity("y"), testChesses[1]),
						HaveKeyWithValue(TVIdentity("TABLE"), testChesses[3])),
					And(
						HaveKeyWithValue(TVIdentity("x"), testChesses[1]),
						HaveKeyWithValue(TVIdentity("y"), testChesses[2]),
						HaveKeyWithValue(TVIdentity("TABLE"), testChesses[3]),
					)))
				Expect(matches[1]).Should(Or(
					And(
						HaveKeyWithValue(TVIdentity("x"), testChesses[0]),
						HaveKeyWithValue(TVIdentity("y"), testChesses[1]),
						HaveKeyWithValue(TVIdentity("TABLE"), testChesses[3])),
					And(
						HaveKeyWithValue(TVIdentity("x"), testChesses[1]),
						HaveKeyWithValue(TVIdentity("y"), testChesses[2]),
						HaveKeyWithValue(TVIdentity("TABLE"), testChesses[3]),
					)))

				bn.RemoveFact(Fact{ID: testChesses[0].ID, Value: NewTVStruct(testChesses[0])})
				Expect(multiNegativeRule.Matches()).To(HaveLen(1))
				bn.AddFact(Fact{ID: testChesses[0].ID, Value: NewTVStruct(testChesses[0])})
				Expect(multiNegativeRule.Matches()).To(HaveLen(2))
			})
		})
	})
})

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
