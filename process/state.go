package process

import (
	"github.com/renproject/hyperdrive/block"
)

// The State of a Process. It is isolated from the Process so that it can be
// easily marshaled to/from JSON.
type State struct {
	CurrentHeight block.Height `json:"currentHeight"`
	CurrentRound  block.Round  `json:"currentRound"`
	CurrentStep   Step         `json:"currentStep"`

	LockedBlock block.Block `json:"lockedBlock"` // the most recent block for which a precommit message has been sent
	LockedRound block.Round `json:"lockedRound"` // the last round in which the process sent a precommit message that is not nil.
	ValidBlock  block.Block `json:"validBlock"`  // store the most recent possible decision value
	ValidRound  block.Round `json:"validRound"`  // is the last round in which valid value is updated

	Proposals  *Inbox `json:"proposals"`
	Prevotes   *Inbox `json:"prevotes"`
	Precommits *Inbox `json:"precommits"`
}

// DefaultState returns a State with all values set to the default. See
// https://arxiv.org/pdf/1807.04938.pdf for more information.
func DefaultState(f int) State {
	return State{
		CurrentHeight: 1, // Skip the genesis block
		CurrentRound:  0,
		CurrentStep:   StepPropose,
		LockedBlock:   block.InvalidBlock,
		LockedRound:   block.InvalidRound,
		ValidBlock:    block.InvalidBlock,
		ValidRound:    block.InvalidRound,
		Proposals:     NewInbox(f, ProposeMessageType),
		Prevotes:      NewInbox(f, PrevoteMessageType),
		Precommits:    NewInbox(f, PrecommitMessageType),
	}
}

// Reset the State (not all values are reset). See
// https://arxiv.org/pdf/1807.04938.pdf for more information.
func (state *State) Reset(height block.Height) {
	state.LockedBlock = block.InvalidBlock
	state.LockedRound = block.InvalidRound
	state.ValidBlock = block.InvalidBlock
	state.ValidRound = block.InvalidRound
	state.Proposals.Reset(height)
	state.Prevotes.Reset(height)
	state.Precommits.Reset(height)
}

// Equal compares one State with another.
func (state *State) Equal(other State) bool {
	return state.CurrentHeight == other.CurrentHeight &&
		state.CurrentRound == other.CurrentRound &&
		state.CurrentStep == other.CurrentStep &&
		state.LockedBlock.Equal(other.LockedBlock) &&
		state.LockedRound == other.LockedRound &&
		state.ValidBlock.Equal(other.ValidBlock) &&
		state.ValidRound == other.ValidRound
}

