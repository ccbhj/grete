package rete

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/ccbhj/grete/internal/log"
	. "github.com/ccbhj/grete/internal/types"
)

type (
	Token struct {
		parent *Token
		level  int
		wme    *WME // dummy node if wme == nil

		node         ReteNode // point to the memory that the token's in
		children     set[*Token]
		nJoinResults set[*negativeJoinResult] // used only on tokens in negative nodes
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
		// which means au,n early conditions match found,
		// CAUTIOUS:  wme could be nil
		leftActivate(token *Token, wme *WME) int
		// detach unlink a BetaNode from its parent, children and tokens associated with it
		detach()
	}

	// BetaNetwork manage the whole beta net and all the BetaNode
	BetaNetwork struct {
		an          *AlphaNetwork
		topNode     ReteNode
		productions map[string]*PNode
	}
)

func newToken(node ReteNode, parent *Token, wme *WME) *Token {
	token := &Token{
		parent:       parent,
		wme:          wme,
		node:         node,
		children:     newSet[*Token](),
		nJoinResults: newSet[*negativeJoinResult](),
	}

	level := 0
	if parent != nil {
		parent.children.Add(token)
		level = parent.level + 1
	}
	if wme != nil {
		wme.tokens.Add(token)
	}

	token.level = level

	return token
}

func (t *Token) Hash() uint64 {
	if t == nil {
		return 0
	}
	return mix64(mix64(t.parent.Hash(), uint64(t.level)), t.wme.Hash())
}

func (t *Token) toWMEs() []*WME {
	wmes := make([]*WME, 0, t.level)
	for p := t; p != nil && p.level > 0; p = p.parent {
		if p.wme == nil {
			continue
		}
		wmes = append(wmes, p.wme)
	}
	return lo.Reverse(wmes)
}

func (t *Token) toWMEIDs() []string {
	return lo.Map(t.toWMEs(), func(item *WME, index int) string { return string(item.ID) })
}

func (t *Token) String() string {
	return fmt.Sprintf("%v", t.toWMEIDs())
}

func (t *Token) destoryDescendents() {
	for child := range t.children {
		child.destory()
	}
	clear(t.children)
}

func (t *Token) destory() {
	// clean children
	t.destoryDescendents()
	t.children.Clear()
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

type (
	// BetaMem holds all the tokens that match all the previous conditions,
	// in another word, all the tokens pass all the join test and usually got activated by an join node
	BetaMem struct {
		ReteNode
		items set[*Token]
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
	tk := newToken(bm, nil, nil)
	bm.items.Add(tk)
	return bm
}

func (bm *BetaMem) isDummyBetaMem() bool {
	return bm.Parent() == nil
}

func (bm *BetaMem) detach() {
	// don't detach the dummy beta mem
	if bm.isDummyBetaMem() {
		return
	}
	for item := range bm.items {
		item.destory()
	}
	bm.items.Clear()
}

func (bm *BetaMem) leftActivate(token *Token, wme *WME) int {
	// a new match found, restore it
	newToken := newToken(bm, token, wme)
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
	TestAtJoinNode struct {
		LhsAttr      string // Attrs of Lhs and RhsAttr
		RhsAttr      string // Attrs of Rhs and RhsAttr, rhs is usually a wme that matches previous conds.
		CondOffRhs   int    // the offset in a token list where the Y is extract from, which is matched the exact cond that refer the lhs's alias.
		TestOp       TestOp // TestOp to perform on Lhs and Rhs, the order matters
		ReverseOrder bool   // whether to reverse order when performing test on rhs and lhs
	}
	// JoinNode perform join tests between wmes from an alpha memory and tokens from beta mem,
	// that means JoinNode is usually the parent of a beta memory(beta memory store the results of JoinNode)
	JoinNode struct {
		ReteNode
		tests     []*TestAtJoinNode
		testSum   uint64
		amem      *AlphaMem
		outputMem *BetaMem // one of then children, for speeding up the construction
		bn        *BetaNetwork
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
			offset := len(prevConds) - i - 1
			ret = append(ret, &TestAtJoinNode{
				LhsAttr:      lhs,
				RhsAttr:      rhs,
				CondOffRhs:   offset,
				TestOp:       testOp,
				ReverseOrder: ro,
			})
		}
		prevCond := prevConds[i]

		if prevCond.Alias == id {
			// JT(ID, ID)
			addNode(FieldID, FieldID, TestOpEqual, false)
		}
		if prevCond.Value.Type() == GValueTypeIdentity && prevCond.Value == id {
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
	tests []*TestAtJoinNode) *JoinNode {
	jn := &JoinNode{
		bn:      bn,
		amem:    amem,
		tests:   tests,
		testSum: calJoinTestSum(tests),
	}
	jn.ReteNode = NewReteNode(parent, jn)
	// order is matter
	amem.AddSuccessor(jn)
	return jn
}

// calJoinTestSum calculate sum all the tests
// return 0 when len(tests) == nil
func calJoinTestSum(tests []*TestAtJoinNode) uint64 {
	sum := uint64(len(tests))
	for _, t := range tests {
		sum = mix64(sum, t.Hash())
	}
	return sum
}

func performTests(tests []*TestAtJoinNode, tk *Token, wme *WME) bool {
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

func (n *JoinNode) rightActivate(w *WME) int {
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
		if tk.wme == nil || // tk is a dummy token, let it pass(see papar page 25)
			performTests(n.tests, tk, w) {
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

	if am.items == nil {
		return 0
	}

	am.ForEachItem(func(w *WME) (stop bool) {
		if tk.wme == nil || // tk is a dummy token, let it pass(see papar page 25)
			performTests(n.tests, tk, w) {
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
	n.outputMem = nil
	n.bn = nil
}

func (n *JoinNode) getOutMem() *BetaMem   { return n.outputMem }
func (n *JoinNode) setOutMem(bm *BetaMem) { n.outputMem = bm }

type (
	// PNode, aka production node, store all the tokens that match the lhs(conditions)
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
	log.DP("PNode", "found new match: %+v ", token.toWMEIDs())
	token = newToken(pn, token, wme)
	pn.items.Add(token)
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

// Matches figure out what the value of the aliases in each match
func (pn *PNode) Matches() ([]map[GVIdentity]any, error) {
	matches := make([]map[GVIdentity]any, 0, len(pn.items))
	for item := range pn.items {
		log.BugOn(item.nJoinResults.Len() == 0, "item %s has join result => %+v", item, item.nJoinResults)
		match := make(map[GVIdentity]any, len(pn.lhs))
		stk := make([]*WME, 0, item.level)
		// collect all the wme of the token
		for root := item; root != nil && root.level > 0; root = root.parent {
			// PNode might add an token with wme == nil
			if root.wme == nil {
				continue
			}
			stk = append(stk, root.wme)
		}
		j := len(stk) - 1
		for i := 0; i < len(pn.lhs) && j >= 0; i++ {
			c, wme := pn.lhs[i], stk[j]
			negativeJoinNode := c.Negative && c.Value.Type() == GValueTypeIdentity
			if negativeJoinNode {
				continue
			}
			_, in := match[c.Alias]
			if !in {
				match[c.Alias] = UnwrapTestValue(wme.Value)
			}
			if c.Value.Type() == GValueTypeIdentity && !mapContains(match, c.Value.(GVIdentity)) {
				valueID := c.Value.(GVIdentity)
				v, err := wme.GetAttrValueRaw(string(c.AliasAttr))
				if err == nil {
					match[valueID] = UnwrapTestValue(v)
				} else {
					// TODO: handle error
					return nil, errors.WithMessagef(err, "fail to GetAttrValue(%s) of WME(%v)", c.AliasAttr, wme.Value)
				}
			}
			j--
		}

		matches = append(matches, match)
	}

	return matches, nil
}

func (pn *PNode) removeToken(tk *Token) {
	log.DP("PNode", "removing token %s", tk)
	pn.items.Del(tk)
}

// Negative Node
type (
	negativeJoinResult struct {
		owner *Token // the tokne in whose local memory this result resides
		wme   *WME   // the WME that matches owner
	}

	NegativeNode struct {
		ReteNode
		// function of BetaMem and JoinNode are combined together
		items     set[*Token] // just like the BetaMem
		amem      *AlphaMem   // just like the JoinNode
		tests     []*TestAtJoinNode
		testSum   uint64
		outputMem *BetaMem // output beta mem, for speeding up buildOrShareBetaMem
		bn        *BetaNetwork
		// nearestAncestorWithSameAmem ReteNode
		// rightUnlinked               bool
	}
)

var _ BetaNode = (*NegativeNode)(nil)
var _ TokenMemory = (*NegativeNode)(nil)

func newNegativeNode(bn *BetaNetwork, parent ReteNode, amem *AlphaMem,
	tests []*TestAtJoinNode) *NegativeNode {
	nn := &NegativeNode{
		bn:      bn,
		amem:    amem,
		tests:   tests,
		testSum: calJoinTestSum(tests),
		items:   newSet[*Token](),
	}
	nn.ReteNode = NewReteNode(parent, nn)
	amem.AddSuccessor(nn)
	if bm, ok := parent.(*BetaMem); ok && bm.isDummyBetaMem() {
		// add an dummy token
		nn.items.Add(newToken(nn, nil, nil))
	}
	return nn
}

func populateNegativeJoinResult(owner *Token, wme *WME) *negativeJoinResult {
	njr := &negativeJoinResult{
		owner: owner,
		wme:   wme,
	}
	if owner != nil {
		owner.nJoinResults.Add(njr)
	}
	if wme != nil {
		wme.nJoinResults.Add(njr)
	}
	return njr
}

func (n *NegativeNode) rightActivate(w *WME) int {
	ret := 0
	log.DP("NegativeNode", "right activating WME[%s]", w.ID)
	n.items.ForEach(func(tk *Token) {
		if performTests(n.tests, tk, w) {
			log.DP("NegativeNode", "invalidate token%s for wme(%s)", tk, w.ID)
			// the negated conditions was previous matched but now it gonna be mismatched
			// delete the token to propagete this change
			if len(tk.nJoinResults) == 0 {
				tk.destoryDescendents()
			}
			populateNegativeJoinResult(tk, w)
			return
		}
	})

	return ret
}

func (n *NegativeNode) leftActivate(tk *Token, w *WME) int {
	var (
		am  = n.amem
		ret = 0
	)
	tk = newToken(n, tk, w)
	n.items.Add(tk)

	log.DP("NegativeNode", "left activate token%s, w=%+v", tk, w.ID)
	am.ForEachItem(func(item *WME) (stop bool) {
		log.DP("NegativeNode", "matching token%s for item(%s)", tk, item.ID)
		if performTests(n.tests, tk, item) {
			// don't use 'item' here
			populateNegativeJoinResult(tk, w)
			log.DP("NegativeNode", "invalidate token%s for item(%s)", tk, item.ID)
		}
		return false
	})

	if len(tk.nJoinResults) == 0 {
		// if no join result matched, which means this negative cond is matched,
		// Inform node's children
		n.ForEachChildNonStop(func(child ReteNode) {
			if bn, ok := child.(BetaNode); ok {
				ret += bn.leftActivate(tk, nil)
			}
		})
	}

	return ret
}

func (n *NegativeNode) removeToken(tk *Token) {
	if tk == nil || !n.items.Contains(tk) {
		return
	}
	n.items.Del(tk)
	tk.nJoinResults.ForEach(func(njr *negativeJoinResult) {
		njr.wme.nJoinResults.Del(njr)
		njr.owner = nil
	})
	clear(tk.nJoinResults)
}

func (n *NegativeNode) detach() {
	for item := range n.items {
		item.destory()
	}
	n.items.Clear()

	n.amem.RemoveSuccessor(n)
	// is n.amem dangling?
	if n.amem.IsSuccessorsEmpty() {
		// destory alpha mem through alpha net
		n.bn.an.DestoryAlphaMem(n.amem)
		n.amem = nil
	}
	clear(n.tests)
	n.bn = nil
}

func (n *NegativeNode) getBetaMem() *BetaMem   { return n.outputMem }
func (n *NegativeNode) setBetaMem(bm *BetaMem) { n.outputMem = bm }

func NewBetaNetwork(an *AlphaNetwork) *BetaNetwork {
	return &BetaNetwork{
		an:          an,
		topNode:     newDummyBetaMem(),
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
		var (
			negativeAM, negativeJoinNode bool
		)
		if c.Negative {
			negativeAM = c.Value.Type() != GValueTypeIdentity
			negativeJoinNode = !negativeAM
		}

		if !negativeJoinNode {
			currentNode = bn.buildOrShareBetaMem(currentNode)
			currentNode = bn.buildOrShareJoinNode(currentNode,
				bn.an.MakeAlphaMem(c, negativeAM),
				buildJoinTestFromConds(c, condsHigherUp))
		} else {
			currentNode = bn.buildOrShareNegativeNode(currentNode,
				bn.an.MakeAlphaMem(c, negativeAM),
				buildJoinTestFromConds(c, condsHigherUp))
		}
		condsHigherUp = append(condsHigherUp, c)
	}

	return currentNode
}

func isCondNeedNegativeJoin(c Cond) bool {
	return c.Negative && c.Value.Type() == GValueTypeIdentity
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

	jn := newJoinNode(bn, parent, am, tests)
	return jn
}

func (bn *BetaNetwork) buildOrShareNegativeNode(parent ReteNode, am *AlphaMem, tests []*TestAtJoinNode) *NegativeNode {
	var (
		rn      = parent
		hitNode *NegativeNode
		testSum = calJoinTestSum(tests)
	)
	rn.ForEachChild(func(child ReteNode) (stop bool) {
		nn, ok := child.(*NegativeNode)
		if !ok {
			return false
		}
		// compare alpha memory and tests
		if nn.amem == am && nn.testSum == testSum {
			hitNode = nn
			return true
		}
		return false
	})
	if hitNode != nil {
		return hitNode
	}

	return newNegativeNode(bn, parent, am, tests)
}

func (bn *BetaNetwork) buildOrShareBetaMem(parent ReteNode) *BetaMem {
	log.BugOn(parent != nil, "buildOrShareBetaMem with nil parent")
	if bm, ok := parent.(*BetaMem); ok {
		// an BetaMem mustn't be another BetaMem, unless parent is bn.topNode(a dummy BetaMem)
		return bm
	}

	type bmCacher interface {
		getOutMem() *BetaMem
		setOutMem(*BetaMem)
	}
	var (
		bm     *BetaMem
		cacher bmCacher
	)
	if c, ok := parent.(bmCacher); ok {
		cacher = c
	}
	if cacher != nil && cacher.getOutMem() != nil {
		return cacher.getOutMem()
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
	if cacher != nil {
		cacher.setOutMem(bm)
	}
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
					parent.rightActivate(w)
					return
				})
			})
		}
	case *NegativeNode:
		for tok := range parent.items {
			// re-activate tokens if they have no negative join result
			if len(tok.nJoinResults) == 0 {
				if bn, ok := newNode.(BetaNode); ok {
					bn.leftActivate(tok, nil)
				}
			}
		}
	}
}

// AddFact add a fact, and propagete addition to the entire network
func (bn *BetaNetwork) AddFact(fact Fact) {
	log.D("add fact %q", fact.ID)
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
