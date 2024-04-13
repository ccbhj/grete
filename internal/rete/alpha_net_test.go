package rete

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	. "github.com/ccbhj/grete/internal/types"
)

type Chess struct {
	ID     GVIdentity
	Color  string
	On     *Chess
	LeftOf *Chess
	Rank   int
}

func getTestFacts() []*Chess {
	var b1, b2, b3, table Chess
	b1 = Chess{
		ID:    "B1",
		Color: "red",
		On:    &b2,
		Rank:  1,
	}
	b2 = Chess{
		ID:     "B2",
		Color:  "blue",
		LeftOf: &b3,
		On:     &table,
		Rank:   2,
	}
	b3 = Chess{
		ID:    "B3",
		On:    &table,
		Color: "red",
		Rank:  3,
	}
	table = Chess{
		ID: "table",
	}
	return []*Chess{&b1, &b2, &b3, &table}
}

var _ = Describe("AlphaNet", func() {
	var (
		an *AlphaNetwork
	)
	BeforeEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
	})

	Describe("fact hashing", func() {
		It("hash by id and value", func() {
			Expect(Fact{ID: "X", Value: GVString("FOO")}.Hash()).
				Should(BeEquivalentTo(firstArg(Fact{ID: "X", Value: GVString("FOO")}.Hash())))
			Expect(Fact{ID: "X", Value: GVString("FOO")}.Hash()).
				ShouldNot(BeEquivalentTo(firstArg(Fact{ID: "Y", Value: GVString("FOO")}.Hash())))
			Expect(Fact{ID: "X", Value: GVString("FOO")}.Hash()).
				ShouldNot(BeEquivalentTo(firstArg(Fact{ID: "X", Value: GVInt(1)}.Hash())))

			Expect(Fact{ID: "X", Value: NewGVStruct(struct{ Foo string }{"FOO"})}.Hash()).
				Should(BeEquivalentTo(firstArg(Fact{ID: "X", Value: NewGVStruct(struct{ Foo string }{"FOO"})}.Hash())))
			Expect(Fact{ID: "X", Value: NewGVStruct(struct{ Foo string }{"FOO"})}.Hash()).
				ShouldNot(BeEquivalentTo(firstArg(Fact{ID: "X", Value: NewGVStruct(struct{ Foo string }{"BAR"})}.Hash())))
			// compare struct
			Expect(Fact{ID: "X", Value: NewGVStruct(struct{ Foo string }{"FOO"})}.Hash()).
				Should(BeEquivalentTo(firstArg(Fact{ID: "X", Value: NewGVStruct(struct{ Bar string }{"FOO"})}.Hash())))

			// compare pointer
			Expect(Fact{ID: "X", Value: NewGVStruct(&struct{ Foo string }{"FOO"})}.Hash()).
				ShouldNot(BeEquivalentTo(firstArg(Fact{ID: "X", Value: NewGVStruct(&struct{ Bar string }{"FOO"})}.Hash())))
		})
	})

	Describe("performing type checking", func() {
		It("should check the constant TestValueType", func() {
			Expect(NewTypeTestNode(nil, TypeInfo{T: GValueTypeString}).PerformTest(&WME{ID: "X", Value: GVString("bar")})).Should(BeTrue())

			Expect(NewTypeTestNode(nil, TypeInfo{T: GValueTypeString}).PerformTest(&WME{ID: "X", Value: GVInt(1)})).Should(BeFalse())

		})

		Context("check the value type of TVStruct", func() {
			It("should check the TestValueType of struct", func() {
				Expect(NewTypeTestNode(nil, TypeInfo{
					T: GValueTypeStruct,
				}).PerformTest(&WME{ID: "X", Value: GVString("")})).Should(BeFalse())
			})

			It("should check the struct type strictly if reflect.Type is provided ", func() {
				n := NewTypeTestNode(nil, TypeInfo{
					T:  GValueTypeStruct,
					VT: reflect.TypeOf(Chess{}),
				})
				Expect(n.PerformTest(&WME{ID: "X", Value: GVString("")})).Should(BeFalse())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(Chess{})})).Should(BeTrue())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(&Chess{})})).Should(BeTrue())

				type _Chess struct {
					ID     GVIdentity
					On     GVString
					Color  GVString
					LeftOf GVIdentity
				}
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(_Chess{})})).Should(BeFalse())
			})

			It("could check if value have the required fields", func() {
				n := NewTypeTestNode(nil, TypeInfo{
					T: GValueTypeStruct,
					Fields: map[string]GValueType{
						"Color": GValueTypeString,
					},
				})
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(struct{ Color GVString }{"red"})})).Should(BeTrue())
				// string type also works too
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(struct{ Color string }{"red"})})).Should(BeTrue())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(struct{ On string }{"Y"})})).Should(BeFalse())
			})

			It("could check by the reflection type", func() {
				n := NewTypeTestNode(nil, TypeInfo{
					T:  GValueTypeStruct,
					VT: reflect.TypeOf(Chess{}),
				})

				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(struct{ Color GVString }{"red"})})).Should(BeFalse())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewGVStruct(Chess{})})).Should(BeTrue())
			})
		})
	})

	Describe("constructing AlphaNetwork", func() {
		var tf TypeInfo

		BeforeEach(func() {
			tf = TypeInfo{
				T: GValueTypeStruct,
				Fields: map[string]GValueType{
					"On":    GValueTypeString,
					"Color": GValueTypeString,
				},
			}
		})

		It("allowed the same condition to be added for more than one time", func() {
			am := an.MakeAlphaMem(tf, []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("red"),
					TestOp:    TestOpEqual,
				},
			})
			Expect(am).NotTo(BeNil())
			Expect(an.AlphaRoot()).NotTo(BeNil())

			otherAM := an.MakeAlphaMem(tf, []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("red"),
					TestOp:    TestOpEqual,
				},
			})
			Expect(otherAM).Should(BeIdenticalTo(am))
		})

		When("testing condition with value of non-Identity type as tested Value", func() {
			var child, grandChild AlphaNode
			var am *AlphaMem
			BeforeEach(func() {
				am = an.MakeAlphaMem(tf, []Guard{
					{
						AliasAttr: "Color",
						Value:     GVString("red"),
						TestOp:    TestOpEqual,
					},
				})

				an.AlphaRoot().ForEachChild(func(tn AlphaNode) (stop bool) {
					child = tn
					return true
				})
				child.ForEachChild(func(tn AlphaNode) (stop bool) {
					grandChild = tn
					return true
				})
			})

			It("should construct a tree with depth of 3 ", func() {
				Expect(an.AlphaRoot()).NotTo(BeNil())
				if Expect(child).NotTo(BeNil()) {
					Expect(grandChild).NotTo(BeNil())
				}

				Expect(child).To(BeAssignableToTypeOf(&TypeTestNode{}))
				Expect(grandChild).To(BeAssignableToTypeOf(&ConstantTestNode{}))
			})

			It("grandchild should be the leaf and output to AlphaMem", func() {
				grandGrandChild := make([]AlphaNode, 0)
				grandChild.ForEachChild(func(an AlphaNode) (stop bool) {
					grandGrandChild = append(grandGrandChild, an)
					return false
				})
				Expect(grandGrandChild).To(BeEmpty())
				Expect(grandChild.OutputMem()).To(Equal(am))
			})

			It("should test the Attr or the Type of WME with TypeTestNode", func() {
				// Type not matched
				Expect(child.PerformTest(NewWME("X", GVString("!!!")))).To(BeFalse())
				// Attr not found
				Expect(child.PerformTest(NewWME("X", NewGVStruct(struct{ Background GVString }{"red"})))).To(BeFalse())
			})

			It("should test the Value of an attr of WME with ConstantTestNode", func() {
				// Value not matched
				Expect(grandChild.PerformTest(NewWME("X", NewGVStruct(struct{ Color GVString }{"blue"})))).To(BeFalse())
				Expect(grandChild.PerformTest(NewWME("X", NewGVStruct(struct{ Color GVString }{"red"})))).To(BeTrue())
			})
		})

		When("building negative constant cond", func() {
			var (
				child, grandChild, grandGrandChild AlphaNode
				am                                 *AlphaMem
				c                                  Guard
			)
			BeforeEach(func() {
				c = Guard{
					AliasAttr: "Color",
					Value:     GVString("red"),
					Negative:  true,
					TestOp:    TestOpEqual,
				}
				am = an.MakeAlphaMem(tf, []Guard{c})
				grandGrandChild = am.inputAlphaNode
				grandChild = grandGrandChild.Parent()
				child = grandChild.Parent()
			})

			It("should be a tree with depth to be 3", func() {
				Expect(grandGrandChild).To(BeAssignableToTypeOf(&NegativeTestNode{}))
				Expect(grandChild).To(BeAssignableToTypeOf(&ConstantTestNode{}))
				Expect(child).To(BeAssignableToTypeOf(&TypeTestNode{}))
				Expect(child.Parent()).To(BeIdenticalTo(an.root))
			})

			It("can share node with its positive node", func() {
				pCond := c
				pCond.Negative = false
				pAm := an.MakeAlphaMem(tf, []Guard{pCond})
				Expect(pAm.inputAlphaNode).To(BeIdenticalTo(grandChild))
				Expect(pAm.inputAlphaNode.Parent()).To(BeIdenticalTo(child))
			})
		})
	})

	Describe("deconstructing alpha mem", func() {
		var (
			am *AlphaMem
			tf TypeInfo
		)
		BeforeEach(func() {
			tf = TypeInfo{
				T: GValueTypeStruct,
				Fields: map[string]GValueType{
					"Rank":  GValueTypeInt,
					"Color": GValueTypeString,
				},
			}
			am = an.MakeAlphaMem(tf, []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("red"),
					TestOp:    TestOpEqual,
				},
			})
			lo.ForEach(getTestFacts(), func(item *Chess, _ int) {
				an.AddFact(Fact{
					ID:    item.ID,
					Value: NewGVStruct(item),
				})
			})
		})

		It("can deconstruct a stand-alone alpha mem", func() {
			constTestNode := am.inputAlphaNode
			typeNode := constTestNode.Parent()

			an.DestoryAlphaMem(am)
			Expect(am.an).Should(BeNil())
			Expect(am.inputAlphaNode).Should(BeNil())
			Expect(typeNode.IsParentOf(constTestNode)).Should(BeFalse())
			Expect(am.NItems()).Should(BeZero())
		})

		It("however won't deconstruct a share construct node", func() {
			newAM := an.MakeAlphaMem(tf, []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("blue"),
					TestOp:    TestOpEqual,
				},
			})
			inputNode := am.inputAlphaNode
			parent := inputNode.Parent()
			grandParent := inputNode.Parent().Parent()

			newInputNode := newAM.inputAlphaNode
			newParent := newInputNode.Parent()
			newGrandParent := newInputNode.Parent().Parent()
			Expect(newParent).Should(BeIdenticalTo(parent))
			Expect(newGrandParent).Should(BeIdenticalTo(grandParent))

			an.DestoryAlphaMem(am)
			Expect(am.an).Should(BeNil())
			Expect(am.inputAlphaNode).Should(BeNil())
			// old am parent's had been already cleaned
			Expect(parent.IsParentOf(inputNode)).Should(BeFalse())

			Expect(newInputNode.Parent()).Should(BeIdenticalTo(newParent))
			Expect(newParent.Parent()).Should(BeIdenticalTo(newGrandParent))
			Expect(newParent.IsParentOf(newInputNode)).Should(BeTrue())
			Expect(newGrandParent.IsParentOf(newParent)).Should(BeTrue())
			Expect(grandParent.IsParentOf(parent)).Should(BeTrue())
		})

		It("can destruct negative node without destructing the positive node", func() {
			negativeAm := an.MakeAlphaMem(tf, []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("red"),
					TestOp:    TestOpEqual,
					Negative:  true,
				},
			})
			n := am.NItems()
			Expect(n).ShouldNot(BeZero())
			an.DestoryAlphaMem(negativeAm)
			Expect(am.NItems()).Should(Equal(n))
		})
	})

	Describe("Adding fact", func() {
		var (
			testFacts []*Chess
			ams       []*AlphaMem
			fieldType = TypeInfo{
				T: GValueTypeStruct,
				Fields: map[string]GValueType{
					"Rank":  GValueTypeInt,
					"Color": GValueTypeString,
				},
			}
			conds = []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("red"),
					TestOp:    TestOpEqual,
				},
				{
					AliasAttr: "Rank",
					Value:     GVInt(10),
					TestOp:    TestOpEqual,
				},
				{
					AliasAttr: "Color",
					Value:     GVString(""),
					TestOp:    TestOpEqual,
					Negative:  true,
				},
			}
		)

		BeforeEach(func() {
			ams = make([]*AlphaMem, 0, len(conds))
			for _, c := range conds {
				am := an.MakeAlphaMem(fieldType, []Guard{c})
				Expect(am).NotTo(BeNil())
				ams = append(ams, am)
			}

			testFacts = getTestFacts()
			lo.ForEach(testFacts, func(item *Chess, _ int) {
				an.AddFact(Fact{ID: GVIdentity(item.ID), Value: NewGVStruct(item)})
			})
		})

		AfterEach(func() {
			testFacts = testFacts[:0]
		})

		It("should matched Facts with contant value and type", func() {
			redFacts := lo.Filter(testFacts, func(f *Chess, idx int) bool {
				return f.Color == "red"
			})
			am := ams[0]
			Expect(redFacts).To(HaveLen(am.NItems()))
			am.ForEachItem(func(w *WME) (stop bool) {
				Expect(redFacts).To(ContainElement(w.Value.(*GVStruct).Value()))
				return false
			})
		})

		When("performing negative testing", func() {
			var negativeAM *AlphaMem
			var positiveAM *AlphaMem

			BeforeEach(func() {
				negativeAM = ams[2]

				pc := conds[2]
				pc.Negative = false
				positiveAM = an.MakeAlphaMem(fieldType, []Guard{pc})
				lo.ForEach(testFacts, func(item *Chess, _ int) {
					an.AddFact(Fact{ID: GVIdentity(item.ID), Value: NewGVStruct(item)})
				})
			})

			It("can match wmes", func() {
				Expect(negativeAM.NItems()).To(Equal(3))
				Expect(positiveAM.NItems()).To(Equal(1))
			})
		})
	})

	Describe("Remove facts", func() {
		var (
			testChess []*Chess
			testFacts []Fact
			ams       []*AlphaMem
			conds     = []Guard{
				{
					AliasAttr: "Color",
					Value:     GVString("blue"),
					TestOp:    TestOpEqual,
				},
				{
					AliasAttr: "Color",
					Value:     GVString("red"),
					TestOp:    TestOpEqual,
				},
			}
			tf = TypeInfo{
				T: GValueTypeStruct,
				Fields: map[string]GValueType{
					"Color": GValueTypeString,
				},
			}
		)

		BeforeEach(func() {
			ams = make([]*AlphaMem, 0, len(conds))
			for _, c := range conds {
				am := an.MakeAlphaMem(tf, []Guard{c})
				Expect(am).NotTo(BeNil())
				ams = append(ams, am)
			}

			testChess = getTestFacts()
			testFacts = lo.Map(testChess, func(item *Chess, index int) Fact {
				return Fact{
					ID:    GVIdentity(item.ID),
					Value: NewGVStruct(item),
				}
			})
			lo.ForEach(testChess, func(item *Chess, _ int) {
				an.AddFact(Fact{ID: GVIdentity(item.ID), Value: NewGVStruct(item)})
			})
		})

		AfterEach(func() {
			testChess = testChess[:0]
		})

		It("Remve facts should remove wme in alpha memory too", func() {
			factToRemoved := lo.Filter(testFacts, func(item Fact, index int) bool {
				return testChess[index].Color == "red"
			})
			lo.ForEach(
				factToRemoved,
				func(item Fact, _ int) { an.RemoveFact(item) },
			)

			Expect(ams[0].NItems()).NotTo(BeZero())
			Expect(ams[1].NItems()).To(BeZero())

			newChess := Chess{
				ID:    "B4",
				On:    testChess[2], // b3
				Color: "red",
			}
			an.AddFact(Fact{ID: GVIdentity(newChess.ID), Value: NewGVStruct(newChess)})
			Expect(ams[1].NItems()).To(BeEquivalentTo(1))
		})
	})
})

func firstArg(vals ...any) any {
	return vals[0]
}
