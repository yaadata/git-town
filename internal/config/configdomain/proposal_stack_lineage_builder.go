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
	// Adds the next branch in the lineage chain
	AddBranch(childBranch gitdomain.LocalBranchName, parentBranch Option[gitdomain.LocalBranchName]) (ProposalStackLineageBuilder, error)
	// Build - creates the proposal lineage based on the display location
	Build(cfgs ...configureProposalLineageBuildOptions) Option[string]
	// GetProposalByBranch
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

func NewProposalStackLineageBuilder(connector forgedomain.Connector, exemptBranches ...gitdomain.LocalBranchName) ProposalStackLineageBuilder {
	if _, hasFindProposal := connector.FindProposalFn().Get(); !hasFindProposal {
		return &noopProposalLineageBuilder{}
	}

	return &proposalStackLineageBuilder{
		orderedLineage:                           make([]*proposalLineage, 0),
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
	orderedLineage                           []*proposalLineage
	branchesExemptFromDisplayingProposalInfo gitdomain.LocalBranchNames
}

func (self *proposalStackLineageBuilder) AddBranch(childBranch gitdomain.LocalBranchName, parentBranch Option[gitdomain.LocalBranchName]) (ProposalStackLineageBuilder, error) {
	if self.branchesExemptFromDisplayingProposalInfo.Contains(childBranch) || parentBranch.IsNone() {
		self.orderedLineage = append(self.orderedLineage, &proposalLineage{
			branch:   childBranch,
			proposal: None[forgedomain.ProposalData](),
		})
		return self, nil
	}

	parent := parentBranch.GetOrPanic().BranchName().LocalName()
	findProposalFn, _ := self.connector.FindProposalFn().Get()

	proposal, err := findProposalFn(childBranch, parent)
	if err != nil {
		return self, fmt.Errorf("failed to find proposal for branch %s: %w", childBranch, err)
	}

	proposalData, hasProposal := proposal.Get()
	if !hasProposal {
		return self, fmt.Errorf("no proposal found branch %q", childBranch)
	}

	self.orderedLineage = append(self.orderedLineage, &proposalLineage{
		branch:   childBranch,
		proposal: Some(proposalData.Data.Data()),
	})
	return self, nil
}

func (self *proposalStackLineageBuilder) Build(cfgs ...configureProposalLineageBuildOptions) Option[string] {
	builderOptions := newProposalLineageBuilderOptions()
	for _, cfg := range cfgs {
		cfg(builderOptions)
	}

	var builder strings.Builder
	for _, text := range builderOptions.beforeStackDisplay {
		builder.WriteString(text)
	}

	length := len(self.orderedLineage)
	var numberOfCapturedProposals uint
	for i := len(self.orderedLineage); i > 0; i-- {
		node := self.orderedLineage[length-i]
		indent := strings.Repeat(" ", (length-i)*2)
		if self.branchesExemptFromDisplayingProposalInfo.Contains(node.branch) {
			builder.WriteString(fmt.Sprintf("%s %s %s\n", indent, builderOptions.indentMarker, node.branch.BranchName()))
			continue
		}

		proposalData, hasProposalData := node.proposal.Get()
		if !hasProposalData {
			break
		}

		builder.WriteString(formattedDisplay(builderOptions, indent, proposalData))
		numberOfCapturedProposals++
	}

	for _, text := range builderOptions.afterStackDisplay {
		builder.WriteString(text)
	}

	return Some(builder.String())
}

func (self *proposalStackLineageBuilder) GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData] {
	response := None[forgedomain.ProposalData]()
	for _, curr := range self.orderedLineage {
		if curr.branch == branch {
			response = curr.proposal
		}
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

func (self *noopProposalLineageBuilder) AddBranch(childBranch gitdomain.LocalBranchName, parentBranch Option[gitdomain.LocalBranchName]) (ProposalStackLineageBuilder, error) {
	return self, nil
}

func (self *noopProposalLineageBuilder) Build(cfgs ...configureProposalLineageBuildOptions) Option[string] {
	return None[string]()
}

func (self *noopProposalLineageBuilder) GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData] {
	return None[forgedomain.ProposalData]()
}
