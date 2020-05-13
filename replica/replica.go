package replica

import (
	"context"

	"github.com/renproject/hyperdrive/mq"
	"github.com/renproject/hyperdrive/process"
	"github.com/renproject/hyperdrive/scheduler"
	"github.com/renproject/hyperdrive/timer"
	"github.com/renproject/id"
)

// A Replica represents one Process in a replicated state machine that is bound
// to a specific Shard. It signs Messages before sending them to other Replicas,
// and verifies Messages before accepting them from other Replicas.
type Replica struct {
	opts Options

	proc         process.Process
	procsAllowed map[id.Signatory]bool

	onTimeoutPropose   <-chan timer.Timeout
	onTimeoutPrevote   <-chan timer.Timeout
	onTimeoutPrecommit <-chan timer.Timeout

	onPropose   chan process.Propose
	onPrevote   chan process.Prevote
	onPrecommit chan process.Precommit
	mq          mq.MessageQueue
}

func New(opts Options, whoami id.Signatory, signatories []id.Signatory, propose process.Proposer, validate process.Validator, commit process.Committer, catch process.Catcher, broadcast process.Broadcaster) *Replica {

	f := len(signatories) / 3
	onTimeoutPropose := make(chan timer.Timeout, 10)
	onTimeoutPrevote := make(chan timer.Timeout, 10)
	onTimeoutPrecommit := make(chan timer.Timeout, 10)
	timer := timer.NewLinearTimer(opts.TimerOpts, onTimeoutPropose, onTimeoutPrevote, onTimeoutPrecommit)
	scheduler := scheduler.NewRoundRobin(signatories)
	proc := process.New(
		whoami,
		f,
		timer,
		scheduler,
		propose,
		validate,
		broadcast,
		commit,
		catch,
	)

	procsAllowed := make(map[id.Signatory]bool)
	for _, signatory := range signatories {
		procsAllowed[signatory] = true
	}

	return &Replica{
		opts: opts,

		proc:         proc,
		procsAllowed: procsAllowed,

		onTimeoutPropose:   onTimeoutPropose,
		onTimeoutPrevote:   onTimeoutPrevote,
		onTimeoutPrecommit: onTimeoutPrecommit,

		onPropose:   make(chan process.Propose, opts.MessageQueueOpts.MaxCapacity),
		onPrevote:   make(chan process.Prevote, opts.MessageQueueOpts.MaxCapacity),
		onPrecommit: make(chan process.Precommit, opts.MessageQueueOpts.MaxCapacity),
		mq:          mq.New(opts.MessageQueueOpts),
	}
}

func (replica *Replica) Run(ctx context.Context) {
	replica.proc.Start()
	for {
		select {
		case <-ctx.Done():
			return

		case timeout := <-replica.onTimeoutPropose:
			replica.proc.OnTimeoutPropose(timeout.Height, timeout.Round)
		case timeout := <-replica.onTimeoutPrevote:
			replica.proc.OnTimeoutPrevote(timeout.Height, timeout.Round)
		case timeout := <-replica.onTimeoutPrecommit:
			replica.proc.OnTimeoutPrecommit(timeout.Height, timeout.Round)

		case propose := <-replica.onPropose:
			if !replica.filterHeight(propose.Height) {
				continue
			}
			if !replica.filterFrom(propose.From) {
				continue
			}
			replica.mq.InsertPropose(propose)
		case prevote := <-replica.onPrevote:
			if !replica.filterHeight(prevote.Height) {
				continue
			}
			if !replica.filterFrom(prevote.From) {
				continue
			}
			replica.mq.InsertPrevote(prevote)
		case precommit := <-replica.onPrecommit:
			if !replica.filterHeight(precommit.Height) {
				continue
			}
			if !replica.filterFrom(precommit.From) {
				continue
			}
			replica.mq.InsertPrecommit(precommit)
		}
		replica.flush()
	}
}

func (replica *Replica) Propose(ctx context.Context, propose process.Propose) {
	select {
	case <-ctx.Done():
	case replica.onPropose <- propose:
	}
}

func (replica *Replica) Prevote(ctx context.Context, prevote process.Prevote) {
	select {
	case <-ctx.Done():
	case replica.onPrevote <- prevote:
	}
}

func (replica *Replica) Precommit(ctx context.Context, precommit process.Precommit) {
	select {
	case <-ctx.Done():
	case replica.onPrecommit <- precommit:
	}
}

func (replica *Replica) filterHeight(height process.Height) bool {
	return height >= replica.proc.CurrentHeight
}

func (replica *Replica) filterFrom(from id.Signatory) bool {
	return replica.procsAllowed[from]
}

func (replica *Replica) flush() {
	for {
		n := replica.mq.Consume(
			replica.proc.CurrentHeight,
			replica.proc.Propose,
			replica.proc.Prevote,
			replica.proc.Precommit,
		)
		if n == 0 {
			return
		}
	}
}
