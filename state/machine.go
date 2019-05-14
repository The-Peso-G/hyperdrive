package state

import (
	"fmt"

	"github.com/renproject/hyperdrive/block"
)

type Machine interface {
	Transition(transition Transition) Action
}

type machine struct {
	state  State
	height block.Height
	round  block.Round

	polka       *block.Polka
	lockedRound *block.Round
	lockedBlock *block.SignedBlock

	polkaBuilder       block.PolkaBuilder
	commitBuilder      block.CommitBuilder
	consensusThreshold int
}

func NewMachine(polkaBuilder block.PolkaBuilder, commitBuilder block.CommitBuilder, consensusThreshold int) Machine {
	return &machine{
		polkaBuilder:       polkaBuilder,
		commitBuilder:      commitBuilder,
		consensusThreshold: consensusThreshold,
	}
}

func (machine *machine) Transition(transition Transition) Action {
	// Check pre-conditions
	if machine.lockedRound == nil {
		if machine.lockedBlock != nil {
			panic("expected locked block to be nil")
		}
	}
	if machine.lockedRound != nil {
		if machine.lockedBlock == nil {
			panic("expected locked round to be nil")
		}
	}

	switch machine.state.(type) {
	case WaitingForPropose:
		return machine.waitForPropose(transition)
	case WaitingForPolka:
		return machine.waitForPolka(transition)
	case WaitingForCommit:
		return machine.waitForCommit(transition)
	default:
		panic(fmt.Errorf("unexpected state type %T", machine.state))
	}
}

func (machine *machine) waitForPropose(transition Transition) Action {
	switch transition := transition.(type) {

	case Proposed:
		// FIXME: Proposals can (optionally) include a Polka to encourage
		// unlocking faster than would otherwise be possible.
		machine.state = WaitingForPolka{}
		return machine.preVote(&transition.SignedBlock)

	case PreVoted:
		if !machine.polkaBuilder.Insert(transition.SignedPreVote) {
			return nil
		}

		polka, preVotingRound := machine.polkaBuilder.Polka(machine.height, machine.consensusThreshold)
		if preVotingRound == nil {
			return nil
		}
		if polka != nil && (machine.polka == nil || polka.Round > machine.polka.Round) {
			machine.polka = polka
		}
		// After any +2/3 prevotes received at (H,R+x). --> goto Prevote(H,R+x)
		machine.round = *preVotingRound
		return machine.preVote(nil)

	case PreCommitted:
		if !machine.commitBuilder.Insert(transition.SignedPreCommit) {
			return nil
		}

		commit, preCommittingRound := machine.commitBuilder.Commit(machine.height, machine.consensusThreshold)
		if preCommittingRound == nil {
			return nil
		}
		if commit != nil && (machine.polka == nil || commit.Polka.Round > machine.polka.Round) {
			machine.polka = &commit.Polka
		}
		if commit != nil && commit.Polka.Block != nil {
			// After +2/3 precommits for a particular block. --> goto Commit(H)
			machine.state = WaitingForCommit{}
			machine.height = commit.Polka.Height + 1
			machine.round = 0
			return Commit{Commit: *commit}
		}
		if *preCommittingRound > machine.round {
			// After any +2/3 precommits received at (H,R+x). --> goto Precommit(H,R+x)
			machine.state = WaitingForCommit{}
			machine.round = *preCommittingRound
			return machine.preCommit()
		}
		return nil

	case TimedOut:
		machine.state = WaitingForPolka{}
		return machine.preVote(nil)

	default:
		panic(fmt.Errorf("unexpected transition type %T", transition))
	}
}

func (machine *machine) waitForPolka(transition Transition) Action {
	switch transition := transition.(type) {

	case Proposed:
		// Ignore
		return nil

	case PreVoted:
		if !machine.polkaBuilder.Insert(transition.SignedPreVote) {
			return nil
		}

		polka, preVotingRound := machine.polkaBuilder.Polka(machine.height, machine.consensusThreshold)
		if preVotingRound == nil {
			return nil
		}
		if polka != nil && (machine.polka == nil || polka.Round > machine.polka.Round) {
			machine.polka = polka
		}
		if polka != nil && polka.Round == machine.round {
			machine.state = WaitingForCommit{}
			return machine.preCommit()
		}

		// After any +2/3 prevotes received at (H,R+x). --> goto Prevote(H,R+x)
		machine.round = *preVotingRound
		return machine.preVote(nil)

	case PreCommitted:
		if !machine.commitBuilder.Insert(transition.SignedPreCommit) {
			return nil
		}

		commit, preCommittingRound := machine.commitBuilder.Commit(machine.height, machine.consensusThreshold)
		if preCommittingRound == nil {
			return nil
		}
		if commit != nil && (machine.polka == nil || commit.Polka.Round > machine.polka.Round) {
			machine.polka = &commit.Polka
		}
		if commit != nil && commit.Polka.Block != nil {
			// After +2/3 precommits for a particular block. --> goto Commit(H)
			machine.state = WaitingForCommit{}
			machine.height = commit.Polka.Height + 1
			machine.round = 0
			return Commit{Commit: *commit}
		}
		if *preCommittingRound > machine.round {
			// After any +2/3 precommits received at (H,R+x). --> goto Precommit(H,R+x)
			machine.state = WaitingForCommit{}
			machine.round = *preCommittingRound
			return machine.preCommit()
		}
		return nil

	case TimedOut:
		_, preVotingRound := machine.polkaBuilder.Polka(machine.height, machine.consensusThreshold)
		if preVotingRound == nil {
			return nil
		}

		machine.state = WaitingForCommit{}
		return machine.preCommit()

	default:
		panic(fmt.Errorf("unexpected transition type %T", transition))
	}
}

func (machine *machine) waitForCommit(transition Transition) Action {
	switch transition := transition.(type) {
	case Proposed:
		// Ignore
		return nil

	case PreVoted:
		if !machine.polkaBuilder.Insert(transition.SignedPreVote) {
			return nil
		}

		polka, preVotingRound := machine.polkaBuilder.Polka(machine.height, machine.consensusThreshold)
		if preVotingRound == nil {
			return nil
		}
		if polka != nil && (machine.polka == nil || polka.Round > machine.polka.Round) {
			machine.polka = polka
		}
		// After any +2/3 prevotes received at (H,R+x). --> goto Prevote(H,R+x)
		machine.round = *preVotingRound
		return machine.preVote(nil)

	case PreCommitted:
		if !machine.commitBuilder.Insert(transition.SignedPreCommit) {
			return nil
		}

		commit, preCommittingRound := machine.commitBuilder.Commit(machine.height, machine.consensusThreshold)
		if preCommittingRound == nil {
			return nil
		}
		if commit != nil && (machine.polka == nil || commit.Polka.Round > machine.polka.Round) {
			machine.polka = &commit.Polka
		}
		if commit != nil && commit.Polka.Block != nil {
			// After +2/3 precommits for a particular block. --> goto Commit(H)
			machine.state = WaitingForCommit{}
			machine.height = commit.Polka.Height + 1
			machine.round = 0
			return Commit{Commit: *commit}
		}
		if commit != nil && commit.Polka.Block == nil && commit.Polka.Round == machine.round {
			machine.state = WaitingForPropose{}
			machine.round++
			return Commit{
				Commit: block.Commit{
					Polka: block.Polka{
						Height: machine.height,
						Round:  machine.round,
					},
				},
			}
		}
		if *preCommittingRound > machine.round {
			// After any +2/3 precommits received at (H,R+x). --> goto Precommit(H,R+x)
			machine.state = WaitingForCommit{}
			machine.round = *preCommittingRound
			return machine.preCommit()
		}
		return nil

	case TimedOut:
		_, preCommittingRound := machine.commitBuilder.Commit(machine.height, machine.consensusThreshold)
		if preCommittingRound == nil {
			return nil
		}

		machine.state = WaitingForPropose{}
		machine.round++
		return Commit{
			Commit: block.Commit{
				Polka: block.Polka{
					Height: machine.height,
					Round:  machine.round,
				},
			},
		}

	default:
		panic(fmt.Errorf("unexpected transition type %T", transition))
	}
}

func (machine *machine) preVote(proposedBlock *block.SignedBlock) Action {
	if machine.lockedRound != nil && machine.polka != nil {
		// If the validator is locked on a block since LastLockRound but now has
		// a PoLC for something else at round PoLC-Round where LastLockRound <
		// PoLC-Round < R, then it unlocks.
		if *machine.lockedRound < machine.polka.Round {
			machine.lockedRound = nil
			machine.lockedBlock = nil
		}
	}

	if machine.lockedRound != nil {
		// If the validator is still locked on a block, it prevotes that.
		return PreVote{
			PreVote: block.PreVote{
				Block:  machine.lockedBlock,
				Height: machine.height,
				Round:  machine.round,
			},
		}
	}

	if proposedBlock != nil && proposedBlock.Height == machine.height {
		// Else, if the proposed block from Propose(H,R) is good, it prevotes that.
		return PreVote{
			PreVote: block.PreVote{
				Block:  proposedBlock,
				Height: machine.height,
				Round:  machine.round,
			},
		}
	}

	// Else, if the proposal is invalid or wasn't received on time, it prevotes <nil>.
	return PreVote{
		PreVote: block.PreVote{
			Block:  nil,
			Height: machine.height,
			Round:  machine.round,
		},
	}
}

func (machine *machine) preCommit() Action {
	if machine.polka != nil {
		if machine.polka.Block != nil {
			// If the validator has a PoLC at (H,R) for a particular block B, it
			// (re)locks (or changes lock to) and precommits B and sets LastLockRound =
			// R.
			machine.lockedRound = &machine.polka.Round
			machine.lockedBlock = machine.polka.Block
			return PreCommit{
				PreCommit: block.PreCommit{
					Polka: *machine.polka,
				},
			}
		}

		// Else, if the validator has a PoLC at (H,R) for <nil>, it unlocks and
		// precommits <nil>.
		machine.lockedRound = nil
		machine.lockedBlock = nil
		return PreCommit{
			PreCommit: block.PreCommit{
				Polka: *machine.polka,
			},
		}
	}

	// Else, it keeps the lock unchanged and precommits <nil>.
	return PreCommit{
		PreCommit: block.PreCommit{
			Polka: *machine.polka,
		},
	}
}
