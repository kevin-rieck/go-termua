package tui

import (
	"termua/internal/opcua"
)

// ViewItem represents a single visible line in the TUI Address Space.
type ViewItem struct {
	Node           opcua.AddressNode
	Depth          int
	IsExpanded     bool
	ChildrenLoaded bool
	IsLoading      bool
	Err            error
}

type treeNode struct {
	node           opcua.AddressNode
	depth          int
	expanded       bool
	childrenLoaded bool
	loading        bool
	err            error
}

// AddressSpace manages the state of the OPC UA Address Space tree.
// It acts as a seam, hiding flat-array stitching and depth tracking from the UI.
type AddressSpace struct {
	tree []treeNode
}

func NewAddressSpace() *AddressSpace {
	return &AddressSpace{
		tree: []treeNode{{
			node: opcua.AddressNode{NodeID: "i=85", DisplayName: "Objects", BrowseName: "Objects", NodeClass: "Object"},
		}},
	}
}

// View returns a flattened, ordered list of currently visible nodes.
func (a *AddressSpace) View() []ViewItem {
	var visible []ViewItem
	hiddenBelowDepth := -1
	for _, n := range a.tree {
		if hiddenBelowDepth >= 0 {
			if n.depth > hiddenBelowDepth {
				continue
			}
			hiddenBelowDepth = -1
		}
		visible = append(visible, ViewItem{
			Node:           n.node,
			Depth:          n.depth,
			IsExpanded:     n.expanded,
			ChildrenLoaded: n.childrenLoaded,
			IsLoading:      n.loading,
			Err:            n.err,
		})
		if !n.expanded {
			hiddenBelowDepth = n.depth
		}
	}
	return visible
}

// Toggle toggles the expansion state of a node.
// Returns true if the TUI needs to dispatch an async fetch (because children aren't loaded).
func (a *AddressSpace) Toggle(id string) bool {
	idx := a.indexByNodeID(id)
	if idx < 0 {
		return false
	}
	if a.tree[idx].loading {
		return false
	}
	if a.tree[idx].childrenLoaded {
		a.tree[idx].expanded = !a.tree[idx].expanded
		return false
	}
	
	a.tree[idx].loading = true
	a.tree[idx].err = nil
	return true
}

// ApplyChildren populates the tree after an asynchronous network fetch.
func (a *AddressSpace) ApplyChildren(id string, children []opcua.AddressNode, err error) {
	idx := a.indexByNodeID(id)
	if idx < 0 {
		return
	}
	a.tree[idx].loading = false
	if err != nil {
		a.tree[idx].err = err
		return
	}
	
	a.tree[idx].childrenLoaded = true
	a.tree[idx].expanded = true
	
	var newChildren []treeNode
	for _, child := range children {
		if child.NodeID == "" {
			continue
		}
		newChildren = append(newChildren, treeNode{
			node:  child,
			depth: a.tree[idx].depth + 1,
		})
	}
	
	a.tree = insertTreeChildren(a.tree, idx, newChildren)
}

// Node returns the underlying AddressNode for details panel.
func (a *AddressSpace) Node(id string) (opcua.AddressNode, bool) {
	idx := a.indexByNodeID(id)
	if idx >= 0 {
		return a.tree[idx].node, true
	}
	return opcua.AddressNode{}, false
}

// Collapse collapses the specified node if it is expanded.
func (a *AddressSpace) Collapse(id string) {
	idx := a.indexByNodeID(id)
	if idx >= 0 && a.tree[idx].childrenLoaded {
		a.tree[idx].expanded = false
	}
}

// MarkLoading marks a node as loading explicitly.
func (a *AddressSpace) MarkLoading(id string) {
	idx := a.indexByNodeID(id)
	if idx >= 0 {
		a.tree[idx].loading = true
		a.tree[idx].err = nil
	}
}

func (a *AddressSpace) indexByNodeID(nodeID string) int {
	for i, node := range a.tree {
		if node.node.NodeID == nodeID {
			return i
		}
	}
	return -1
}

func insertTreeChildren(tree []treeNode, parentIndex int, children []treeNode) []treeNode {
	parentDepth := tree[parentIndex].depth
	end := parentIndex + 1
	for end < len(tree) && tree[end].depth > parentDepth {
		end++
	}
	result := make([]treeNode, 0, len(tree)-(end-parentIndex-1)+len(children))
	result = append(result, tree[:parentIndex+1]...)
	result = append(result, children...)
	result = append(result, tree[end:]...)
	return result
}
