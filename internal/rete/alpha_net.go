package rete

import (
	"reflect"

	"github.com/pkg/errors"

	"github.com/ccbhj/grete/internal/log"
	"github.com/zyedidia/generic/list"
)

type (
	// WME is the working memory element, use to store the fact when matching
	WME struct {
		ID    TVIdentity
		Value TestValue

		tokens    set[*Token]
		alphaMems set[*AlphaMem]
		// negativeJoinResults *list.List[*NegativeJoinResult]
	}
)

func NewWME(name TVIdentity, value TestValue) *WME {
	return &WME{
		ID:    name,
		Value: value,

		tokens:    newSet[*Token](),
		alphaMems: newSet[*AlphaMem](),
	}
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
	v, err := val.(*TVStruct).GetField(attr)
	if err != nil {
		if errors.Is(err, ErrFieldNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return v, nil
}

type (
	AlphaMemSuccesor interface {
		// RightActivate notify there is an new WME added
		// which means there is a new fact comming
		RightActivate(w *WME) int
	}

	// AlphaMem store those WMEs that passes constant test
	AlphaMem struct {
		cond           Cond
		inputAlphaNode AlphaNode
		items          set[*WME]                    // wmes that passed tests of ConstantTestNode
		successors     *list.List[AlphaMemSuccesor] // ordered
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
}

type AlphaNetworkOption struct {
	LazyDestory bool // whether to reserve alpha mem that is dangling when destorying
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

func (n *AlphaNetwork) addWME(sum uint64, w *WME) int {
	n.workingMems[sum] = w
	return n.activateAlphaNode(n.root, w)
}

func (n *AlphaNetwork) activateAlphaNode(node AlphaNode, w *WME) int {
	testOk, err := node.PerformTest(w)
	if err != nil {
		log.L("fail to perform test on wme(%+v): %s", w, err)
		return 0
	}
	if !testOk {
		return 0
	}

	if mem := node.OutputMem(); mem != nil {
		mem.Activate(w)
	}

	ret := 0
	node.ForEachChild(func(child AlphaNode) (stop bool) {
		ret += n.activateAlphaNode(child, w)
		return false
	})
	return ret
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

func (n *AlphaNetwork) buildOrShareTestNode(parent AlphaNode, c Cond, makeFn func(*alphaNode, Cond) AlphaNode) AlphaNode {
	base := newAlphaNode(parent)
	an := makeFn(base, c)
	if _, ok := an.(sharableAlphaNode); !ok {
		parent.AddChild(an)
		return an
	}
	h := an.Hash()
	child := parent.GetChild(h)
	if child != nil {
		if shareNode, ok := child.(sharableAlphaNode); ok {
			shareNode.Adjust(c)
			return child
		}
	}

	parent.AddChild(an)
	return an
}

func (n *AlphaNetwork) MakeAlphaMem(c Cond) *AlphaMem {
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
	}

	// finish building test node
	if am := currentNode.OutputMem(); am != nil {
		return am
	}

	am := newAlphaMem(c, currentNode, n)
	currentNode.SetOutputMem(am)
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

// Alpha test nodes
type (
	alphaNode struct {
		parent    AlphaNode            // parent, nil for the root node
		children  map[uint64]AlphaNode // children node
		outputMem *AlphaMem
	}

	AlphaNode interface {
		Hash() uint64
		PerformTest(*WME) (bool, error)
		Parent() AlphaNode
		SetParent(AlphaNode)
		IsChild(child AlphaNode) bool
		AddChild(child AlphaNode)
		GetChild(hash uint64) AlphaNode
		RemoveChild(child AlphaNode)
		NChildren() int
		SetOutputMem(mem *AlphaMem)
		OutputMem() *AlphaMem
		ForEachChild(fn func(AlphaNode) (stop bool))
	}

	sharableAlphaNode interface {
		Adjust(c Cond)
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

func (n *alphaNode) IsChild(child AlphaNode) bool {
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
	FieldConstraints map[string]int // field names we learned from conditions, and its reference count(count of cond)
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
	return false, nil
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
	*alphaNode `hash:"ignore"`
	Field      string    // which field in WME we gonna test
	V          TestValue // the value to be compared
	TestOp     TestOp    // test operation
}

var _ AlphaNode = (*ConstantTestNode)(nil)

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
	val2test, err := w.Value.GetField(n.Field)
	if err != nil {
		return false, err
	}
	fn := n.TestOp.ToFunc()
	return fn(n.V, val2test), nil
}

func (t *ConstantTestNode) Adjust(c Cond) {}
