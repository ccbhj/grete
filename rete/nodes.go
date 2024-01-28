package rete

import "github.com/zyedidia/generic/list"

type (
	ReteNode interface {
		Parent() ReteNode
		AddChild(children ...ReteNode)
		AnyChild() bool
		DetachParent()
		ForEachChild(fn func(ReteNode) (stop bool))
		ForEachChildNonStop(fn func(ReteNode))
	}

	reteNode struct {
		children *list.List[ReteNode]
		parent   ReteNode
	}
)

func NewReteNode(parent ReteNode, self ReteNode) ReteNode {
	node := &reteNode{
		parent: parent,
	}
	if parent != nil {
		parent.AddChild(self)
	}

	return node
}

func (n *reteNode) AddChild(children ...ReteNode) {
	for i := range children {
		child := children[i]
		n.children.PushFront(child)
	}
}

func (n *reteNode) AnyChild() bool {
	return isListEmpty(n.children)
}

func (n *reteNode) ForEachChild(fn func(ReteNode) (stop bool)) {
	listHeadForEach(n.children, fn)
}

func (n *reteNode) ForEachChildNonStop(fn func(ReteNode)) {
	listHeadForEach(n.children, func(n ReteNode) (stop bool) {
		fn(n)
		return false
	})
}

func (n *reteNode) DetachParent() {
	n.parent = nil
}

func (n *reteNode) Parent() ReteNode {
	return n.parent
}

func (r *reteNode) RemoveChild(child ReteNode) {
	if removeOneFromListTailWhen(r.children,
		func(x ReteNode) bool { return x == child }) {
		child.DetachParent()
	}
}
