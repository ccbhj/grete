package rete

type (
	ReteNode interface {
		Parent() ReteNode
		AddChild(children ...ReteNode)
		AnyChild() bool
		ClearAndRestoreChildren(fn func())
		AttachParent(parent ReteNode)
		RemoveChild(child ReteNode) bool
		DetachParent()
		ForEachChild(fn func(child ReteNode) (stop bool))
		ForEachChildNonStop(fn func(child ReteNode))
	}

	reteNode struct {
		children set[ReteNode]
		parent   ReteNode
	}
)

func NewReteNode(parent ReteNode, self ReteNode) ReteNode {
	node := &reteNode{
		parent:   parent,
		children: newSet[ReteNode](),
	}
	if parent != nil {
		parent.AddChild(self)
	}

	return node
}

func (n *reteNode) AddChild(children ...ReteNode) {
	for i := range children {
		child := children[i]
		n.children.Add(child)
	}
}

func (n *reteNode) AnyChild() bool {
	return n.children.Len() > 0
}

func (n *reteNode) ForEachChild(fn func(ReteNode) (stop bool)) {
	for child := range n.children {
		if fn(child) {
			return
		}
	}
}

func (n *reteNode) ForEachChildNonStop(fn func(ReteNode)) {
	for child := range n.children {
		fn(child)
	}
}

func (n *reteNode) AttachParent(parent ReteNode) {
	n.parent = parent
}

func (n *reteNode) DetachParent() {
	n.parent = nil
}

func (n *reteNode) Parent() ReteNode {
	return n.parent
}

func (r *reteNode) RemoveChild(child ReteNode) bool {
	if r.children.Contains(child) {
		r.children.Del(child)
		child.DetachParent()
		return true
	}
	return false
}

func (r *reteNode) ClearAndRestoreChildren(fn func()) {
	savedListOfChild := r.children
	l := newSet[ReteNode]()
	r.children = l
	fn()
	r.children = savedListOfChild
}
