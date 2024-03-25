package rete

import (
	"reflect"

	"github.com/pkg/errors"

	"github.com/zyedidia/generic/list"

	"github.com/ccbhj/grete/internal/log"
)

type (
	// WME is the working memory element, use to store the fact when matching
	WME struct {
		ID    TVIdentity
		Value TestValue

		tokens       set[*Token]
		alphaMems    set[*AlphaMem]
		nJoinResults set[*negativeJoinResult]
	}
)

func NewWME(name TVIdentity, value TestValue) *WME {
	return &WME{
		ID:    name,
		Value: value,

		tokens:       newSet[*Token](),
		alphaMems:    newSet[*AlphaMem](),
		nJoinResults: newSet[*negativeJoinResult](),
	}
}

func (w *WME) Hash() uint64 {
	if w == nil {
		return 0
	}
	return w.FactOfWME().Hash()
}

func (w *WME) FactOfWME() Fact {
	return Fact{
		ID:    w.ID,
		Value: w.Value,
	}
}

func (w *WME) HasAttr(attr string) bool {
	switch attr {
	case FieldSelf, FieldID:
		return true
	}
	val := w.Value
	// ony TVStruct has fields other than FieldSelf
	if val.Type() != TestValueTypeStruct {
		return false
	}
	return val.(*TVStruct).HasField(attr)
}

func (w *WME) GetAttrValue(attr string) (TestValue, error) {
	v, err := w.getAttrValue(attr, false)
	if err != nil {
		return nil, err
	}

	return v.(TestValue), nil
}

func (w *WME) GetAttrValueRaw(attr string) (any, error) {
	return w.getAttrValue(attr, true)
}

func (w *WME) getAttrValue(attr string, raw bool) (any, error) {
	switch attr {
	case FieldSelf:
		return w.Value, nil
	case FieldID:
		return w.ID, nil
	}
	val := w.Value
	// ony TVStruct has fields other than FieldSelf
	if val.Type() != TestValueTypeStruct {
		return nil, errors.WithMessagef(ErrFieldNotFound, "for type is %s", val.Type().String())
	}

	var ret any
	v, rv, err := val.(*TVStruct).GetField(attr)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if raw {
		ret = rv
	} else {
		ret = v
	}
	return ret, nil
}

// _destory should only be called by an AlphaNetwork, since it manages all the WMEs
func (w *WME) _destory() {
	w.clearAlphaMems()
	w.clearTokens()
	w.clearNegativeJoinResult()
}

func (w *WME) clearAlphaMems() {
	w.alphaMems.Clear()
}

func (w *WME) clearTokens() {
	for t := range w.tokens {
		t.destory()
	}
	w.tokens.Clear()
}

func (w *WME) clearNegativeJoinResult() {
	if len(w.nJoinResults) == 0 {
		return
	}
	w.nJoinResults.ForEach(func(njr *negativeJoinResult) {
		owner := njr.owner
		owner.nJoinResults.Del(njr)
		if len(owner.nJoinResults) == 0 {
			// negative cond was previously false but is now true
			owner.node.ForEachChildNonStop(func(child ReteNode) {
				if bn, ok := child.(BetaNode); ok {
					bn.leftActivate(owner, nil)
				}
			})
		}
	})
	w.nJoinResults.Clear()
}

type (
	alphaMemSuccesor interface {
		// RightActivate notify there is an new WME added
		// which means there is a new fact comming
		rightActivate(w *WME) int
	}

	// AlphaMem store those WMEs that passes constant test
	AlphaMem struct {
		cond           Cond
		inputAlphaNode AlphaNode
		items          set[*WME]                    // wmes that passed tests of ConstantTestNode
		successors     *list.List[alphaMemSuccesor] // must be ordered, see Figure 2.5 in paper 2.4
		an             *AlphaNetwork                // which AlphaNetwork this mem belong to
	}
)

func (w *WME) passAllConstantTest(c Cond) bool {
	return w.HasAttr(string(c.AliasAttr)) &&
		((c.Value.Type() == TestValueTypeIdentity) || c.TestOp.ToFunc()(c.Value, w.Value))
}

func newAlphaMem(cond Cond, input AlphaNode, an *AlphaNetwork) *AlphaMem {
	am := &AlphaMem{
		cond:           cond,
		items:          newSet[*WME](),
		inputAlphaNode: input,
		an:             an,
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

func (m *AlphaMem) AddSuccessor(successors ...alphaMemSuccesor) {
	if m.successors == nil {
		m.successors = list.New[alphaMemSuccesor]()
	}
	for i := range successors {
		m.successors.PushFront(successors[i])
	}
}

func (m *AlphaMem) RemoveSuccessor(successor alphaMemSuccesor) {
	removeOneFromListTailWhen(m.successors, func(x alphaMemSuccesor) bool {
		return x == successor
	})
}

func (m *AlphaMem) forEachSuccessorNonStop(fn func(alphaMemSuccesor)) {
	if m.successors == nil {
		return
	}
	m.successors.Back.EachReverse(func(n alphaMemSuccesor) {
		fn(n)
	})
}

func (m *AlphaMem) forEachSuccessor(fn func(alphaMemSuccesor) (stop bool)) {
	if m.successors == nil {
		return
	}
	node := m.successors.Back
	for node != nil {
		if fn(node.Value) {
			return
		}
		node = node.Prev
	}
}

func (m *AlphaMem) IsSuccessorsEmpty() bool {
	return isListEmpty(m.successors)
}

func (m *AlphaMem) Activate(w *WME) int {
	if !m.hasWME(w) {
		m.addWME(w)
	}

	ret := 0
	m.forEachSuccessorNonStop(func(node alphaMemSuccesor) {
		ret += node.rightActivate(w)
	})
	return ret
}

// _destory clean remove all the WME from alpha mem along with all the ConstantTestNode that is no long in use
func (m *AlphaMem) _destory() {
	m.items.ForEach(func(item *WME) {
		h := item.FactOfWME().Hash()
		m.an.removeWME(h, item)
	})
	m.items.Clear()

	// clean up AlphaNode
	var (
		parent  AlphaNode
		current = m.inputAlphaNode
	)
	current.SetOutputMem(nil)
	// remove current node from its parent when current node has no child (leaf node)
	for current != nil && current.Hash() != 0 && current.NChildren() == 0 {
		parent = current.Parent()
		if parent != nil {
			parent.RemoveChild(current)
		}
		current.SetParent(nil)
		current = parent
	}

	m.inputAlphaNode = nil
	m.an = nil
}

type AlphaNetwork struct {
	root          AlphaNode
	cond2AlphaMem map[uint64]*AlphaMem
	workingMems   map[uint64]*WME
}

func NewAlphaNetwork() *AlphaNetwork {
	root := newAlphaNode(nil)
	alphaNet := &AlphaNetwork{
		root:          root,
		cond2AlphaMem: make(map[uint64]*AlphaMem),
		workingMems:   make(map[uint64]*WME),
	}
	return alphaNet
}

func (n *AlphaNetwork) AlphaRoot() AlphaNode {
	return n.root
}

func (n *AlphaNetwork) activateAlphaNode(node AlphaNode, w *WME) int {
	ret := 0
	testOk, err := node.PerformTest(w)
	if err != nil {
		log.L("fail to perform test on wme(%+v): %s", w, err)
		return 0
	}

	// check if node support negative activation
	var negativeChild *NegativeTestNode
	nnode, ok := node.(negatableAlphaNode)
	if ok {
		negativeChild = nnode.GetNegativeNode()
	}

	if !testOk {
		if negativeChild != nil {
			ret += n.activateAlphaNode(negativeChild, w)
		}
		return ret
	}

	if mem := node.OutputMem(); mem != nil {
		mem.Activate(w)
		ret++
	}

	node.ForEachChild(func(child AlphaNode) (stop bool) {
		// avoid negative activation
		if child == negativeChild {
			return false
		}
		log.BugOn(n.root.OutputMem() == nil || child.OutputMem() != n.root.OutputMem(),
			"dummy output mem is found!!!")
		ret += n.activateAlphaNode(child, w)
		return false
	})
	return ret
}

func (n *AlphaNetwork) AddFact(f Fact) int {
	h := f.Hash()
	w, in := n.workingMems[h]
	if in {
		return n.activateAlphaNode(n.root, w)
	}
	w = f.WMEFromFact()
	return n.addWME(h, w)
}

func (n *AlphaNetwork) addWME(sum uint64, w *WME) int {
	n.workingMems[sum] = w
	return n.activateAlphaNode(n.root, w)
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
	delete(n.workingMems, sum)
	w.alphaMems.ForEach(func(am *AlphaMem) {
		am.removeWME(w)
	})
	w._destory()
}

func (n *AlphaNetwork) buildOrShareTestNode(parent AlphaNode, c Cond, makeFn func(*alphaNode, Cond) AlphaNode) AlphaNode {
	var (
		child AlphaNode
		base  = newAlphaNode(parent)
		an    = makeFn(base, c)
		h     uint64
	)
	if _, ok := an.(sharableAlphaNode); !ok {
		goto build
	}
	h = an.Hash()
	child = parent.GetChild(h)
	if child != nil {
		if shareNode, ok := child.(sharableAlphaNode); ok {
			shareNode.Adjust(c)
			return child
		}
	}

build:
	parent.AddChild(an)
	return an
}

func (n *AlphaNetwork) buildOrShareNegativeTestNode(c Cond, nn negatableAlphaNode) AlphaNode {
	negativeNode := nn.GetNegativeNode()
	if negativeNode != nil {
		return negativeNode
	}

	base := newAlphaNode(nn.(AlphaNode))
	newNode := newNegativeTestNode(base, c)
	nn.SetNegativeNode(newNode)
	return newNode
}

func (n *AlphaNetwork) MakeAlphaMem(c Cond, negative bool) *AlphaMem {
	h := n.hashCond(c)
	if am, in := n.cond2AlphaMem[h]; in {
		return am
	}

	var (
		currentNode = n.root
	)

	// test type
	currentNode = n.buildOrShareTestNode(currentNode, c, NewTypeTestNode)
	if c.Value.Type() != TestValueTypeIdentity {
		currentNode = n.buildOrShareTestNode(currentNode, c, NewConstantTestNode)
		if negative {
			nnode, ok := currentNode.(negatableAlphaNode)
			if !ok {
				// TODO: handle error
				panic("node not supported negation")
			}
			currentNode = n.buildOrShareNegativeTestNode(c, nnode)
		}
	}

	if am := currentNode.OutputMem(); am != nil {
		return am
	}

	am := newAlphaMem(c, currentNode, n)
	currentNode.SetOutputMem(am)
	n.cond2AlphaMem[h] = am
	return am
}

// initialize am with any current working memory
// TODO: deprecate this function
func (n *AlphaNetwork) InitAlphaMem(am *AlphaMem, c Cond) {
	for _, w := range n.workingMems {
		n.addWME(w.Hash(), w)
	}
}

func (n *AlphaNetwork) InitDummyAlphaMem(am *AlphaMem, c Cond) {
	for _, w := range n.workingMems {
		am.Activate(w)
	}
}

func (n *AlphaNetwork) dummyAlphaNode() *AlphaMem {
	if am := n.root.OutputMem(); am != nil {
		return am
	}
	return nil
}

func (n *AlphaNetwork) makeDummyAlphaMem() *AlphaMem {
	// finish building test node
	if am := n.root.OutputMem(); am != nil {
		return am
	}

	am := newAlphaMem(Cond{}, n.root, n)
	n.root.SetOutputMem(am)
	// initialize am with any current working memory
	for _, w := range n.workingMems {
		am.Activate(w)
	}
	return am
}

func (n *AlphaNetwork) DestoryAlphaMem(alphaMem *AlphaMem) {
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
	//   - cond.ID must be TestValueTypeIdentity and we will build TypeTestNode for each alias;
	//   - cond.Attr must be constant(TVString) and we will test it in TypeTestNode to see if a wme has such an attr.
	//   - cond.Value might be TestValueTypeIdentity or constant and we will sometimes build ConstantTestNode for it;
	// see (*AlphaNetwork).MakeAlphaMem for the implementation.
	// since we will not test Value whose type is TestValueTypeIdentity, we can mask it when hasing conds so that Cond($x, ^On, $y) and Cond($x, ^On, ^z) will be treated as they are the same.

	var opt uint64
	if c.Value.Type() == TestValueTypeIdentity {
		opt |= CondHashOptMaskValue
	}
	h := c.Hash(opt)
	return h
}

// Alpha test nodes
type (
	alphaNode struct {
		parent    AlphaNode            // parent, nil for the root node
		children  map[uint64]AlphaNode // children node to be activated when PerformTest() returns true
		outputMem *AlphaMem
	}

	AlphaNode interface {
		Hash() uint64
		PerformTest(*WME) (bool, error)
		Parent() AlphaNode
		SetParent(AlphaNode)
		IsParentOf(child AlphaNode) bool
		AddChild(child AlphaNode)
		GetChild(hash uint64) AlphaNode
		RemoveChild(child AlphaNode)
		NChildren() int
		SetOutputMem(mem *AlphaMem)
		OutputMem() *AlphaMem
		ForEachChild(fn func(AlphaNode) (stop bool))
	}

	sharableAlphaNode interface {
		AlphaNode
		Adjust(c Cond)
	}

	negatableAlphaNode interface {
		AlphaNode
		GetNegativeNode() *NegativeTestNode
		SetNegativeNode(n *NegativeTestNode)
	}
)

func newAlphaNode(parent AlphaNode) *alphaNode {
	return &alphaNode{
		parent:   parent,
		children: make(map[uint64]AlphaNode),
	}
}

func (n *alphaNode) Parent() AlphaNode {
	return n.parent
}

func (n *alphaNode) SetParent(p AlphaNode) {
	n.parent = p
}

func (n *alphaNode) IsParentOf(child AlphaNode) bool {
	h := child.Hash()
	_, in := n.children[h]
	return in
}

func (n *alphaNode) GetChild(h uint64) AlphaNode {
	return n.children[h]
}

func (n *alphaNode) AddChild(child AlphaNode) {
	h := child.Hash()
	_, in := n.children[h]
	if in {
		return
	}
	n.children[h] = child
}

func (n *alphaNode) ForEachChild(fn func(AlphaNode) (stop bool)) {
	for _, child := range n.children {
		if fn(child) {
			break
		}
	}
}

func (n *alphaNode) RemoveChild(child AlphaNode) {
	h := child.Hash()
	delete(n.children, h)
}

func (n *alphaNode) NChildren() int {
	return len(n.children)
}

func (n *alphaNode) SetOutputMem(mem *AlphaMem) {
	n.outputMem = mem
}

func (n *alphaNode) OutputMem() *AlphaMem {
	return n.outputMem
}

// identify root nodes by Hash() == 0
func (n *alphaNode) Hash() uint64                   { return 0 }
func (n *alphaNode) PerformTest(*WME) (bool, error) { return true, nil }

type TypeTestNode struct {
	*alphaNode       `hash:"ignore"`
	TypeInfo         *TypeInfo      `hash:"ignore"` // TypeInfo specified in a Cond
	FieldConstraints map[string]int `hash:"ignore"` // field names we learned from conditions, and its reference count(count of cond)
	Alias            TVIdentity
}

var _ AlphaNode = (*TypeTestNode)(nil)

func NewTypeTestNode(alphaNode *alphaNode, c Cond) AlphaNode {
	var (
		tf               *TypeInfo
		fieldConstraints map[string]int
	)
	if c.AliasType == nil {
		fieldConstraints = make(map[string]int)
		if c.AliasAttr != FieldSelf {
			// must has field c.Attr, but we don't know what type it is
			// leave it until we perform the test op
			fieldConstraints = map[string]int{
				string(c.AliasAttr): 1,
			}
		}
	} else {
		tf = c.AliasType
	}

	return &TypeTestNode{
		alphaNode:        alphaNode,
		TypeInfo:         tf,
		Alias:            c.Alias,
		FieldConstraints: fieldConstraints,
	}
}

func (t *TypeTestNode) Adjust(c Cond) {
	if c.AliasAttr == FieldSelf || c.AliasType != nil {
		return
	}
	// t.RequiredFields is generated by us, we can add more field constrait here by new Cond
	t.FieldConstraints[string(c.AliasAttr)] = t.FieldConstraints[string(c.AliasAttr)] + 1
}

func (t *TypeTestNode) PerformTest(w *WME) (bool, error) {
	if t.TypeInfo != nil {
		switch t.TypeInfo.T {
		case TestValueTypeInt, TestValueTypeUint,
			TestValueTypeFloat, TestValueTypeString:
			return t.TypeInfo.T == w.Value.Type(), nil
		case TestValueTypeStruct:
			return w.Value.Type() == TestValueTypeStruct && t.checkStructType(w), nil
		case TestValueTypeUnknown:
			// unknow type ? leave it until we perform the test op
			return true, nil
		}
	} else if len(t.FieldConstraints) > 0 {
		return w.Value.Type() == TestValueTypeStruct && t.checkFieldConstraints(w), nil
	}
	return true, nil
}

func (t *TypeTestNode) checkFieldConstraints(w *WME) bool {
	v := w.Value.(*TVStruct).v
	vt := reflect.TypeOf(v)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}
	// check whether the struct contains fields in tf
	for f := range t.FieldConstraints {
		_, in := vt.FieldByName(f)
		if !in {
			return false
		}
	}

	return true
}

func (t *TypeTestNode) checkStructType(w *WME) bool {
	tf := t.TypeInfo
	v := w.Value.(*TVStruct).v
	vt := reflect.TypeOf(v)
	if vt.Kind() == reflect.Ptr {
		vt = vt.Elem()
	}
	// strict type checking if the reflect.Type is provided
	if tf.VT != nil {
		return tf.VT == vt
	}

	// check whether the struct contains fields in tf
	for f, t := range tf.Fields {
		sf, in := vt.FieldByName(f)
		if !in {
			return false
		}
		if t == TestValueTypeUnknown || t == TestValueTypeStruct {
			// skip field type checking
			continue
		}
		sft := sf.Type
		if sft.Kind() == reflect.Ptr {
			sft = sft.Elem()
		}
		if rt := t.RType(); rt != nil && !sft.ConvertibleTo(rt) {
			return false
		}
	}

	return true
}

func (t *TypeTestNode) Hash() uint64 {
	return hashAny(t)
}

// ConstantTestNode perform constant test for each WME,
// and implements by 'Dataflow Network'(see 2.2.1(page 27) in the paper)
type ConstantTestNode struct {
	*alphaNode   `hash:"ignore"`
	negativeNode *NegativeTestNode `hash:"ignore"`
	Field        string            // which field in WME we gonna test
	V            TestValue         // the value to be compared
	TestOp       TestOp            // test operation
}

var _ AlphaNode = (*ConstantTestNode)(nil)
var _ negatableAlphaNode = (*ConstantTestNode)(nil)

func NewConstantTestNode(alphaNode *alphaNode, c Cond) AlphaNode {
	return &ConstantTestNode{
		alphaNode: alphaNode,
		Field:     string(c.AliasAttr),
		V:         c.Value,
		TestOp:    c.TestOp,
	}
}

func (n *ConstantTestNode) Hash() uint64 {
	return hashAny(n)
}

func (n *ConstantTestNode) PerformTest(w *WME) (bool, error) {
	val2test, err := w.GetAttrValue(n.Field)
	if err != nil {
		return false, err
	}
	fn := n.TestOp.ToFunc()
	return fn(n.V, val2test), nil
}

func (t *ConstantTestNode) Adjust(c Cond) {}

func (t *ConstantTestNode) GetNegativeNode() *NegativeTestNode {
	return t.negativeNode
}

func (t *ConstantTestNode) SetNegativeNode(n *NegativeTestNode) {
	if n == nil {
		t.alphaNode.RemoveChild(t.negativeNode)
		t.negativeNode = nil
		return
	}
	// add it into t.children
	t.alphaNode.AddChild(n)
	t.negativeNode = n
}

func (t *ConstantTestNode) RemoveChild(child AlphaNode) {
	if child == t.negativeNode {
		t.SetNegativeNode(nil)
		return
	}
	t.alphaNode.RemoveChild(child)
}

type NegativeTestNode struct {
	*alphaNode
}

var _ AlphaNode = (*NegativeTestNode)(nil)

func newNegativeTestNode(alphaNode *alphaNode, c Cond) *NegativeTestNode {
	return &NegativeTestNode{
		alphaNode: alphaNode,
	}
}

func (n *NegativeTestNode) Hash() uint64 {
	return hashAny(n)
}

func (n *NegativeTestNode) PerformTest(w *WME) (bool, error) {
	return true, nil
}

func (t *NegativeTestNode) Adjust(c Cond) {}
