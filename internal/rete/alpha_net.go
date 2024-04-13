package rete

import (
	"reflect"

	"github.com/pkg/errors"

	"github.com/zyedidia/generic/list"

	"github.com/ccbhj/grete/internal/log"
	. "github.com/ccbhj/grete/internal/types"
)

type (
	// WME is the working memory element, use to store the fact when matching
	WME struct {
		ID    GVIdentity
		Value GValue

		tokens    set[*Token]
		alphaMems set[*AlphaMem]
	}
)

func NewWME(name GVIdentity, value GValue) *WME {
	return &WME{
		ID:    name,
		Value: value,

		tokens:    newSet[*Token](),
		alphaMems: newSet[*AlphaMem](),
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
	if val.Type() != GValueTypeStruct {
		return false
	}
	return val.(*GVStruct).HasField(attr)
}

func (w *WME) GetAttrValue(attr string) (GValue, error) {
	v, err := w.getAttrValue(attr, false)
	if err != nil {
		return nil, err
	}

	return v.(GValue), nil
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
	if val.Type() != GValueTypeStruct {
		return nil, errors.WithMessagef(ErrFieldNotFound, "for type is %s", val.Type().String())
	}

	var ret any
	v, rv, err := val.(*GVStruct).GetField(attr)
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

// _destory should only be called by an AlphaNetwork, who manages all the WMEs
func (w *WME) _destory() {
	w.clearAlphaMems()
	w.clearTokens()
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

type (
	alphaMemSuccesor interface {
		// RightActivate notify there is an new WME added
		// which means there is a new fact comming
		rightActivate(w *WME) int
	}

	// AlphaMem store those WMEs that passes constant test
	AlphaMem struct {
		typeInfo       TypeInfo
		guards         []Guard
		inputAlphaNode AlphaNode
		items          set[*WME]                    // wmes that passed tests of ConstantTestNode
		successors     *list.List[alphaMemSuccesor] // must be ordered, see Figure 2.5 in paper 2.4
		an             *AlphaNetwork                // which AlphaNetwork this mem belong to
	}
)

func newAlphaMem(typeInfo TypeInfo, guards []Guard, input AlphaNode, an *AlphaNetwork) *AlphaMem {
	am := &AlphaMem{
		typeInfo:       typeInfo,
		guards:         guards,
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
	m.items.Del(w)
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
		if tn, ok := current.(*TypeTestNode); ok {
			delete(m.an.typeNodes, tn.Hash())
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
	typeNodes     map[uint64]*TypeTestNode
}

func NewAlphaNetwork() *AlphaNetwork {
	root := newAlphaNode(nil)
	alphaNet := &AlphaNetwork{
		root:          root,
		cond2AlphaMem: make(map[uint64]*AlphaMem),
		workingMems:   make(map[uint64]*WME),
		typeNodes:     make(map[uint64]*TypeTestNode),
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
	// clear all the alpha memories
	w.alphaMems.ForEach(func(am *AlphaMem) {
		am.removeWME(w)
	})
	w.alphaMems.Clear()
	w._destory()
}

func (n *AlphaNetwork) buildOrShareNegativeTestNode(c Guard, nn negatableAlphaNode) AlphaNode {
	negativeNode := nn.GetNegativeNode()
	if negativeNode != nil {
		return negativeNode
	}

	base := newAlphaNode(nn.(AlphaNode))
	newNode := newNegativeTestNode(base, c)
	nn.SetNegativeNode(newNode)
	return newNode
}

func (n *AlphaNetwork) MakeAlphaMem(aliasType TypeInfo, guards []Guard) *AlphaMem {
	var err error
	h := n.hashGuards(aliasType, guards)
	if am, in := n.cond2AlphaMem[h]; in {
		return am
	}

	var (
		currentNode = n.root
	)

	// test type
	th := aliasType.Hash()
	tn, in := n.typeNodes[th]
	if in {
		if currentNode.IsParentOf(tn) {
			currentNode = tn
		} else {
			defer func() {
				if err != nil {
					n.root.RemoveChild(tn)
				}
			}()
			currentNode.AddChild(tn)
			currentNode = tn
		}
	} else {
		tn := NewTypeTestNode(newAlphaNode(currentNode), aliasType)
		defer func() {
			if err != nil {
				n.root.RemoveChild(tn)
			}
		}()
		n.typeNodes[tn.Hash()] = tn.(*TypeTestNode)
		n.root.AddChild(tn)
		currentNode = tn
	}
	for _, g := range guards {
		if g.Value.Type() == GValueTypeIdentity {
			panic("alias as value is not allowed in guard")
		}
		base := newAlphaNode(currentNode)
		newNode := NewConstantTestNode(base, g)
		if cached := currentNode.GetChild(newNode.Hash()); cached != nil {
			currentNode = cached
		} else {
			parent := currentNode
			defer func(child, parent AlphaNode) {
				if err != nil {
					log.L("fail to build ConvertibleTo: %s", err)
					parent.RemoveChild(child)
				}
			}(newNode, parent)
			parent.AddChild(newNode)
			currentNode = newNode
		}
		if g.Negative {
			nnode, ok := currentNode.(negatableAlphaNode)
			if !ok {
				// TODO: handle error
				panic("node not supported negation")
			}
			currentNode = n.buildOrShareNegativeTestNode(g, nnode)
		}
	}

	if am := currentNode.OutputMem(); am != nil {
		return am
	}

	am := newAlphaMem(aliasType, guards, currentNode, n)
	currentNode.SetOutputMem(am)
	n.cond2AlphaMem[h] = am
	return am
}

// initialize am with any current working memory
func (n *AlphaNetwork) InitAlphaMem(am *AlphaMem) {
	node := am.inputAlphaNode
	// find the top AlphaNode, but not the root, to avoid massive activation as possible as we can
	for node != nil {
		if node.Parent() == n.root {
			break
		}
		node = node.Parent()
	}
	if node != nil {
		for _, w := range n.workingMems {
			n.activateAlphaNode(node, w)
		}
	}
}

func (n *AlphaNetwork) InitDummyAlphaMem(am *AlphaMem, c Guard) {
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

func (n *AlphaNetwork) DestoryAlphaMem(alphaMem *AlphaMem) {
	h := n.hashGuards(alphaMem.typeInfo, alphaMem.guards)
	if _, in := n.cond2AlphaMem[h]; !in {
		return
	}
	alphaMem._destory()
	delete(n.cond2AlphaMem, h)
}

func (n *AlphaNetwork) hashGuards(typeInfo TypeInfo, guards []Guard) uint64 {
	ret := typeInfo.Hash()

	for _, g := range guards {
		ret = mix64(ret, g.Hash())
	}
	return ret
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
	*alphaNode `hash:"ignore"`
	TypeInfo   TypeInfo `hash:"ignore"` // TypeInfo specified in a Cond
}

var _ AlphaNode = (*TypeTestNode)(nil)

func NewTypeTestNode(alphaNode *alphaNode, tf TypeInfo) AlphaNode {
	return &TypeTestNode{
		alphaNode: alphaNode,
		TypeInfo:  tf,
	}
}

func (t *TypeTestNode) PerformTest(w *WME) (bool, error) {
	switch t.TypeInfo.T {
	case GValueTypeInt, GValueTypeUint,
		GValueTypeFloat, GValueTypeString:
		return t.TypeInfo.T == w.Value.Type(), nil
	case GValueTypeStruct:
		return w.Value.Type() == GValueTypeStruct && t.checkStructType(w), nil
	case GValueTypeUnknown:
		return false, errors.New("invalid TypeInfo, T cannot be GValueTypeUnknown")
	}
	return true, nil
}

func (t *TypeTestNode) checkStructType(w *WME) bool {
	tf := t.TypeInfo
	v := w.Value.(*GVStruct).V
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
		if t == GValueTypeUnknown || t == GValueTypeStruct {
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
	return t.TypeInfo.Hash()
}

// ConstantTestNode perform constant test for each WME,
// and implements by 'Dataflow Network'(see 2.2.1(page 27) in the paper)
type ConstantTestNode struct {
	*alphaNode   `hash:"ignore"`
	negativeNode *NegativeTestNode `hash:"ignore"`
	Field        string            // which field in WME we gonna test
	V            GValue            // the value to be compared
	TestOp       TestOp            // test operation
}

var _ AlphaNode = (*ConstantTestNode)(nil)
var _ negatableAlphaNode = (*ConstantTestNode)(nil)

func NewConstantTestNode(alphaNode *alphaNode, g Guard) AlphaNode {
	return &ConstantTestNode{
		alphaNode: alphaNode,
		Field:     string(g.AliasAttr),
		V:         g.Value,
		TestOp:    g.TestOp,
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
	return fn(n.V, val2test)
}

func (t *ConstantTestNode) Adjust(c Guard) {}

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

func newNegativeTestNode(alphaNode *alphaNode, c Guard) *NegativeTestNode {
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

func (t *NegativeTestNode) Adjust(c Guard) {}
