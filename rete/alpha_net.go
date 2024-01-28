package rete

import (
	"github.com/zyedidia/generic/list"
)

type (
	WME struct {
		Name  TVIdentity
		Field TVString
		Value TestValue

		tokens    []*Token
		alphaMems []*AlphaMem
		// negativeJoinResults *list.List[*NegativeJoinResult]
	}

	TestType uint8

	// ConstantTestNode perform constant test for each WME,
	// and implements by 'Dataflow Network'(see 2.2.1(page 27) in the paper)
	ConstantTestNode struct {
		hash       uint64              // hash of this ConstantTestNode
		value2test TestValue           // which field in WME we gonna test
		testFn     func(any, any) bool // how to test wme's field

		parent    *ConstantTestNode            // parent, nil for the root node
		outputMem *AlphaMem                    // where to output those wmes that passed the test
		children  map[uint64]*ConstantTestNode // children node
	}
)

const (
	TestTypeNone TestType = iota
	TestTypeID
	TestTypeAttr
	TestTypeValue
)

func (t TestType) GetField(wme *WME) any {
	if wme == nil {
		panic("wme is nil")
	}
	switch t {
	case TestTypeID:
		return wme.Name
	case TestTypeAttr:
		return wme.Field
	case TestTypeValue:
		return wme.Value
	}
	return nil
}

func NewWME(name TVIdentity, field TVString, value TestValue) *WME {
	return &WME{
		Name:  name,
		Field: field,
		Value: value,
	}
}

func (w WME) Clone() *WME {
	return NewWME(w.Name, w.Field, w.Value)
}

func newTopConstantTestNode() *ConstantTestNode {
	return &ConstantTestNode{
		parent: nil,
		testFn: func(any, any) bool { return true }, // always pass the root node test
	}
}

func NewConstantTestNode(parent *ConstantTestNode, hash uint64, val2test TestValue,
	testFn func(any, any) bool) *ConstantTestNode {
	if testFn == nil {
		testFn = TestEqual
	}
	return &ConstantTestNode{
		testFn:     testFn,
		value2test: val2test,
		outputMem:  nil,
		hash:       hash,
		children:   nil,
		parent:     parent,
	}
}

/*
 * Some constants to generate hash for ConstantTestNode
 *
 *                    test_type(4)
 *          reserved      |   test_value_type(8)    test_value_hash(32)
 *             ^          +-+     ^                     ^
 *             |            |     |                     |
 * +---------------------+----+-------+------------------......--------------+
 * 63                    44 40 39   32 31                                    0
 */
const (
	ctnHashTestTypeMask      uint64 = 0x0000_0F00_0000_0000
	ctnHashTestTypeOff       uint64 = 40
	ctnHashTestValueTypeMask uint64 = 0x0000_00FF_0000_0000
	ctnHashTestValueTypeOff  uint64 = 32
	ctnHashValue2TestMask    uint64 = 0xFFFF_FFFF
)

func genConstantTestHash(tt TestType, tv TestValue) uint64 {
	return uint64(tv.Hash())&ctnHashValue2TestMask |
		(uint64(tv.Type()&0xFF) << ctnHashTestValueTypeOff) |
		(uint64(tt&0xF) << ctnHashTestTypeOff)
}

func (n *ConstantTestNode) GetTestType() TestType {
	return TestType((n.hash >> ctnHashTestTypeOff) & 0xF)
}

func (n *ConstantTestNode) GetValueHash() uint32 {
	return uint32(n.hash)
}

func (n *ConstantTestNode) GetTestValueType() TestValueType {
	return TestValueType((n.hash >> ctnHashTestValueTypeOff) & 0xff)
}

func (n *ConstantTestNode) Activate(wme *WME) int {
	switch n.GetTestType() {
	case TestTypeNone:
		break
	case TestTypeID:
		if !n.testFn(wme.Name, n.value2test) {
			return 0
		}
	case TestTypeAttr:
		if !n.testFn(TVString(wme.Field), n.value2test) {
			return 0
		}
	case TestTypeValue:
		if !n.testFn(wme.Value, n.value2test) {
			return 0
		}
	}

	if n.outputMem != nil {
		// this is a leaf node
		n.outputMem.Activate(wme)
		return 0
	}

	ret := 0
	for _, child := range n.children {
		ret += child.Activate(wme)
	}
	return ret
}

type (
	AlphaNode interface {
		Activate(wme *WME) int
	}

	AlphaMemSuccesor interface {
		// RightActivate notify there is an new WME added
		// which means there is a new fact comming
		RightActivate(wme *WME) int
	}

	// AlphaMem store those WMEs that passes constant test
	AlphaMem struct {
		inputConstantNode *ConstantTestNode            // input ConstantTestNode
		items             *list.List[*WME]             // wmes that passed tests of ConstantTestNode
		nitem             int                          // count of items
		successors        *list.List[AlphaMemSuccesor] // ordered
		// reference_count int
	}
)

func (w *WME) passAllConstantTest(c Cond) bool {
	return w.Field == c.Attr &&
		((c.Value.Type() == TestValueTypeIdentity) || c.testFn(c.Value, w.Value))
}

func NewAlphaMem(input *ConstantTestNode, items []*WME) *AlphaMem {
	am := &AlphaMem{
		items:             list.New[*WME](),
		inputConstantNode: input,
	}
	for i := 0; i < len(items); i++ {
		am.items.PushBack(items[i])
	}
	return am
}

func (m *AlphaMem) AddSuccessor(successors ...AlphaMemSuccesor) {
	if m.successors == nil {
		m.successors = list.New[AlphaMemSuccesor]()
	}
	for i := range successors {
		m.successors.PushFront(successors[i])
	}
}

func (m *AlphaMem) RemoveSuccessor(successor AlphaMemSuccesor) {
	removeOneFromListTailWhen(m.successors, func(x AlphaMemSuccesor) bool {
		return x == successor
	})
}

func (m *AlphaMem) ForEachSuccessor(fn func(AlphaMemSuccesor)) {
	if m.successors == nil {
		return
	}
	m.successors.Back.EachReverse(func(n AlphaMemSuccesor) {
		fn(n)
	})
}

func (m *AlphaMem) IsSuccessorsEmpty() bool {
	return isListEmpty(m.successors)
}

func (m *AlphaMem) ForEachItem(fn func(*WME) (stop bool)) {
	listHeadForEach(m.items, fn)
}

func (m *AlphaMem) NItems() int {
	return m.nitem
}

func (m *AlphaMem) Activate(wme *WME) int {
	// insert wme at the head of node->items
	if m.items == nil {
		m.items = list.New[*WME]()
	}
	m.nitem++
	m.items.PushFront(wme)

	// for tree-based removal
	wme.alphaMems = append(wme.alphaMems, m)

	ret := 0
	m.ForEachSuccessor(func(node AlphaMemSuccesor) {
		ret += node.RightActivate(wme)
	})

	return ret
}

type AlphaNetwork struct {
	root          *ConstantTestNode
	cond2AlphaMem map[*Cond]*AlphaMem
}

func NewAlphaNetwork() *AlphaNetwork {
	root := newTopConstantTestNode()
	alphaNet := &AlphaNetwork{
		root: root,
	}
	return alphaNet
}

func (n *AlphaNetwork) AddWME(wmes ...*WME) {
	for _, wme := range wmes {
		n.root.Activate(wme)
	}
}

func (n *AlphaNetwork) makedConstantTestNode(parent *ConstantTestNode, tt TestType, tv TestValue) *ConstantTestNode {
	// look for an existing node we can share

	// TODO: validate tt and tv
	h := genConstantTestHash(tt, tv)
	node, in := parent.children[h]
	if in && node != nil {
		return node
	}

	newNode := NewConstantTestNode(parent, h, tv, func(x, y any) bool {
		return x == y
	})

	if parent.children == nil {
		parent.children = make(map[uint64]*ConstantTestNode, 2)
	}
	parent.children[h] = newNode
	return newNode
}

func (n *AlphaNetwork) MakeAlphaMem(c Cond) *AlphaMem {
	currentNode := n.root
	buildNode := func(field TestType, symbol TestValue) {
		currentNode = n.makedConstantTestNode(currentNode, field, symbol)
	}
	buildNode(TestTypeAttr, TVString(c.Attr))
	if c.Value.Type() != TestValueTypeIdentity {
		buildNode(TestTypeValue, c.Value)
	}

	if currentNode.outputMem != nil {
		return currentNode.outputMem
	}

	am := NewAlphaMem(currentNode, nil)
	currentNode.outputMem = am
	// initialize am with any current working memory
	// for i := 0; i < len(wmes); i++ {
	// 	w := wmes[i]
	// 	if w.passAllConstantTest(c) {
	// 		am.Activate(w)
	// 	}
	// }

	return am
}
