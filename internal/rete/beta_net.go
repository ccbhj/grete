package rete

type (
	Fact struct {
		ID    TVIdentity
		Field TVString
		Value TestValue
	}

	Token struct {
		parent *Token
		wme    *WME // dummy node if wme == nil

		node     ReteNode // point to the memory that the token's in
		children map[*Token]struct{}

		// joinResults *list.List[*NegativeJoinResult] // used only on tokens in negative nodes
		// nccResult *list.List[*Token] // similar to JoinNode but used only in NCC node
		// Owner *Token // on tokens in NCC partner: token in whose nccResult this result reside
	}

	TokenMemory interface {
		ReteNode
		RemoveToken(*Token)
	}

	BetaNode interface {
		// LeftActivate notify when there is an token found,
		// which means an early conditions match found
		LeftActivate(token *Token, wme *WME) int
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
		children: make(map[*Token]struct{}),
	}

	if parent != nil {
		parent.children[token] = struct{}{}
	}
	if wme != nil {
		wme.tokens[token] = struct{}{}
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
		tm.RemoveToken(t)
		// TODO: try right unlink here
	}
	t.node = nil

	// remove token from list of tok.wme.tokens if not dummy node
	if t.wme != nil {
		delete(t.wme.tokens, t)
		t.wme = nil
	}

	// remove tok from the list of tok.parent.children if not dummy node
	if t.parent != nil {
		pc := t.parent.children
		delete(pc, t)
		t.parent = nil
	}
}

// BetaNode
type (
	BetaMem struct {
		ReteNode
		items         map[*Token]struct{}
		rightUnlinked bool
	}
)

var _ TokenMemory = (*BetaMem)(nil)

func (bm *BetaMem) RemoveToken(token *Token) {
	delete(bm.items, token)
}

func NewBetaMem(parent ReteNode) *BetaMem {
	bm := &BetaMem{
		items: make(map[*Token]struct{}),
	}
	bm.ReteNode = NewReteNode(parent, bm)
	return bm
}

func newDummyBetaMem() *BetaMem {
	bm := NewBetaMem(nil)
	tk := NewToken(bm, nil, nil)
	bm.addToken(tk)
	return bm
}

func (bm *BetaMem) addToken(tk *Token) {
	bm.items[tk] = struct{}{}
}

func (bm *BetaMem) removeToken(tk *Token) {
	delete(bm.items, tk)
}

func (bm *BetaMem) Remove() {
	panic("TODO")
	// for len(bm.items) > 0 {
	// 	deleteTokenAndDescendents(m.items[0])
	// }
}

func (bm *BetaMem) LeftActivate(token *Token, wme *WME) int {
	newToken := NewToken(bm, token, wme)
	bm.addToken(newToken)

	ret := 0
	bm.ForEachChild(func(child ReteNode) (stop bool) {
		if bn, ok := child.(BetaNode); ok {
			ret += bn.LeftActivate(newToken, wme)
		}
		return false
	})

	return ret
}

type (
	TestAtJoinNode struct {
		fieldOfArg1      TestType
		fieldOfArg2      TestType
		condOffsetOfArg2 int
	}

	JoinNode struct {
		ReteNode
		amem                        *AlphaMem
		tests                       []*TestAtJoinNode
		nearestAncestorWithSameAmem ReteNode
		betaMem                     *BetaMem // for speeding up the construction
	}
)

func (t TestAtJoinNode) equal(other TestAtJoinNode) bool {
	return t.fieldOfArg1 == other.fieldOfArg1 &&
		t.fieldOfArg2 == other.fieldOfArg2 &&
		t.condOffsetOfArg2 == other.condOffsetOfArg2
}

func NewJoinNode(parent ReteNode, amem *AlphaMem,
	tests []*TestAtJoinNode, nearestAncestorWithSameAmem ReteNode) *JoinNode {
	jn := &JoinNode{
		amem:                        amem,
		tests:                       tests,
		nearestAncestorWithSameAmem: nearestAncestorWithSameAmem,
	}
	jn.ReteNode = NewReteNode(parent, jn)
	// order is matter
	amem.AddSuccessor(jn)
	return jn
}

func performTests(tests []*TestAtJoinNode, tk *Token, wme *WME) bool {
	if tk.wme == nil {
		// this is a dummy token(see papar page 25)
		return true
	}
	for i := 0; i < len(tests); i++ {
		thisTest := tests[i]
		arg1 := thisTest.fieldOfArg1.GetField(wme)

		p := tk
		for off := thisTest.condOffsetOfArg2; off > 0; off-- {
			p = p.parent
		}
		otherWME := p.wme
		arg2 := thisTest.fieldOfArg2.GetField(otherWME)
		if !TestEqual(arg1, arg2) {
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
					ret += bn.LeftActivate(tk, w)
				}
			})
		}
	}

	return ret
}

func (n *JoinNode) LeftActivate(tk *Token, _ *WME) int {
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
					ret += bn.LeftActivate(tk, w)
				}
			})
		}

		return false
	})

	return ret
}

type (
	PNode struct {
		ReteNode
		items map[*Token]struct{}
		lhs   []Cond
	}
)

var _ TokenMemory = (*PNode)(nil)

func NewPNode(parent ReteNode, lhs []Cond) *PNode {
	pn := &PNode{
		items: make(map[*Token]struct{}),
		lhs:   lhs,
	}

	pn.ReteNode = NewReteNode(parent, pn)
	return pn
}

func (pn *PNode) LeftActivate(token *Token, wme *WME) int {
	newToken := NewToken(pn, token, wme)
	pn.items[newToken] = struct{}{}
	return 1
}

func (pn *PNode) AnyMatches() bool {
	return len(pn.items) > 0
}

func (pn *PNode) Matches() []map[TVIdentity]Fact {
	matches := make([]map[TVIdentity]Fact, 0, len(pn.items))
	for item := range pn.items {
		j := len(pn.lhs) - 1
		match := make(map[TVIdentity]Fact, len(pn.lhs))
		for ; item.wme != nil; item, j = item.parent, j-1 {
			id := pn.lhs[j].ID
			wme := item.wme
			match[id] = factOfWME(wme)
		}
		matches = append(matches, match)
	}

	return matches
}

func (pn *PNode) RemoveToken(token *Token) {
	delete(pn.items, token)
}

func NewBetaNetwork(an *AlphaNetwork) *BetaNetwork {
	return &BetaNetwork{
		an:          an,
		topNode:     newDummyBetaMem(),
		workingMem:  make(map[uint64]*WME),
		productions: make(map[string]*PNode),
	}
}

func getJoinTestFromConds(c Cond, prevConds []Cond) []*TestAtJoinNode {
	ret := make([]*TestAtJoinNode, 0, 2)
	id, val := c.ID, c.Value
	// each condition will mapped to a token
	for i := len(prevConds) - 1; i >= 0; i-- {
		addNode := func(fieldOfArg1, fieldOfArg2 TestType) {
			ret = append(ret, &TestAtJoinNode{
				fieldOfArg1:      fieldOfArg1,
				fieldOfArg2:      fieldOfArg2,
				condOffsetOfArg2: len(prevConds) - i - 1,
			})
		}
		prevCond := prevConds[i]

		// check id
		if prevCond.ID == id {
			addNode(TestTypeID, TestTypeID)
		}
		if isIdentity(prevCond.Value) && prevCond.Value == id {
			addNode(TestTypeID, TestTypeValue)
		}

		// check value
		if isIdentity(val) {
			if prevCond.ID == val {
				addNode(TestTypeValue, TestTypeID)
			}
			if isIdentity(prevCond.Value) && prevCond.Value == val {
				addNode(TestTypeValue, TestTypeValue)
			}
		}
	}

	return ret
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
				bn.an.MakeAlphaMem(c, bn.workingMem),
				getJoinTestFromConds(c, condsHigherUp))
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
	)
	rn.ForEachChild(func(child ReteNode) (stop bool) {
		jn, ok := child.(*JoinNode)
		if !ok {
			return false
		}
		// compare alpha memory and tests
		if jn.amem == am && len(jn.tests) == len(tests) {
			for i := 0; i < len(jn.tests); i++ {
				if !jn.tests[i].equal(*tests[i]) {
					return false
				}
			}
			hitNode = jn
			return true
		}
		return false
	})
	if hitNode != nil {
		return hitNode
	}

	// TODO: set the nearestAncestorWithSameAmem
	jn := NewJoinNode(parent, am, tests, nil)
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
				bn.LeftActivate(tok, nil)
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

func (bn *BetaNetwork) AddFact(fact Fact) {
	h := fact.Hash()
	if _, in := bn.workingMem[h]; in {
		return
	}
	w := wmeFromFact(fact)
	bn.workingMem[h] = w
	bn.an.AddWME(w)
}

func (bn *BetaNetwork) RemoveFact(fact Fact) {
	h := fact.Hash()
	w, in := bn.workingMem[h]
	if !in {
		return
	}
	bn.an.RemoveWME(w)
	delete(bn.workingMem, h)
}

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

func (bn *BetaNetwork) GetPNode(id string) *PNode {
	n, in := bn.productions[id]
	if !in {
		return nil
	}
	return n
}

func wmeFromFact(fact Fact) *WME {
	return NewWME(fact.ID, fact.Field, fact.Value)
}

func factOfWME(w *WME) Fact {
	return Fact{
		ID:    w.Name,
		Field: w.Field,
		Value: w.Value,
	}
}

func (f Fact) Hash() uint64 {
	return uint64(mix32(mix32(f.ID.Hash(), f.Field.Hash()), f.Value.Hash()))
}
