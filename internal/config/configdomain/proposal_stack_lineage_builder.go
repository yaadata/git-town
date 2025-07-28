package configdomain

import (
	"fmt"
	"strings"

	"github.com/git-town/git-town/v21/internal/cli/colors"
	"github.com/git-town/git-town/v21/internal/forge/forgedomain"
	"github.com/git-town/git-town/v21/internal/git/gitdomain"
	. "github.com/git-town/git-town/v21/pkg/prelude"
)

type ProposalStackLineageBuilder interface {
	// Build - creates the proposal lineage based on the display location
	Build(tree LineageTree, cfgs ...configureProposalLineageBuildOptions) Option[string]
	// Adds a branch and tracks the proposal if there is one
	AddBranch(branch gitdomain.LocalBranchName) (ProposalStackLineageBuilder, error)
	// GetProposal fetches the proposal data given a branch name.
	GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData]
}

type proposalLineageBuildOptions struct {
	currentBranch          Option[gitdomain.LocalBranchName]
	location               ProposalLineageIn
	indentMarker           string
	currentBranchIndicator string
	beforeStackDisplay     []string
	afterStackDisplay      []string
}

func newProposalLineageBuilderOptions() *proposalLineageBuildOptions {
	return &proposalLineageBuildOptions{
		currentBranch:          None[gitdomain.LocalBranchName](),
		location:               ProposalLineageOperationInProposalBody,
		indentMarker:           "-",
		currentBranchIndicator: "point_left",
		beforeStackDisplay:     make([]string, 0),
		afterStackDisplay:      make([]string, 0),
	}
}

type configureProposalLineageBuildOptions func(opts *proposalLineageBuildOptions)

func WithStringBeforeStackDisplay(text string) configureProposalLineageBuildOptions {
	return func(opts *proposalLineageBuildOptions) {
		opts.beforeStackDisplay = append(opts.beforeStackDisplay, text)
	}
}

func WithStringAfterStackDisplay(text string) configureProposalLineageBuildOptions {
	return func(opts *proposalLineageBuildOptions) {
		opts.afterStackDisplay = append(opts.afterStackDisplay, text)
	}
}

func WithIndentMarker(marker string) configureProposalLineageBuildOptions {
	return func(opts *proposalLineageBuildOptions) {
		opts.indentMarker = marker
	}
}

func WithCurrentBranchIndicator(indicator string) configureProposalLineageBuildOptions {
	return func(opts *proposalLineageBuildOptions) {
		opts.currentBranchIndicator = indicator
	}
}

func WithProposalLineageIn(location ProposalLineageIn) configureProposalLineageBuildOptions {
	return func(opts *proposalLineageBuildOptions) {
		opts.location = location
	}
}

func WithCurrentBranch(branch gitdomain.LocalBranchName) configureProposalLineageBuildOptions {
	return func(opts *proposalLineageBuildOptions) {
		opts.currentBranch = Some(branch)
	}
}

func NewProposalStackLineageBuilder(connector forgedomain.Connector, lineage Lineage, exemptBranches ...gitdomain.LocalBranchName) ProposalStackLineageBuilder {
	if _, hasFindProposal := connector.FindProposalFn().Get(); !hasFindProposal {
		return &noopProposalLineageBuilder{}
	}

	return &proposalStackLineageBuilder{
		lineage:                                  lineage,
		branchToProposal:                         make(map[gitdomain.LocalBranchName]Option[forgedomain.ProposalData]),
		connector:                                connector,
		branchesExemptFromDisplayingProposalInfo: exemptBranches,
	}
}

type proposalLineage struct {
	branch   gitdomain.LocalBranchName
	proposal Option[forgedomain.ProposalData]
}

type proposalStackLineageBuilder struct {
	connector                                forgedomain.Connector
	lineage                                  Lineage
	branchToProposal                         map[gitdomain.LocalBranchName]Option[forgedomain.ProposalData]
	branchesExemptFromDisplayingProposalInfo gitdomain.LocalBranchNames
}

func (self *proposalStackLineageBuilder) AddBranch(branch gitdomain.LocalBranchName) (ProposalStackLineageBuilder, error) {
	parentBranch := self.lineage.Parent(branch)
	if self.branchesExemptFromDisplayingProposalInfo.Contains(branch) || parentBranch.IsNone() {
		self.branchToProposal[branch] = None[forgedomain.ProposalData]()
		return self, nil
	}

	parent := parentBranch.GetOrPanic().BranchName().LocalName()
	findProposalFn, _ := self.connector.FindProposalFn().Get()

	proposal, err := findProposalFn(branch, parent)
	if err != nil {
		return self, fmt.Errorf("failed to find proposal for branch %s: %w", branch, err)
	}

	proposalData, hasProposal := proposal.Get()
	if !hasProposal {
		return self, fmt.Errorf("no proposal found branch %q", branch)
	}

	self.branchToProposal[branch] = Some(proposalData.Data.Data())
	return self, nil
}

func (self *proposalStackLineageBuilder) Build(tree LineageTree, cfgs ...configureProposalLineageBuildOptions) Option[string] {
	builderOptions := newProposalLineageBuilderOptions()
	for _, cfg := range cfgs {
		cfg(builderOptions)
	}

	var builder strings.Builder
	for _, text := range builderOptions.beforeStackDisplay {
		builder.WriteString(text)
	}

	builder.WriteString(self.build(tree.Node, builderOptions))

	for _, text := range builderOptions.afterStackDisplay {
		builder.WriteString(text)
	}

	return Some(builder.String())
}

func (self *proposalStackLineageBuilder) build(node *LineageTreeNode, builderOptions *proposalLineageBuildOptions) string {
	var builder strings.Builder
	indent := strings.Repeat(" ", node.depth*2)
	if self.branchesExemptFromDisplayingProposalInfo.Contains(node.branch) {
		builder.WriteString(fmt.Sprintf("%s %s %s\n", indent, builderOptions.indentMarker, node.branch.BranchName()))
		for _, child := range node.childNodes {
			builder.WriteString(self.build(child, builderOptions))
		}
		return builder.String()
	}

	proposalData, ok := self.branchToProposal[node.branch]
	if !ok || proposalData.IsNone() {
		return builder.String()
	}

	builder.WriteString(formattedDisplay(builderOptions, indent, proposalData.GetOrPanic()))
	for _, child := range node.childNodes {
		builder.WriteString(self.build(child, builderOptions))
	}

	return builder.String()
}

func (self *proposalStackLineageBuilder) GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData] {
	response, ok := self.branchToProposal[branch]
	if !ok {
		return None[forgedomain.ProposalData]()
	}
	return response
}

func formattedDisplay(builderOptions *proposalLineageBuildOptions, currentIndentLevel string, proposalData forgedomain.ProposalData) string {
	if builderOptions.location == ProposalLineageInTerminal {
		if builderOptions.currentBranch.GetOrDefault() == proposalData.Source {
			return colors.Green().Styled(fmt.Sprintf("%s%s %s PR #%d %s (%s)\n", builderOptions.currentBranchIndicator, currentIndentLevel, builderOptions.indentMarker, proposalData.Number, proposalData.Title, proposalData.URL))
		}
		return fmt.Sprintf("%s %s PR #%d %s (%s)\n", currentIndentLevel, builderOptions.indentMarker, proposalData.Number, proposalData.Title, proposalData.URL)
	} else {
		if builderOptions.currentBranch.GetOrDefault() == proposalData.Source {
			return fmt.Sprintf("%s %s PR %s %s\n", currentIndentLevel, builderOptions.indentMarker, proposalData.URL, builderOptions.currentBranchIndicator)
		}
		return fmt.Sprintf("%s %s PR %s\n", currentIndentLevel, builderOptions.indentMarker, proposalData.URL)
	}
}

type noopProposalLineageBuilder struct{}

func (self *noopProposalLineageBuilder) AddBranch(branch gitdomain.LocalBranchName) (ProposalStackLineageBuilder, error) {
	return self, nil
}

func (self *noopProposalLineageBuilder) Build(tree LineageTree, cfgs ...configureProposalLineageBuildOptions) Option[string] {
	return None[string]()
}

func (self *noopProposalLineageBuilder) GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData] {
	return None[forgedomain.ProposalData]()
}
