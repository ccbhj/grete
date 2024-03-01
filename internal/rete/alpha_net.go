package rete

import (
	"github.com/zyedidia/generic/list"
)

type (
	// WME is the working memory element, use to store the fact when matching
	WME struct {
		ID    TVIdentity
		Field TVString
		Value TestValue

		tokens    set[*Token]
		alphaMems set[*AlphaMem]
		// negativeJoinResults *list.List[*NegativeJoinResult]
	}

	// TestType specify which field of a WME we are going to test
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
		return w.ID
	case TestTypeAttr:
		return w.Field
	case TestTypeValue:
		return w.Value
	}
	return nil
}

func NewWME(name TVIdentity, field TVString, value TestValue) *WME {
	return &WME{
		ID:    name,
		Field: field,
		Value: value,

		tokens:    newSet[*Token](),
		alphaMems: newSet[*AlphaMem](),
	}
}

func (w WME) Clone() *WME {
	return NewWME(w.ID, w.Field, w.Value)
}

func (w WME) FactOfWME() Fact {
	return Fact{
		ID:    w.ID,
		Field: w.Field,
		Value: w.Value,
	}
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
		if !n.testFn(w.ID, n.value2test) {
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
		cond              Cond
		inputConstantNode *ConstantTestNode            // input ConstantTestNode
		items             set[*WME]                    // wmes that passed tests of ConstantTestNode
		successors        *list.List[AlphaMemSuccesor] // ordered
		an                *AlphaNetwork                // which AlphaNetwork this mem belong to
	}
)

func (w *WME) passAllConstantTest(c Cond) bool {
	return w.Field == c.Attr &&
		((c.Value.Type() == TestValueTypeIdentity) || c.TestOp.ToFunc()(c.Value, w.Value))
}

func newAlphaMem(cond Cond, input *ConstantTestNode, an *AlphaNetwork) *AlphaMem {
	am := &AlphaMem{
		cond:              cond,
		items:             newSet[*WME](),
		inputConstantNode: input,
		an:                an,
	}
	return am
}

func (m *AlphaMem) hasWME(w *WME) bool {
	return m.items.Contains(w)
}

func (m *AlphaMem) addWME(w *WME) {
	m.items.Add(w)
	w.alphaMems.Add(m)
}

func (m *AlphaMem) removeWME(w *WME) {
	m.items.Add(w)
	for am := range w.alphaMems {
		am.items.Del(w)
		// TODO: try to left unlink the am from join node
	}
}

func (m *AlphaMem) NItems() int {
	return m.items.Len()
}

func (m *AlphaMem) ForEachItem(fn func(*WME) (stop bool)) {
	for item := range m.items {
		if fn(item) {
			return
		}
	}
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

func (m *AlphaMem) forEachSuccessor(fn func(AlphaMemSuccesor)) {
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

func (m *AlphaMem) Activate(w *WME) int {
	if m.hasWME(w) {
		return 1
	}
	m.addWME(w)

	ret := 0
	m.forEachSuccessor(func(node AlphaMemSuccesor) {
		ret += node.RightActivate(w)
	})
	return ret
}

// _destory clean remove all the WME from alpha mem along with all the ConstantTestNode that is no long in use
func (m *AlphaMem) _destory() {
	m.items.ForEach(func(item *WME) {
		h := item.FactOfWME().Hash()
		m.an.removeWME(h, item)
	})

	// clean up ConstantTestNode
	var (
		parent  *ConstantTestNode
		current = m.inputConstantNode
	)
	current.outputMem = nil
	// remove current node from its parent when current node has no child (leaf node)
	for current != nil && current.hash != 0 && len(current.children) == 0 {
		parent = current.parent
		if parent != nil {
			delete(parent.children, current.hash)
		}
		current.parent = nil
		current = parent
	}
}

type AlphaNetworkOption struct {
	LazyDestory bool // whether to reserve alpha mem that is dangling when destorying
}

type AlphaNetwork struct {
	root          *ConstantTestNode
	cond2AlphaMem map[uint64]*AlphaMem
	workingMems   map[uint64]*WME
}

func NewAlphaNetwork() *AlphaNetwork {
	root := newTopConstantTestNode()
	alphaNet := &AlphaNetwork{
		root:          root,
		cond2AlphaMem: make(map[uint64]*AlphaMem),
		workingMems:   make(map[uint64]*WME),
	}
	return alphaNet
}

func (n *AlphaNetwork) AlphaRoot() *ConstantTestNode {
	return n.root
}

func (n *AlphaNetwork) addWME(sum uint64, w *WME) int {
	n.workingMems[sum] = w
	return n.root.Activate(w)
}

func (n *AlphaNetwork) AddFact(f Fact) int {
	h := f.Hash()
	if _, in := n.workingMems[h]; in {
		return 0
	}
	w := f.WMEFromFact()
	return n.addWME(h, w)
}

func (n *AlphaNetwork) RemoveFact(f Fact) {
	h := f.Hash()
	w, in := n.workingMems[h]
	if !in {
		return
	}
	n.removeWME(h, w)
}

func (n *AlphaNetwork) removeWME(sum uint64, w *WME) {
	for am := range w.alphaMems {
		am.removeWME(w)
	}
	w.alphaMems.Clear()
	for t := range w.tokens {
		t.destory()
	}
	w.tokens.Clear()
	delete(n.workingMems, sum)
	// TODO: support negative node(clear negativeJoinResults)
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

func (n *AlphaNetwork) MakeAlphaMem(c Cond) *AlphaMem {
	h := n.hashCond(c)
	if am, in := n.cond2AlphaMem[h]; in {
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

	am := newAlphaMem(c, currentNode, n)
	currentNode.outputMem = am
	// initialize am with any current working memory
	for _, w := range n.workingMems {
		if w.passAllConstantTest(c) {
			am.Activate(w)
		}
	}
	n.cond2AlphaMem[h] = am
	return am
}

func (n *AlphaNetwork) DestoryAlphaMem(alphaMem *AlphaMem) {
	if alphaMem == nil {
		return
	}
	h := n.hashCond(alphaMem.cond)
	if _, in := n.cond2AlphaMem[h]; !in {
		return
	}
	alphaMem._destory()
	delete(n.cond2AlphaMem, h)
}

func (n *AlphaNetwork) hashCond(c Cond) uint64 {
	// In the paper, the authors propose to use exhaustive-table-lookup to figure out which alpha memory should be used.
	// The exhaustive-table-lookup combine the id, attribute and value in an condition to lookup in an cache table but all the
	// value that is not a "constant" value will be treated as an wildcard and got ignored. The author assumed that all the
	// value in a condition can be a "constant" but I doubt that supporting variables to be attributes or "constant" to be
	// attribute are necessary. As a result, here I assume that:
	//   - cond.ID must be TestValueTypeIdentity and we will NOT build ConstantTestNode for it;
	//   - cond.Attr must be constant(TVString) and we will always build ConstantTestNode for it;
	//   - cond.Value might be TestValueTypeIdentity and we will sometimes build ConstantTestNode for it;
	// see (*AlphaNetwork).MakeAlphaMem for the implementation.

	// constant test will never perform constant test on values whose type is TestValueTypeIdentity
	// so we don't need to take c.Name into consideration when hashing it.
	opt := CondHashOptMaskID
	if c.Value.Type() == TestValueTypeIdentity {
		opt |= CondHashOptMaskValue
	}

	return c.Hash(opt)
}

func (f Fact) WMEFromFact() *WME {
	return NewWME(f.ID, f.Field, f.Value)
}

func (f Fact) Hash() uint64 {
	return uint64(mix32(mix32(f.ID.Hash(), f.Field.Hash()), f.Value.Hash()))
}
