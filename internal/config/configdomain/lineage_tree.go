package configdomain

import (
	"github.com/git-town/git-town/v21/internal/git/gitdomain"
)

type LineageTreeNode struct {
	branch     gitdomain.LocalBranchName
	childNodes []*LineageTreeNode
	depth      int
}

func newLineageTreeNode(branch gitdomain.LocalBranchName) *LineageTreeNode {
	return &LineageTreeNode{
		branch:     branch,
		childNodes: make([]*LineageTreeNode, 0),
		depth:      -1,
	}
}

func (self *LineageTreeNode) BranchName() gitdomain.LocalBranchName {
	return self.branch
}

func (self *LineageTreeNode) ChildBranches() gitdomain.LocalBranchNames {
	var branches gitdomain.LocalBranchNames
	for _, node := range self.childNodes {
		branches = append(branches, node.branch)
	}

	return branches
}

func (self *LineageTreeNode) TreeDepth() int {
	return self.depth
}

func (self *LineageTreeNode) ChildNodes() []*LineageTreeNode {
	return self.childNodes
}

type LineageTree struct {
	Node *LineageTreeNode
}

func NewLineageTree(currentBranch gitdomain.LocalBranchName, lineage Lineage) LineageTree {
	tree := LineageTree{
		Node: newLineageTreeNode(""),
	}

	mapper := make(map[gitdomain.LocalBranchName]*LineageTreeNode)

	ancestors := lineage.Ancestors(currentBranch)
	previous := tree.Node
	var currentNode *LineageTreeNode
	for _, ancestor := range ancestors {
		currentNode = newLineageTreeNode(ancestor)
		previous.childNodes = append(previous.childNodes, currentNode)
		currentNode.depth = previous.depth + 1
		mapper[ancestor] = currentNode
		previous = currentNode
	}

	currentNode = newLineageTreeNode(currentBranch)
	currentNode.depth = previous.depth + 1
	previous.childNodes = append(previous.childNodes, currentNode)
	mapper[currentBranch] = currentNode

	descendants := lineage.Descendants(currentBranch)
	for _, descendant := range descendants {
		parent := lineage.Parent(descendant)
		parentNode := mapper[parent.GetOrPanic()]
		descendantNode := newLineageTreeNode(descendant)
		descendantNode.depth = parentNode.depth + 1
		parentNode.childNodes = append(parentNode.childNodes, descendantNode)
		mapper[descendant] = descendantNode
	}

	tree.Node = tree.Node.childNodes[0]
	return tree
}
