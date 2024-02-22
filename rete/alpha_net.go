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
		hash       uint64    // hash of this ConstantTestNode
		value2test TestValue // which field in WME we gonna test
		testFn     TestFunc  // how to test wme's field

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

func (t TestType) GetField(w *WME) TestValue {
	if w == nil {
		panic("wme is nil")
	}
	switch t {
	case TestTypeID:
		return w.Name
	case TestTypeAttr:
		return w.Field
	case TestTypeValue:
		return w.Value
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

func (w WME) Hash() uint64 {
	return uint64(mix32(mix32(w.Name.Hash(), w.Field.Hash()), w.Value.Hash()))
}

func newTopConstantTestNode() *ConstantTestNode {
	return &ConstantTestNode{
		parent: nil,
		testFn: func(TestValue, TestValue) bool { return true }, // always pass the root node test
	}
}

func NewConstantTestNode(parent *ConstantTestNode, hash uint64, val2test TestValue, testFn TestFunc) *ConstantTestNode {
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

func (n *ConstantTestNode) TestType() TestType {
	return TestType((n.hash >> ctnHashTestTypeOff) & 0xF)
}

func (n *ConstantTestNode) Hash() uint32 {
	return uint32(n.hash)
}

func (n *ConstantTestNode) TestValueType() TestValueType {
	return TestValueType((n.hash >> ctnHashTestValueTypeOff) & 0xff)
}

func (n *ConstantTestNode) OutputMem() *AlphaMem {
	return n.outputMem
}

func (n *ConstantTestNode) ForEachChild(fn func(*ConstantTestNode) (stop bool)) {
	for _, child := range n.children {
		if stop := fn(child); stop {
			break
		}
	}
}

func (n *ConstantTestNode) Activate(w *WME) int {
	switch n.TestType() {
	case TestTypeNone:
		break
	case TestTypeID:
		if !n.testFn(w.Name, n.value2test) {
			return 0
		}
	case TestTypeAttr:
		if !n.testFn(TVString(w.Field), n.value2test) {
			return 0
		}
	case TestTypeValue:
		if !n.testFn(w.Value, n.value2test) {
			return 0
		}
	}

	if n.outputMem != nil {
		// this is a leaf node
		n.outputMem.Activate(w)
		return 1
	}

	ret := 0
	for _, child := range n.children {
		ret += child.Activate(w)
	}
	return ret
}

type (
	AlphaNode interface {
		Activate(*WME) int
	}

	AlphaMemSuccesor interface {
		// RightActivate notify there is an new WME added
		// which means there is a new fact comming
		RightActivate(w *WME) int
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
		((c.Value.Type() == TestValueTypeIdentity) || c.TestOp.ToFunc()(c.Value, w.Value))
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

func (m *AlphaMem) Activate(w *WME) int {
	// insert wme at the head of node->items
	if m.items == nil {
		m.items = list.New[*WME]()
	}
	m.nitem++
	m.items.PushFront(w)

	// for tree-based removal
	w.alphaMems = append(w.alphaMems, m)

	ret := 0
	m.ForEachSuccessor(func(node AlphaMemSuccesor) {
		ret += node.RightActivate(w)
	})

	return ret
}

type AlphaNetwork struct {
	root          *ConstantTestNode
	cond2AlphaMem map[uint64]*AlphaMem
}

func NewAlphaNetwork() *AlphaNetwork {
	root := newTopConstantTestNode()
	alphaNet := &AlphaNetwork{
		root:          root,
		cond2AlphaMem: make(map[uint64]*AlphaMem),
	}
	return alphaNet
}

func (n *AlphaNetwork) AlphaRoot() *ConstantTestNode {
	return n.root
}

func (n *AlphaNetwork) AddWME(wmes ...*WME) {
	for _, wme := range wmes {
		n.root.Activate(wme)
	}
}

func (n *AlphaNetwork) makeConstantTestNode(parent *ConstantTestNode, tt TestType, tv TestValue, fn TestFunc) *ConstantTestNode {
	// look for an existing node we can share

	// TODO: validate tt and tv
	h := genConstantTestHash(tt, tv)
	node, in := parent.children[h]
	if in && node != nil {
		return node
	}

	newNode := NewConstantTestNode(parent, h, tv, fn)

	if parent.children == nil {
		parent.children = make(map[uint64]*ConstantTestNode, 2)
	}
	parent.children[h] = newNode
	return newNode
}

func (n *AlphaNetwork) MakeAlphaMem(c Cond, initWMEs map[uint64]*WME) *AlphaMem {
	ch := c.Hash()
	if am, in := n.cond2AlphaMem[ch]; in {
		return am
	}
	currentNode := n.root
	currentNode = n.makeConstantTestNode(currentNode, TestTypeAttr, TVString(c.Attr), TestOpEqual.ToFunc())
	if c.Value.Type() != TestValueTypeIdentity {
		currentNode = n.makeConstantTestNode(currentNode, TestTypeValue, c.Value, c.TestOp.ToFunc())
	}

	if currentNode.outputMem != nil {
		return currentNode.outputMem
	}

	am := NewAlphaMem(currentNode, nil)
	currentNode.outputMem = am
	// initialize am with any current working memory
	for _, w := range initWMEs {
		if w.passAllConstantTest(c) {
			am.Activate(w)
		}
	}
	n.cond2AlphaMem[ch] = am
	return am
}
