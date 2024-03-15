package rete

import (
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"
)

type Chess struct {
	ID     TVIdentity
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
	}
	b2 = Chess{
		ID:     "B2",
		Color:  "blue",
		LeftOf: &b3,
		On:     &table,
	}
	b3 = Chess{
		ID:    "B3",
		On:    &table,
		Color: "red",
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

	Context("fact hashing", func() {
		It("hash by id and value", func() {
			Expect(Fact{ID: "X", Value: TVString("FOO")}.Hash()).
				Should(BeEquivalentTo(Fact{ID: "X", Value: TVString("FOO")}.Hash()))
			Expect(Fact{ID: "X", Value: TVString("FOO")}.Hash()).
				ShouldNot(BeEquivalentTo(Fact{ID: "Y", Value: TVString("FOO")}.Hash()))
			Expect(Fact{ID: "X", Value: TVString("FOO")}.Hash()).
				ShouldNot(BeEquivalentTo(Fact{ID: "X", Value: TVInt(1)}.Hash()))

			Expect(Fact{ID: "X", Value: NewTVStruct(struct{ Foo string }{"FOO"})}.Hash()).
				Should(BeEquivalentTo(Fact{ID: "X", Value: NewTVStruct(struct{ Foo string }{"FOO"})}.Hash()))
			Expect(Fact{ID: "X", Value: NewTVStruct(struct{ Foo string }{"FOO"})}.Hash()).
				ShouldNot(BeEquivalentTo(Fact{ID: "X", Value: NewTVStruct(struct{ Foo string }{"BAR"})}.Hash()))
			Expect(Fact{ID: "X", Value: NewTVStruct(struct{ Foo string }{"FOO"})}.Hash()).
				ShouldNot(BeEquivalentTo(Fact{ID: "X", Value: NewTVStruct(struct{ Bar string }{"FOO"})}.Hash()))
		})
	})

	Describe("performing type checking", func() {
		It("should check the constant TestValueType", func() {
			Expect(NewTypeTestNode(nil, Cond{
				Alias:     "X",
				AliasAttr: FieldSelf,
				Value:     TVString("foo"),
				AliasType: &TypeInfo{T: TestValueTypeString},
			}).PerformTest(&WME{ID: "X", Value: TVString("bar")})).Should(BeTrue())

			Expect(NewTypeTestNode(nil, Cond{
				Alias:     "X",
				AliasAttr: FieldSelf,
				Value:     TVString("foo"),
				AliasType: &TypeInfo{T: TestValueTypeString},
			}).PerformTest(&WME{ID: "X", Value: TVInt(1)})).Should(BeFalse())

		})

		Context("check the value type of TVStruct", func() {
			It("should check the TestValueType of struct", func() {
				Expect(NewTypeTestNode(nil, Cond{
					Alias:     "X",
					AliasAttr: "On",
					Value:     NewTVStruct(Chess{}),
				}).PerformTest(&WME{ID: "X", Value: TVString("")})).Should(BeFalse())
			})

			It("should check the struct type strictly if reflect.Type is provided ", func() {
				n := NewTypeTestNode(nil, Cond{
					Alias:     "X",
					AliasAttr: "On",
					Value:     NewTVStruct(Chess{}),
					AliasType: &TypeInfo{
						T:  TestValueTypeStruct,
						VT: reflect.TypeOf(Chess{}),
					},
				})
				Expect(n.PerformTest(&WME{ID: "X", Value: TVString("")})).Should(BeFalse())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(Chess{})})).Should(BeTrue())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(&Chess{})})).Should(BeTrue())

				type _Chess struct {
					ID     TVIdentity
					On     TVString
					Color  TVString
					LeftOf TVIdentity
				}
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(_Chess{})})).Should(BeFalse())
			})

			It("could check if value have the required fields", func() {
				n := NewTypeTestNode(nil, Cond{
					Alias:     "X",
					AliasAttr: "On",
					Value:     TVString("Y"),
					AliasType: &TypeInfo{
						T: TestValueTypeStruct,
						Fields: map[string]TestValueType{
							"Color": TestValueTypeString,
						},
					},
				})
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(struct{ Color TVString }{"red"})})).Should(BeTrue())
				// string type also works too
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(struct{ Color string }{"red"})})).Should(BeTrue())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(struct{ On string }{"Y"})})).Should(BeFalse())
			})

			It("could pass the type check for struct having fields required in Cond", func() {
				n := NewTypeTestNode(nil, Cond{
					Alias:     "X",
					AliasAttr: "On",
					Value:     TVString("Y"),
					AliasType: nil,
				})
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(struct{ Color string }{"red"})})).Should(BeFalse())
				Expect(n.PerformTest(&WME{ID: "X", Value: NewTVStruct(struct{ On string }{"Y"})})).Should(BeTrue())
			})
		})

	})

	Describe("constructing AlphaNetwork", func() {
		It("allowed the same condition to be added for more than one time", func() {
			am := an.MakeAlphaMem(Cond{
				Alias:     TVIdentity("x"),
				AliasAttr: "On",
				Value:     TVString("y"),
				TestOp:    TestOpEqual,
			})
			Expect(am).NotTo(BeNil())
			Expect(an.AlphaRoot()).NotTo(BeNil())
		})

		It("allowed the conditions with same Attr but different Values that are an Identity to generate same AlphaMem", func() {
			am0 := an.MakeAlphaMem(Cond{
				Alias:     TVIdentity("x"),
				AliasAttr: "On",
				Value:     TVIdentity("y"),
				TestOp:    TestOpEqual,
			})
			Expect(am0).NotTo(BeNil())
			am1 := an.MakeAlphaMem(Cond{
				Alias:     TVIdentity("x"),
				AliasAttr: "On",
				Value:     TVIdentity("z"),
				TestOp:    TestOpEqual,
			})
			Expect(am1).NotTo(BeNil())
			Expect(fmt.Sprintf("%p", am0)).To(BeEquivalentTo(fmt.Sprintf("%p", am1)))
		})

		When("testing condition with value of Identity type", func() {
			var child AlphaNode
			var am *AlphaMem
			BeforeEach(func() {
				am = an.MakeAlphaMem(Cond{
					Alias:     TVIdentity("x"),
					AliasAttr: "On",
					Value:     TVIdentity("y"),
					TestOp:    TestOpEqual,
				})
				an.AlphaRoot().ForEachChild(func(tn AlphaNode) (stop bool) {
					child = tn
					return true
				})
			})

			It("should only test the Attr of a WME", func() {
				Expect(child.OutputMem()).To(Equal(am))
				Expect(child.PerformTest(NewWME("X", TVString("Y")))).To(BeFalse())
				Expect(child.PerformTest(NewWME("X", NewTVStruct(struct{ Color TVString }{""})))).To(BeFalse())
				Expect(child.PerformTest(NewWME("X", NewTVStruct(struct{ On TVString }{""})))).To(BeTrue())
			})
		})

		Context("testing condition with value of non-Identity type as tested Value", func() {
			var child, grandChild AlphaNode
			var am *AlphaMem
			BeforeEach(func() {
				am = an.MakeAlphaMem(Cond{
					Alias:     TVIdentity("x"),
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
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

			It("should test the Attr or the Type of WME with TypeTestNode  ", func() {
				// Type not matched
				Expect(child.PerformTest(NewWME("X", TVString("!!!")))).To(BeFalse())
				// Attr not found
				Expect(child.PerformTest(NewWME("X", NewTVStruct(struct{ Background TVString }{"red"})))).To(BeFalse())
			})

			It("should test the Value of an attr of WME with ConstantTestNode", func() {
				// Value not matched
				Expect(grandChild.PerformTest(NewWME("X", NewTVStruct(struct{ Color TVString }{"blue"})))).To(BeFalse())
				Expect(grandChild.PerformTest(NewWME("X", NewTVStruct(struct{ Color TVString }{"red"})))).To(BeTrue())
			})

		})
	})

	Describe("Adding fact", func() {
		var (
			testFacts []*Chess
			ams       []*AlphaMem
			conds     = []Cond{
				{
					Alias:     TVIdentity("x"),
					AliasAttr: "On",
					Value:     TVIdentity("y"),
					TestOp:    TestOpEqual,
				},
				{
					Alias:     TVIdentity("x"),
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
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
			lo.ForEach(testFacts, func(item *Chess, _ int) {
				an.AddFact(Fact{ID: TVIdentity(item.ID), Value: NewTVStruct(item)})
			})
		})

		AfterEach(func() {
			testFacts = testFacts[:0]
		})

		When("performing type checking", func() {

			It("any item in testFacts has the matched type", func() {
				typeAM := ams[0]
				Expect(typeAM.NItems()).To(BeEquivalentTo(len(testFacts)))
				typeAM.ForEachItem(func(w *WME) (stop bool) {
					Expect(testFacts).To(ContainElement(w.Value.(*TVStruct).Value()))
					return false
				})
			})

			It("should not match any fact with different TestValueType", func() {
				typeAM := ams[0]
				an.AddFact(Fact{
					ID:    "B4",
					Value: TVString("string"),
				})
				Expect(typeAM.NItems()).To(BeEquivalentTo(len(testFacts)))
			})

			It("should not match any fact with type missing field 'On' or 'Color' ", func() {
				typeAM := ams[0]
				an.AddFact(Fact{
					ID: "B5",
					Value: NewTVStruct(struct {
						LeftOf TVIdentity
					}{"B3"}),
				})
				Expect(typeAM.NItems()).To(BeEquivalentTo(len(testFacts)))
			})

			It("should match any fact with differen struct type but has field 'On' or 'Color' ", func() {
				typeAM := ams[0]
				type Foo struct {
					On    TVString
					Color TVString
				}
				an.AddFact(Fact{
					ID:    "B6",
					Value: NewTVStruct(Foo{"B4", "black"}),
				})
				Expect(typeAM.NItems()).To(BeEquivalentTo(1 + len(testFacts)))
			})
		})

		It("should matched Facts with contant value", func() {
			redFacts := lo.Filter(testFacts, func(f *Chess, idx int) bool {
				return f.Color == "red"
			})
			am := ams[1]
			Expect(redFacts).To(HaveLen(am.NItems()))
			am.ForEachItem(func(w *WME) (stop bool) {
				Expect(redFacts).To(ContainElement(w.Value.(*TVStruct).Value()))
				return false
			})
		})
	})

	Describe("Remove facts", func() {
		var (
			testChess []*Chess
			testFacts []Fact
			ams       []*AlphaMem
			conds     = []Cond{
				{
					Alias:     TVIdentity("x"),
					AliasAttr: "On",
					Value:     TVIdentity("y"),
					TestOp:    TestOpEqual,
				},
				{
					Alias:     TVIdentity("x"),
					AliasAttr: "Color",
					Value:     TVString("red"),
					TestOp:    TestOpEqual,
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

			testChess = getTestFacts()
			testFacts = lo.Map(testChess, func(item *Chess, index int) Fact {
				return Fact{
					ID:    TVIdentity(item.ID),
					Value: NewTVStruct(item),
				}
			})
			lo.ForEach(testChess, func(item *Chess, _ int) {
				an.AddFact(Fact{ID: TVIdentity(item.ID), Value: NewTVStruct(item)})
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
			an.AddFact(Fact{ID: TVIdentity(newChess.ID), Value: NewTVStruct(newChess)})
			Expect(ams[1].NItems()).To(BeEquivalentTo(1))
		})
	})
})
