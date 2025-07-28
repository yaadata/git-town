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
	Build(cfgs ...configureProposalStackLineage) Option[string]
	// GetProposal fetches the proposal data for a branch, if there is one.
	GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData]
}

type proposalStackLineageBuildOptions struct {
	currentBranch          Option[gitdomain.LocalBranchName]
	location               ProposalLineageIn
	indentMarker           string
	currentBranchIndicator string
	beforeStackDisplay     []string
	afterStackDisplay      []string
}

func newProposalStackLineageBuilderOptions() *proposalStackLineageBuildOptions {
	return &proposalStackLineageBuildOptions{
		currentBranch:          None[gitdomain.LocalBranchName](),
		location:               ProposalLineageOperationInProposalBody,
		indentMarker:           "-",
		currentBranchIndicator: "point_left",
		beforeStackDisplay:     make([]string, 0),
		afterStackDisplay:      make([]string, 0),
	}
}

type configureProposalStackLineage func(opts *proposalStackLineageBuildOptions)

// WithStringBeforeStackDisplay
// A set of texts to appear before the main stack information is displayed.
// If this method is called more than once, the text appear in FIFO order.
func WithStringBeforeStackDisplay(text string) configureProposalStackLineage {
	return func(opts *proposalStackLineageBuildOptions) {
		opts.beforeStackDisplay = append(opts.beforeStackDisplay, text)
	}
}

// WithStringAfterStackDisplay
// A set of texts to appear after the main stack information is displayed.
// If this method is called more than once, the text appear in FIFO order.
func WithStringAfterStackDisplay(text string) configureProposalStackLineage {
	return func(opts *proposalStackLineageBuildOptions) {
		opts.afterStackDisplay = append(opts.afterStackDisplay, text)
	}
}

// WithIndentMarker
// Controls the marker following an indent.
func WithIndentMarker(marker string) configureProposalStackLineage {
	return func(opts *proposalStackLineageBuildOptions) {
		opts.indentMarker = marker
	}
}

// WithCurrentBranchIndicator
// Special character used to denote the current branch's proposal (if there is one).
func WithCurrentBranchIndicator(indicator string) configureProposalStackLineage {
	return func(opts *proposalStackLineageBuildOptions) {
		opts.currentBranchIndicator = indicator
	}
}

// WithProposalLineageIn
// Determines the context the proposal stack lineage is displayed.
func WithProposalLineageIn(location ProposalLineageIn) configureProposalStackLineage {
	return func(opts *proposalStackLineageBuildOptions) {
		opts.location = location
	}
}

// WithCurrentBranch
// Informs the builder which branch is the current. This is used to determine when the
// `WithCurrentBranchIndicator` character appears.
func WithCurrentBranch(branch gitdomain.LocalBranchName) configureProposalStackLineage {
	return func(opts *proposalStackLineageBuildOptions) {
		opts.currentBranch = Some(branch)
	}
}

// NewProposalStackLineageBuilder enables generating the proposal stack lineage under different contexts
// connector - forgedomain.Connector
// exemptBranches - the branches we do not care to fetch proposal data.
func NewProposalStackLineageBuilder(connector forgedomain.Connector, exemptBranches ...gitdomain.LocalBranchName) ProposalStackLineageBuilder {
	if _, hasFindProposal := connector.FindProposalFn().Get(); !hasFindProposal {
		// If there is no way to find proposals, use a no-op builder
		return &noopProposalStackLineageBuilder{}
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

func (self *proposalStackLineageBuilder) Build(cfgs ...configureProposalStackLineage) Option[string] {
	builderOptions := newProposalStackLineageBuilderOptions()
	for _, cfg := range cfgs {
		cfg(builderOptions)
	}

	var builder strings.Builder
	for _, text := range builderOptions.beforeStackDisplay {
		builder.WriteString(text)
	}

	length := len(self.orderedLineage)
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

func formattedDisplay(builderOptions *proposalStackLineageBuildOptions, currentIndentLevel string, proposalData forgedomain.ProposalData) string {
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

type noopProposalStackLineageBuilder struct{}

func (self *noopProposalStackLineageBuilder) AddBranch(childBranch gitdomain.LocalBranchName, parentBranch Option[gitdomain.LocalBranchName]) (ProposalStackLineageBuilder, error) {
	return self, nil
}

func (self *noopProposalStackLineageBuilder) Build(cfgs ...configureProposalStackLineage) Option[string] {
	return None[string]()
}

func (self *noopProposalStackLineageBuilder) GetProposal(branch gitdomain.LocalBranchName) Option[forgedomain.ProposalData] {
	return None[forgedomain.ProposalData]()
}
