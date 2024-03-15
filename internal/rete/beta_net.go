package rete

import (
	"errors"

	"github.com/ccbhj/grete/internal/log"
)

type (
	Token struct {
		parent *Token
		wme    *WME // dummy node if wme == nil

		node     ReteNode // point to the memory that the token's in
		children set[*Token]

		// joinResults *list.List[*NegativeJoinResult] // used only on tokens in negative nodes
		// nccResult *list.List[*Token] // similar to JoinNode but used only in NCC node
		// Owner *Token // on tokens in NCC partner: token in whose nccResult this result reside
	}

	TokenMemory interface {
		ReteNode
		removeToken(*Token)
	}

	BetaNode interface {
		ReteNode
		// leftActivate notify when there is an token found,
		// which means an early conditions match found
		leftActivate(token *Token, wme *WME) int
		// detach unlink a BetaNode from it parent and children
		detach()
	}

	BetaNetwork struct {
		an          *AlphaNetwork
		topNode     ReteNode
		workingMem  map[uint64]*WME
		productions map[string]*PNode
	}
)

func NewToken(node ReteNode, parent *Token, wme *WME) *Token {
	token := &Token{
		parent:   parent,
		wme:      wme,
		node:     node,
		children: newSet[*Token](),
	}

	if parent != nil {
		parent.children.Add(token)
	}
	if wme != nil {
		wme.tokens.Add(token)
	}

	return token
}

func (t *Token) toWMEs() []*WME {
	wmes := make([]*WME, 0, 4)
	for p := t; p != nil && p.wme != nil; p = p.parent {
		wmes = append(wmes, p.wme)
	}
	return wmes
}

func (t *Token) destory() {
	// clean children
	for child := range t.children {
		child.destory()
	}
	t.children = nil

	// remove token from TokenMemory
	node := t.node
	if tm, ok := node.(TokenMemory); ok {
		tm.removeToken(t)
		// TODO: try right unlink here
	}
	t.node = nil

	// remove token from list of tok.wme.tokens if not dummy node
	if t.wme != nil {
		t.wme.tokens.Del(t)
		t.wme = nil
	}

	// remove tok from the list of tok.parent.children if not dummy node
	if t.parent != nil {
		pc := t.parent.children
		pc.Del(t)
		t.parent = nil
	}
}

// BetaNode
type (
	BetaMem struct {
		ReteNode
		items         set[*Token]
		rightUnlinked bool
	}
)

var _ TokenMemory = (*BetaMem)(nil)
var _ BetaNode = (*BetaMem)(nil)

func (bm *BetaMem) removeToken(token *Token) {
	if bm.isDummyBetaMem() {
		// do not remove the dummy token in a dummy betaMem
		return
	}
	bm.items.Del(token)
}

func NewBetaMem(parent ReteNode) *BetaMem {
	bm := &BetaMem{
		items: newSet[*Token](),
	}
	bm.ReteNode = NewReteNode(parent, bm)
	return bm
}

func newDummyBetaMem() *BetaMem {
	bm := NewBetaMem(nil)
	tk := NewToken(bm, nil, nil)
	bm.items.Add(tk)
	return bm
}

func (bm *BetaMem) isDummyBetaMem() bool {
	return bm.Parent() == nil
}

func (bm *BetaMem) detach() {
	if bm.isDummyBetaMem() {
		return
	}
	for item := range bm.items {
		item.destory()
	}
	bm.items.Clear()
}

func (bm *BetaMem) leftActivate(token *Token, wme *WME) int {
	newToken := NewToken(bm, token, wme)
	bm.items.Add(newToken)

	ret := 0
	bm.ForEachChild(func(child ReteNode) (stop bool) {
		if bn, ok := child.(BetaNode); ok {
			ret += bn.leftActivate(newToken, wme)
		}
		return false
	})

	return ret
}

type (
	// JoinTestType specify which field of a WME we are going to be tested at the JoinTestNode
	JoinTestType   int
	TestAtJoinNode struct {
		LhsAttr      string // Attrs of Lhs and RhsAttr
		RhsAttr      string // Attrs of Rhs and RhsAttr, rhs is usually a wme that matches previous conds.
		CondOffRhs   int    // the offset in a token list where the Y is extract from, which is matched the exact cond that refer the lhs's alias.
		TestOp       TestOp // TestOp to perform on Lhs and Rhs, the order matters
		ReverseOrder bool   // whether to reverse order when performing test on rhs and lhs
	}

	JoinNode struct {
		ReteNode
		amem                        *AlphaMem
		tests                       []*TestAtJoinNode
		testSum                     uint64
		nearestAncestorWithSameAmem ReteNode
		betaMem                     *BetaMem // one of then children, for speeding up the construction
		bn                          *BetaNetwork
	}
)

func (t TestAtJoinNode) Equal(other TestAtJoinNode) bool {
	return t.LhsAttr == other.LhsAttr &&
		t.RhsAttr == other.RhsAttr &&
		t.CondOffRhs == other.CondOffRhs
}

func (t TestAtJoinNode) Hash() uint64 {
	return hashAny(t)
}

func (t TestAtJoinNode) performTest(x, y *WME) bool {
	xv, err := x.GetAttrValue(t.LhsAttr)
	if err != nil {
		log.L("fail to get %s of X: %s", t.LhsAttr, err)
		return false
	}
	log.BugOn(xv != nil, "attr %s of X must not be nil", t.LhsAttr)

	yv, err := y.GetAttrValue(t.RhsAttr)
	if err != nil {
		log.L("fail to get %s of Y: %s", t.RhsAttr, err)
		return false
	}
	log.BugOn(yv != nil, "attr %s of Y must not be nil", t.RhsAttr)

	testFn := t.TestOp.ToFunc()
	if t.ReverseOrder {
		return testFn(yv, xv)
	}
	return testFn(xv, yv)
}

func buildJoinTestFromConds(c Cond, prevConds []Cond) []*TestAtJoinNode {
	ret := make([]*TestAtJoinNode, 0, 2)
	id, val := c.Alias, c.Value
	// each condition will mapped to a token
	for i := len(prevConds) - 1; i >= 0; i-- {
		addNode := func(lhs, rhs string, testOp TestOp, ro bool) {
			ret = append(ret, &TestAtJoinNode{
				LhsAttr:      lhs,
				RhsAttr:      rhs,
				CondOffRhs:   len(prevConds) - i - 1,
				TestOp:       testOp,
				ReverseOrder: ro,
			})
		}
		prevCond := prevConds[i]

		if prevCond.Alias == id {
			// JT(ID, ID)
			addNode(FieldID, FieldID, TestOpEqual, false)
		}
		if prevCond.Value.Type() == TestValueTypeIdentity && prevCond.Value == id {
			// JT(ID, Value) -> JT(Self, Value)
			// When performing test between ID and Value, we should use FieldSelf instead of FieldID,
			// Attention: the lhs and rhs order must be reversed here
			addNode(FieldSelf, string(prevCond.AliasAttr), prevCond.TestOp, true)
		}

		if isIdentity(val) {
			switch {
			case prevCond.Alias == val:
				// JT(Value, ID)
				// When performing test between ID and Value, we should use FieldSelf instead of FieldID
				addNode(string(c.AliasAttr), FieldSelf, c.TestOp, false)
			case isIdentity(prevCond.Value) && prevCond.Value == val:
				// JT(Value, Value)
				addNode(string(c.AliasAttr), string(prevCond.AliasAttr), TestOpEqual, false)
			}
		}
	}

	return ret
}

var _ BetaNode = (*JoinNode)(nil)

func newJoinNode(bn *BetaNetwork, parent ReteNode, amem *AlphaMem,
	tests []*TestAtJoinNode, nearestAncestorWithSameAmem ReteNode) *JoinNode {
	jn := &JoinNode{
		bn:                          bn,
		amem:                        amem,
		tests:                       tests,
		nearestAncestorWithSameAmem: nearestAncestorWithSameAmem,
		testSum:                     calJoinTestSum(tests),
	}
	jn.ReteNode = NewReteNode(parent, jn)
	// order is matter
	amem.AddSuccessor(jn)
	return jn
}

func calJoinTestSum(tests []*TestAtJoinNode) uint64 {
	sum := uint64(len(tests))
	for _, t := range tests {
		sum = mix64(sum, t.Hash())
	}
	return sum
}

func performTests(tests []*TestAtJoinNode, tk *Token, wme *WME) bool {
	if tk.wme == nil {
		// this is a dummy token(see papar page 25)
		return true
	}
	for _, test := range tests {
		p := tk
		for off := test.CondOffRhs; off > 0; off-- {
			p = p.parent
		}
		log.BugOn(p != nil, "p must never be nil")
		lhs, rhs := wme, p.wme
		if !test.performTest(lhs, rhs) {
			return false
		}
	}

	return true
}

func (n *JoinNode) RightActivate(w *WME) int {
	var (
		bm  = n.Parent().(*BetaMem)
		ret = 0
	)

	// just become nonempty
	// am  := n.amem
	// if items := am.items; !isListEmpty(items) && items.Front == am.items.Back {
	// relink to AlphaMem
	// }

	for tk := range bm.items {
		if performTests(n.tests, tk, w) {
			n.ForEachChildNonStop(func(child ReteNode) {
				if bn, ok := child.(BetaNode); ok {
					ret += bn.leftActivate(tk, w)
				}
			})
		}
	}

	return ret
}

func (n *JoinNode) leftActivate(tk *Token, _ *WME) int {
	var (
		am  = n.amem
		ret = 0
	)

	// try right relink

	if am.items == nil {
		return 0
	}

	am.ForEachItem(func(w *WME) (stop bool) {
		if performTests(n.tests, tk, w) {
			n.ForEachChildNonStop(func(child ReteNode) {
				if bn, ok := child.(BetaNode); ok {
					ret += bn.leftActivate(tk, w)
				}
			})
		}

		return false
	})

	return ret
}

func (n *JoinNode) detach() {
	n.amem.RemoveSuccessor(n)
	// is n.amem dangling?
	if n.amem.IsSuccessorsEmpty() {
		// destory alpha mem through alpha net
		n.bn.an.DestoryAlphaMem(n.amem)
		n.amem = nil
	}
	clear(n.tests)
	n.betaMem = nil
	n.bn = nil
}

type (
	PNode struct {
		ReteNode
		items set[*Token]
		lhs   []Cond
	}
)

var _ TokenMemory = (*PNode)(nil)
var _ BetaNode = (*PNode)(nil)

func NewPNode(parent ReteNode, lhs []Cond) *PNode {
	pn := &PNode{
		items: newSet[*Token](),
		lhs:   lhs,
	}

	pn.ReteNode = NewReteNode(parent, pn)
	return pn
}

func (pn *PNode) leftActivate(token *Token, wme *WME) int {
	newToken := NewToken(pn, token, wme)
	pn.items.Add(newToken)
	return 1
}

func (pn *PNode) detach() {
	for item := range pn.items {
		item.destory()
	}
	pn.items.Clear()
}

// AnyMatches check if there is any match in a production node
func (pn *PNode) AnyMatches() bool {
	return pn.items.Len() > 0
}

func (pn *PNode) Matches() []map[TVIdentity]Fact {
	matches := make([]map[TVIdentity]Fact, 0, len(pn.items))
	for item := range pn.items {
		j := len(pn.lhs) - 1
		match := make(map[TVIdentity]Fact, len(pn.lhs))
		for ; item.wme != nil; item, j = item.parent, j-1 {
			id := pn.lhs[j].Alias
			wme := item.wme
			match[id] = wme.FactOfWME()
		}
		matches = append(matches, match)
	}

	return matches
}

func (pn *PNode) removeToken(token *Token) {
	pn.items.Del(token)
}

func NewBetaNetwork(an *AlphaNetwork) *BetaNetwork {
	return &BetaNetwork{
		an:          an,
		topNode:     newDummyBetaMem(),
		workingMem:  make(map[uint64]*WME),
		productions: make(map[string]*PNode),
	}
}

func (bn *BetaNetwork) buildOrShareNetwork(parent ReteNode, conds []Cond, earlierConds []Cond) ReteNode {
	var (
		currentNode   ReteNode
		condsHigherUp []Cond
	)

	currentNode = parent
	condsHigherUp = earlierConds
	for _, c := range conds {
		switch {
		case !c.Negative /* TODO: exclude NCCNode */ :
			currentNode = bn.buildOrShareBetaMem(currentNode)
			currentNode = bn.buildOrShareJoinNode(currentNode,
				bn.an.MakeAlphaMem(c),
				buildJoinTestFromConds(c, condsHigherUp))
		case c.Negative:
			// TODO: support negative node
		}
		condsHigherUp = append(condsHigherUp, c)
	}

	return currentNode
}

func (bn *BetaNetwork) buildOrShareJoinNode(parent ReteNode, am *AlphaMem, tests []*TestAtJoinNode) *JoinNode {
	var (
		rn      = parent
		hitNode *JoinNode
		testSum = calJoinTestSum(tests)
	)
	rn.ForEachChild(func(child ReteNode) (stop bool) {
		jn, ok := child.(*JoinNode)
		if !ok {
			return false
		}
		// compare alpha memory and tests
		if jn.amem == am && jn.testSum == testSum {
			hitNode = jn
			return true
		}
		return false
	})
	if hitNode != nil {
		return hitNode
	}

	// TODO: set the nearestAncestorWithSameAmem
	jn := newJoinNode(bn, parent, am, tests, nil)
	return jn
}

func (bn *BetaNetwork) buildOrShareBetaMem(parent ReteNode) *BetaMem {
	if parent == nil {
		return newDummyBetaMem()
	} else if bm, ok := parent.(*BetaMem); ok {
		// an BetaMem mustn't be another BetaMem, unless parent is bn.topNode(a dummy BetaMem)
		return bm
	}

	var (
		bm             *BetaMem
		parentJoinNode *JoinNode
	)
	if jn, ok := parent.(*JoinNode); ok {
		parentJoinNode = jn
	}
	if parentJoinNode != nil && parentJoinNode.betaMem != nil {
		return parentJoinNode.betaMem
	}
	parent.ForEachChild(func(child ReteNode) (stop bool) {
		if m, ok := child.(*BetaMem); ok {
			bm = m
			return true
		}
		return false
	})

	if bm != nil {
		return bm
	}

	bm = NewBetaMem(parent)
	bn.updateNewNodeWithMatchesFromAbove(bm)
	parentJoinNode.betaMem = bm
	return bm
}

func (bn *BetaNetwork) updateNewNodeWithMatchesFromAbove(newNode ReteNode) {
	switch parent := newNode.Parent().(type) {
	case *BetaMem:
		// directly check all the items from parent and matched them
		for tok := range parent.items {
			if bn, ok := newNode.(BetaNode); ok {
				bn.leftActivate(tok, nil)
			}
		}
	case *JoinNode:
		// pretend that parent only has one child which is newNode temporarily
		// and then RightActivate parent to allow any new matches propageted to
		// newNode
		if parent.amem != nil {
			parent.ClearAndRestoreChildren(func() {
				parent.AddChild(newNode)
				parent.amem.ForEachItem(func(w *WME) (stop bool) {
					parent.RightActivate(w)
					return
				})
			})
		}
		// case *NegativeNode: // TODO:  support negative node
		// 	for _, tok := range parent.items {
		// 		if isListEmpty(tok.joinResults) {
		// 			newNode.LeftActivate(tok, nil)
		// 		}
		// 	}
	}
}

// AddFact add a fact, and propagete addition to the entire network
func (bn *BetaNetwork) AddFact(fact Fact) {
	bn.an.AddFact(fact)
}

// RemoveFact remove a fact, and propagete removal to the entire network
func (bn *BetaNetwork) RemoveFact(fact Fact) {
	bn.an.RemoveFact(fact)
}

// AddProduction add an production and register its unique id
func (bn *BetaNetwork) AddProduction(id string, lhs ...Cond) *PNode {
	if len(lhs) == 0 {
		panic("need some condition")
	}

	if pn, in := bn.productions[id]; in {
		return pn
	}

	currentNode := bn.buildOrShareNetwork(bn.topNode, lhs, nil)
	pn := NewPNode(currentNode, lhs)
	bn.updateNewNodeWithMatchesFromAbove(pn)
	bn.productions[id] = pn
	return pn
}

// GetProduction query an production by its id
func (bn *BetaNetwork) GetProduction(id string) *PNode {
	n, in := bn.productions[id]
	if !in {
		return nil
	}
	return n
}

// RemoveProduction remove a production by a production id, along with all the facts
// related with it.
// TODO: provide an option for reserving fact, we can just delete the pnode and its parent, grandparent and grand..grandparent if they are not shared, and leave the alpha mem so that it can still be shared and will not activate any successors if there is none
func (bn *BetaNetwork) RemoveProduction(id string) error {
	pnode, in := bn.productions[id]
	if !in {
		return errors.New("production not found")
	}
	delete(bn.productions, id)
	bn.removeProduction(pnode)
	return nil
}

func (bn *BetaNetwork) removeProduction(pnode *PNode) {
	bn.deleteNodeAndAnyUnusedAncestors(pnode)
}

func (bn *BetaNetwork) deleteNodeAndAnyUnusedAncestors(node BetaNode) {
	node.detach()

	parent := node.Parent()
	if parent != nil {
		parent.RemoveChild(node)
		if !parent.AnyChild() {
			if bnode, ok := parent.(BetaNode); ok {
				bn.deleteNodeAndAnyUnusedAncestors(bnode)
			}
		}
	}
	node.DetachParent()
}
