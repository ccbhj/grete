package rete

type (
	Token struct {
		parent *Token
		wme    *WME // dummy node if wme == nil

		node     ReteNode // point to the memory that the token's in
		children []*Token

		// joinResults *list.List[*NegativeJoinResult] // used only on tokens in negative nodes
		// nccResult *list.List[*Token] // similar to JoinNode but used only in NCC node
		// Owner *Token // on tokens in NCC partner: token in whose nccResult this result reside
	}

	BetaNode interface {
		// LeftActivate notify when there is an token found,
		// which means an early conditions match found
		LeftActivate(token *Token, wme *WME) int
	}
)

func makeToken(node ReteNode, parent *Token, wme *WME) *Token {
	token := &Token{
		parent: parent,
		wme:    wme,
		node:   node,
	}

	parent.children = append(parent.children, token)
	if wme != nil {
		wme.tokens = append(wme.tokens, token) // for tree-based removal
	}

	return token
}

// BetaNode
type (
	BetaMem struct {
		ReteNode
		items         []*Token
		rightUnlinked bool
	}
)

func NewBetaMem(parent ReteNode, items ...*Token) *BetaMem {
	bm := &BetaMem{
		items: items,
	}
	bm.ReteNode = NewReteNode(parent, bm)
	return bm
}

func NewDummyBetaMem() *BetaMem {
	tk := &Token{}
	return NewBetaMem(nil, tk)
}

func buildOrShareBetaMem(parent ReteNode) *BetaMem {
	var bm *BetaMem
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
	// updateNewNodeWithMatchesFromAbove(bm)
	return bm
}

func (bm *BetaMem) Remove() {
	panic("TODO")
	// for len(bm.items) > 0 {
	// 	deleteTokenAndDescendents(m.items[0])
	// }
}

func (bm *BetaMem) LeftActivate(token *Token, wme *WME) int {
	newToken := makeToken(bm, token, wme)
	bm.items = append(bm.items, newToken)

	ret := 0
	bm.ForEachChild(func(child ReteNode) (stop bool) {
		if bn, ok := child.(BetaNode); ok {
			ret += bn.LeftActivate(token, wme)
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
		cmpFn            TestFunc
	}

	JoinNode struct {
		ReteNode
		amem                        *AlphaMem
		tests                       []*TestAtJoinNode
		nearestAncestorWithSameAmem ReteNode
	}
)

func NewJoinNode(parent ReteNode, amem *AlphaMem,
	tests []*TestAtJoinNode, nearestAncestorWithSameAmem ReteNode) *JoinNode {
	jn := &JoinNode{
		amem:                        amem,
		tests:                       tests,
		nearestAncestorWithSameAmem: nearestAncestorWithSameAmem,
	}
	jn.ReteNode = NewReteNode(parent, jn)

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
		if !thisTest.cmpFn(arg1, arg2) {
			return false
		}
	}

	return true
}

func (n *JoinNode) RightActivate(w *WME) int {
	var (
		bm  = n.Parent().(*BetaMem)
		am  = n.amem
		ret = 0
	)

	// just become nonempty
	if items := am.items; !isListEmpty(items) && items.Front == am.items.Back {
		// relink to AlphaMem
	}

	for i := 0; i < len(bm.items); i++ {
		tk := bm.items[i]
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
		items []*Token
	}
)

func NewPNode(parent ReteNode) *PNode {
	pn := &PNode{
		items: make([]*Token, 0, 1),
	}

	pn.ReteNode = NewReteNode(parent, pn)
	return pn
}

func (pn *PNode) LeftActivate(token *Token, wme *WME) int {
	newToken := makeToken(pn, token, wme)
	pn.items = append(pn.items, newToken)
	return 1
}
