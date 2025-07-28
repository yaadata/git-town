package configdomain

import (
	"github.com/git-town/git-town/v21/internal/git/gitdomain"
)

type ProposalLineageTreeNode struct {
	branch     gitdomain.LocalBranchName
	childNodes []*ProposalLineageTreeNode
	depth      int
}

func newLineageTreeNode(branch gitdomain.LocalBranchName) *ProposalLineageTreeNode {
	return &ProposalLineageTreeNode{
		branch:     branch,
		childNodes: make([]*ProposalLineageTreeNode, 0),
		depth:      -1,
	}
}

func (self *ProposalLineageTreeNode) BranchName() gitdomain.LocalBranchName {
	return self.branch
}

func (self *ProposalLineageTreeNode) ChildBranches() gitdomain.LocalBranchNames {
	var branches gitdomain.LocalBranchNames
	for _, node := range self.childNodes {
		branches = append(branches, node.branch)
	}

	return branches
}

func (self *ProposalLineageTreeNode) TreeDepth() int {
	return self.depth
}

func (self *ProposalLineageTreeNode) ChildNodes() []*ProposalLineageTreeNode {
	return self.childNodes
}

type ProposalLineageTree struct {
	Node     *ProposalLineageTreeNode
	branches gitdomain.LocalBranchNames
}

func (self *ProposalLineageTree) Branches() gitdomain.LocalBranchNames {
	return self.branches
}

func NewLineageTree(currentBranch gitdomain.LocalBranchName, lineage Lineage, mainAndPerennials gitdomain.LocalBranchNames) ProposalLineageTree {
	tree := ProposalLineageTree{
		Node: newLineageTreeNode(""),
	}

	visited := make(map[gitdomain.LocalBranchName]*ProposalLineageTreeNode)
	ancestors := lineage.Ancestors(currentBranch)
	descendantsOfAncestor := gitdomain.NewLocalBranchNames(currentBranch.String())
	previous := tree.Node
	var currentNode *ProposalLineageTreeNode
	for _, ancestor := range ancestors {
		currentNode = newLineageTreeNode(ancestor)
		previous.childNodes = append(previous.childNodes, currentNode)
		currentNode.depth = previous.depth + 1
		visited[ancestor] = currentNode

		children := lineage.Children(ancestor)
		for _, child := range children {
			if !mainAndPerennials.Contains(child) && !ancestors.Contains(child) {
				descendantsOfAncestor = append(descendantsOfAncestor, child)
			}
		}

		previous = currentNode
	}

	for _, descendant := range descendantsOfAncestor {
		addDescendantNodes(descendant, lineage, visited)
	}

	tree.Node = tree.Node.childNodes[0]
	branches := make([]gitdomain.LocalBranchName, 0, len(visited))
	for branch := range visited {
		branches = append(branches, branch)
	}
	tree.branches = branches
	return tree
}

func addDescendantNodes(branch gitdomain.LocalBranchName, lineage Lineage, visited map[gitdomain.LocalBranchName]*ProposalLineageTreeNode) {
	if _, ok := visited[branch]; ok {
		return
	}

	parent := lineage.Parent(branch)
	parentNode := visited[parent.GetOrPanic()]
	branchNode := newLineageTreeNode(branch)
	branchNode.depth = parentNode.depth + 1
	parentNode.childNodes = append(parentNode.childNodes, branchNode)
	visited[branch] = branchNode

	children := lineage.Children(branch)
	for _, child := range children {
		addDescendantNodes(child, lineage, visited)
	}
}
