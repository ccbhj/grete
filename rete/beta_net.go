package rete

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/ccbhj/grete/log"
	. "github.com/ccbhj/grete/types"
)

type (
	// Token stores WMEs that match guards or join test
	Token struct {
		parent *Token
		level  int
		wme    *WME // dummy node if wme == nil

		nodes    set[ReteNode] // nodes set that contains this token
		children set[*Token]
	}

	// tokenMemory store tokens
	tokenMemory interface {
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

func forkTokenIfWMEPresent(node ReteNode, parent *Token, wme *WME) *Token {
	if wme == nil {
		parent.nodes.Add(node)
		return parent
	}
	return forkToken(node, parent, wme)
}

func forkToken(node ReteNode, parent *Token, wme *WME) *Token {
	token := &Token{
		parent:   parent,
		wme:      wme,
		nodes:    setFrom[ReteNode](node),
		children: newSet[*Token](),
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

// destory token's children and wmes
func (t *Token) destory() {
	// clean children
	t.destoryDescendents()
	t.children.Clear()
	t.children = nil

	// remove token from TokenMemory
	t.nodes.ForEach(func(node ReteNode) {
		if tm, ok := node.(tokenMemory); ok {
			tm.removeToken(t)
			// TODO: try right unlink here
		}
		node = nil
	})
	t.nodes.Clear()

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

var _ tokenMemory = (*BetaMem)(nil)
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
	tk := forkToken(bm, nil, nil)
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
	newTk := forkTokenIfWMEPresent(bm, token, wme)
	bm.items.Add(newTk)

	ret := 0
	bm.ForEachChild(func(child ReteNode) (stop bool) {
		if bn, ok := child.(BetaNode); ok {
			ret += bn.leftActivate(newTk, wme)
		}
		return false
	})

	return ret
}

type (
	// TestAtJoinNode defines join test between WMEs
	TestAtJoinNode struct {
		AliasOffsets []int
		AliasAttr    []string
		TestOp       TestOp
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

func (t TestAtJoinNode) Hash() uint64 {
	return hashAny(t)
}

func (t TestAtJoinNode) String() string {
	s := make([]string, 0, len(t.AliasOffsets))
	for i := range t.AliasOffsets {
		s = append(s, fmt.Sprintf("$%d.%s", t.AliasOffsets[i], t.AliasAttr[i]))
	}

	return fmt.Sprintf("(%s %s)", t.TestOp, strings.Join(s, " "))
}

func (t TestAtJoinNode) performTest(token *Token) (bool, error) {
	args := make([]GValue, 0, len(t.AliasOffsets))
	wmes := token.toWMEs()
	for i, offset := range t.AliasOffsets {
		wme := wmes[offset]
		attr := t.AliasAttr[i]
		value, err := wme.GetAttrValue(attr)
		if err != nil {
			return false, errors.WithMessagef(err, "fail to GetAttrValue(attr=%s) of %v\n", attr, wme.Value)
		}
		args = append(args, value)
	}

	return t.TestOp.ToFunc()(args...)
}

// buildJoinTestFromConds convert JoinTest into positional arguments for TestOp
func buildJoinTestFromConds(c JoinTest, orders map[GVIdentity]int) (*TestAtJoinNode, error) {
	aliasOffset := make([]int, 0, 2)
	aliastAttr := make([]string, 0, 2)
	for _, s := range c.Alias {
		order, in := orders[s.Alias]
		if !in {
			return nil, errors.Errorf("unguarded alias %s", c.Alias)
		}
		aliasOffset = append(aliasOffset, order)
		aliastAttr = append(aliastAttr, string(s.AliasAttr))
	}

	return &TestAtJoinNode{
		AliasOffsets: aliasOffset,
		AliasAttr:    aliastAttr,
		TestOp:       c.TestOp,
	}, nil
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
	if amem != nil {
		amem.AddSuccessor(jn)
	}
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
	tk = forkTokenIfWMEPresent(nil, tk, wme)
	for _, test := range tests {
		ok, err := test.performTest(tk)
		if err != nil {
			// TODO: handle error
			log.L("fail to perform join test %s: %s", test, err)
			return false
		}
		if !ok {
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

	if am == nil {
		if performTests(n.tests, tk, nil) {
			n.ForEachChildNonStop(func(child ReteNode) {
				if bn, ok := child.(BetaNode); ok {
					ret += bn.leftActivate(tk, nil)
				}
			})
		}
		return ret
	}

	if am.items.Len() == 0 {
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
	if n.amem != nil {
		n.amem.RemoveSuccessor(n)
		// is n.amem dangling?
		if n.amem.IsSuccessorsEmpty() {
			// destory alpha mem through alpha net
			n.bn.an.DestoryAlphaMem(n.amem)
			n.amem = nil
		}
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
		items     set[*Token]
		AliasInfo []AliasDeclaration
	}
)

var _ tokenMemory = (*PNode)(nil)
var _ BetaNode = (*PNode)(nil)

func NewPNode(parent ReteNode, aliasDecls []AliasDeclaration) *PNode {
	pn := &PNode{
		items:     newSet[*Token](),
		AliasInfo: aliasDecls,
	}

	pn.ReteNode = NewReteNode(parent, pn)
	return pn
}

func (pn *PNode) leftActivate(token *Token, wme *WME) int {
	log.DP("PNode", "found new match: %+v ", token.toWMEIDs())
	token = forkTokenIfWMEPresent(pn, token, wme)
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
		match := make(map[GVIdentity]any, len(pn.AliasInfo))
		wmes := item.toWMEs()
		for i, decl := range pn.AliasInfo {
			match[decl.Alias] = UnwrapTestValue(wmes[i].Value)
		}

		matches = append(matches, match)
	}

	return matches, nil
}

func (pn *PNode) removeToken(tk *Token) {
	log.DP("PNode", "removing token %s", tk)
	pn.items.Del(tk)
}

func NewBetaNetwork(an *AlphaNetwork) *BetaNetwork {
	return &BetaNetwork{
		an:          an,
		topNode:     newDummyBetaMem(),
		productions: make(map[string]*PNode),
	}
}

func (bn *BetaNetwork) buildOrShareNetwork(parent ReteNode, aliasDecl []AliasDeclaration, joinTests []JoinTest) ReteNode {
	var (
		currentNode ReteNode
		aliasOrders = make(map[GVIdentity]int, len(aliasDecl))
	)

	currentNode = parent
	for i, decl := range aliasDecl {
		currentNode = bn.buildOrShareBetaMem(currentNode)
		am := bn.an.MakeAlphaMem(decl.Type, decl.Guards)
		bn.an.InitAlphaMem(am)
		currentNode = bn.buildOrShareJoinNode(currentNode, am, nil)
		aliasOrders[decl.Alias] = i
	}
	for _, jt := range joinTests {
		jn, err := buildJoinTestFromConds(jt, aliasOrders)
		if err != nil {
			// TODO: handle error here
			panic(err)
		}
		currentNode = bn.buildOrShareBetaMem(currentNode)
		currentNode = bn.buildOrShareJoinNode(currentNode, nil, []*TestAtJoinNode{jn})
	}

	return currentNode
}

func isCondNeedNegativeJoin(c Guard) bool {
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

type AliasDeclaration struct {
	Alias  GVIdentity
	Type   TypeInfo
	Guards []Guard
}

type Production struct {
	ID    string
	When  []AliasDeclaration
	Match []JoinTest
}

// AddProduction add an production and register its unique id
func (bn *BetaNetwork) AddProduction(p Production) *PNode {
	aliasDecls := p.When
	if len(aliasDecls) == 0 {
		panic("need some guards")
	}

	id := p.ID
	if pn, in := bn.productions[id]; in {
		return pn
	}

	jt := p.Match
	currentNode := bn.buildOrShareNetwork(bn.topNode, aliasDecls, jt)
	pn := NewPNode(currentNode, aliasDecls)
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
