package rete_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/lo"

	. "github.com/ccbhj/grete/rete"
)

func oneInMap[K comparable, V any](m map[K]V) V {
	for _, v := range m {
		return v
	}
	panic("empty map")
}

func getTestWMEs() []*WME {
	var testWMEs = [...]*WME{
		{Name: "B1", Field: "on", Value: TVIdentity("B2")},
		{Name: "B1", Field: "on", Value: TVIdentity("B3")},
		{Name: "B1", Field: "color", Value: TVString("red")},
		{Name: "B2", Field: "on", Value: TVString("table")},
		{Name: "B2", Field: "left-of", Value: TVIdentity("B3")},
		{Name: "B2", Field: "color", Value: TVString("blue")},
		{Name: "B3", Field: "left-of", Value: TVIdentity("B4")},
		{Name: "B3", Field: "on", Value: TVString("table")},
		{Name: "B3", Field: "color", Value: TVString("red")},
	}
	ret := make([]*WME, 0, len(testWMEs))
	for _, w := range testWMEs {
		tmp := w.Clone()
		ret = append(ret, tmp)
	}

	return ret
}

var _ = Describe("AlphaNet", func() {
	var (
		an *AlphaNetwork
	)
	BeforeEach(func() {
		an = NewAlphaNetwork()
		Expect(an).NotTo(BeNil())
	})

	Describe("when constructing AlphaNetwork", func() {
		It("allowed the same condition to be added for more than one time", func() {
			am := an.MakeAlphaMem(Cond{
				Name:   TVIdentity("x"),
				Attr:   "on",
				Value:  TVIdentity("y"),
				TestOp: TestOpEqual,
			}, nil)
			Expect(am).NotTo(BeNil())
			Expect(an.AlphaRoot()).NotTo(BeNil())
		})

		When("testing condition with value of Identity type", func() {
			var child *ConstantTestNode
			var am *AlphaMem
			BeforeEach(func() {
				am = an.MakeAlphaMem(Cond{
					Name:   TVIdentity("x"),
					Attr:   "on",
					Value:  TVIdentity("y"),
					TestOp: TestOpEqual,
				}, nil)
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
					Name:   TVIdentity("x"),
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				}, nil)

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

	Context("AddWME", func() {
		var (
			testWMEs []*WME
			ams      []*AlphaMem
			conds    = []Cond{
				{
					Name:   TVIdentity("x"),
					Attr:   "on",
					Value:  TVIdentity("y"),
					TestOp: TestOpEqual,
				},
				{
					Name:   TVIdentity("x"),
					Attr:   "color",
					Value:  TVString("red"),
					TestOp: TestOpEqual,
				},
			}
		)
		BeforeEach(func() {
			ams = make([]*AlphaMem, 0, len(conds))
			for _, c := range conds {
				am := an.MakeAlphaMem(c, nil)
				Expect(am).NotTo(BeNil())
				ams = append(ams, am)
			}

			testWMEs = getTestWMEs()
			an.AddWME(testWMEs...)
		})

		AfterEach(func() {
			testWMEs = testWMEs[:0]
		})

		It("should matched WMEs with Identity value", func() {
			onCondWMEs := lo.Filter(testWMEs, func(w *WME, idx int) bool {
				return w.Field == "on"
			})
			onCondAM := ams[0]
			Expect(onCondWMEs).To(HaveLen(onCondAM.NItems()))
			onCondAM.ForEachItem(func(w *WME) (stop bool) {
				Expect(onCondWMEs).To(ContainElement(w))
				return false
			})
		})

		It("should matched WMEs with contant value", func() {
			redCondWMEs := lo.Filter(testWMEs, func(w *WME, idx int) bool {
				return w.Field == "color" && w.Value == TVString("red")
			})
			redCondAM := ams[1]
			GinkgoWriter.Printf("redCondWMEs = %s %s\n", redCondWMEs[0], redCondWMEs[1])
			GinkgoWriter.Printf("redCondWMEs = %p %p\n", redCondWMEs[0], redCondWMEs[1])
			Expect(redCondWMEs).To(HaveLen(redCondAM.NItems()))
			redCondAM.ForEachItem(func(w *WME) (stop bool) {
				GinkgoWriter.Printf("redCondAM => %p => %s", w, w)
				Expect(redCondWMEs).To(ContainElement(w))
				return false
			})
		})
	})
})
