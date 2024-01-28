package rete

import "testing"
import "github.com/stretchr/testify/assert"

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

func TestAlphaNetwork(t *testing.T) {
	{
		t.Log("[TEST] Identity type value should pass directly")
		an := NewAlphaNetwork()
		am := an.MakeAlphaMem(Cond{
			Name:   TVIdentity("x"),
			Attr:   "on",
			Value:  TVIdentity("y"),
			testFn: TestEqual,
		})
		assert.NotNil(t, am)
		assert.NotEmpty(t, an.root.children)

		testWMEs := getTestWMEs()
		an.AddWME(testWMEs...)
		for _, child := range an.root.children {
			assert.EqualValues(t, am, child.outputMem)
			if assert.NotNil(t, am.items.Front) {
				wmes := []*WME{testWMEs[7], testWMEs[3], testWMEs[1], testWMEs[0]}
				assert.EqualValues(t, wmes, listToSlice(am.items))
				for _, wme := range wmes {
					assert.EqualValues(t, []*AlphaMem{am}, wme.alphaMems)
				}
			}
		}

		t.Log("[TEST] add it again, same alpha should be thrown")
		newAM := an.MakeAlphaMem(
			Cond{
				Name:   TVIdentity("x"),
				Attr:   "on",
				Value:  TVIdentity("y"),
				testFn: TestEqual,
			})
		assert.Equal(t, am, newAM)
		assert.Len(t, an.root.children, 1)
	}

	{
		t.Log("[TEST] test attr value condition")
		an := NewAlphaNetwork()
		testWMEs := getTestWMEs()
		am := an.MakeAlphaMem(Cond{
			Name:   TVIdentity("z"),
			Attr:   "on",
			Value:  TVString("table"),
			testFn: TestEqual,
		})
		assert.NotNil(t, am)
		assert.Len(t, an.root.children, 1)

		an.AddWME(testWMEs...)
		child := oneInMap(an.root.children) // node testing Attr == on

		assert.Len(t, child.children, 1)
		grandChild := oneInMap(child.children) // node testing Value == "table"
		assert.EqualValues(t, TestTypeValue, grandChild.GetTestType())
		assert.EqualValues(t, am, grandChild.outputMem)
		if assert.NotNil(t, am.items.Front) {
			wmes := []*WME{testWMEs[7], testWMEs[3]}
			assert.EqualValues(t, wmes, listToSlice(am.items))
			for _, wme := range wmes {
				assert.EqualValues(t, []*AlphaMem{am}, wme.alphaMems)
			}
		}
	}

	{
		t.Log("[TEST] test identity value condition")
		an := NewAlphaNetwork()
		am := an.MakeAlphaMem(Cond{
			Name:   TVIdentity("x"),
			Attr:   "left-of",
			Value:  TVIdentity("y"),
			testFn: TestEqual,
		})
		assert.NotNil(t, am)
		assert.Len(t, an.root.children, 1)

		testWMEs := getTestWMEs()
		an.AddWME(testWMEs...)

		child := oneInMap(an.root.children) // node testing Attr == left-of
		assert.Len(t, child.children, 0)
		assert.EqualValues(t, TestTypeAttr, child.GetTestType())
		assert.EqualValues(t, am, child.outputMem)

		if assert.NotNil(t, am.items.Front) {
			wmes := []*WME{testWMEs[6], testWMEs[4]}
			assert.EqualValues(t, wmes, listToSlice(am.items))
			for _, wme := range wmes {
				assert.EqualValues(t, []*AlphaMem{am}, wme.alphaMems)
			}
		}
	}

}

func TestAddWME(t *testing.T) {
	testWMEs := getTestWMEs()
	an := NewAlphaNetwork()
	am := an.MakeAlphaMem(Cond{
		Name:   TVIdentity("z"),
		Attr:   "color",
		Value:  TVString("red"),
		testFn: TestEqual,
	})
	an.AddWME(testWMEs...)
	child := oneInMap(an.root.children)
	grandchild := oneInMap(child.children)
	assert.EqualValues(t, grandchild.outputMem, am)

	wmes := []*WME{testWMEs[8], testWMEs[2]}
	if assert.NotNil(t, am.items.Front) {
		assert.EqualValues(t, wmes, listToSlice(am.items))
	}
	for _, wme := range wmes {
		assert.EqualValues(t, []*AlphaMem{am}, wme.alphaMems)
	}

	{
		matchWME := NewWME("u", "color", TVString("red"))
		wmes = append(wmes, matchWME)
		an.AddWME(matchWME)

		for _, wme := range wmes {
			assert.EqualValues(t, []*AlphaMem{am}, wme.alphaMems)
		}
		assert.EqualValues(t, am, matchWME.alphaMems[0])
	}

	{
		mismatchWME := NewWME("u", "color", TVString("white"))
		an.AddWME(mismatchWME)

		for _, wme := range wmes {
			assert.EqualValues(t, []*AlphaMem{am}, wme.alphaMems)
		}
		assert.Empty(t, mismatchWME.alphaMems)
	}
}
