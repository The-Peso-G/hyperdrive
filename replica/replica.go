package replica

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"github.com/renproject/abi"
	"github.com/renproject/hyperdrive/block"
	"github.com/renproject/hyperdrive/process"
	"github.com/renproject/hyperdrive/schedule"
	"github.com/renproject/id"
	"github.com/sirupsen/logrus"
)

type Shards []Shard

// Shard uniquely identifies the Shard being maintained by the Replica.
type Shard [32]byte

// Equal compares one Shard with another.
func (shard Shard) Equal(other Shard) bool {
	return bytes.Equal(shard[:], other[:])
}

// String implements the `fmt.Stringer` interface.
func (shard Shard) String() string {
	return base64.RawStdEncoding.EncodeToString(shard[:])
}

func (shard Shard) SizeHint() int {
	return 32
}

func (shard Shard) Marshal(w io.Writer, m int) (int, error) {
	return abi.Bytes32(shard).Marshal(w, m)
}

func (shard *Shard) Unmarshal(r io.Reader, m int) (int, error) {
	return (*abi.Bytes32)(shard).Unmarshal(r, m)
}

type Messages []Message

// A Message sent/received by a Replica is composed of a Shard and the
// underlying `process.Message` data. It is expected that a Replica will sign
// the underlying `process.Message` data before sending the Message.
type Message struct {
	Message process.Message
	Shard   Shard
}

// ProcessStorage saves and restores `process.State` to persistent memory. This
// guarantees that in the event of an unexpected shutdown, the Replica will only
// drop the `process.Message` that was currently being handling.
type ProcessStorage interface {
	SaveState(state *process.State, shard Shard)
	RestoreState(state *process.State, shard Shard)
}

// Options define a set of properties that can be used to parameterise the
// Replica and its behaviour.
type Options struct {
	// Logging
	Logger logrus.FieldLogger

	// Timeout options for proposing, prevoting, and precommiting
	BackOffExp  float64
	BackOffBase time.Duration
	BackOffMax  time.Duration

	// MaxMessageQueueSize is the maximum number of "future" messages that the
	// Replica will buffer in memory.
	MaxMessageQueueSize int
}

func (options *Options) setZerosToDefaults() {
	if options.Logger == nil {
		options.Logger = logrus.StandardLogger()
	}
	if options.BackOffExp == 0 {
		options.BackOffExp = 1.6
	}
	if options.BackOffBase == time.Duration(0) {
		options.BackOffBase = 20 * time.Second
	}
	if options.BackOffMax == time.Duration(0) {
		options.BackOffMax = 5 * time.Minute
	}
	if options.MaxMessageQueueSize == 0 {
		options.MaxMessageQueueSize = 512
	}
}

type Replicas []Replica

// A Replica represents one Process in a replicated state machine that is bound
// to a specific Shard. It signs Messages before sending them to other Replicas,
// and verifies Messages before accepting them from other Replicas.
type Replica struct {
	options      Options
	shard        Shard
	p            *process.Process
	blockStorage BlockStorage

	scheduler schedule.Scheduler
	rebaser   *shardRebaser
	cache     baseBlockCache

	messageQueue MessageQueue
}

func New(options Options, pStorage ProcessStorage, blockStorage BlockStorage, blockIterator BlockIterator, validator Validator, observer Observer, broadcaster Broadcaster, scheduler schedule.Scheduler, catcher process.Catcher, shard Shard, privKey ecdsa.PrivateKey) Replica {
	options.setZerosToDefaults()
	latestBase := blockStorage.LatestBaseBlock(shard)
	if len(latestBase.Header().Signatories())%3 != 1 || len(latestBase.Header().Signatories()) < 4 {
		panic(fmt.Errorf("invariant violation: number of nodes needs to be 3f +1, got %v", len(latestBase.Header().Signatories())))
	}
	shardRebaser := newShardRebaser(blockStorage, blockIterator, validator, observer, shard)

	// Create a Process in the default state and then restore it
	p := process.New(
		options.Logger,
		id.NewSignatory(privKey.PublicKey),
		blockStorage.Blockchain(shard),
		process.DefaultState((len(latestBase.Header().Signatories())-1)/3),
		newSaveRestorer(pStorage, shard),
		shardRebaser,
		shardRebaser,
		shardRebaser,
		newSigner(broadcaster, shard, privKey),
		scheduler,
		newBackOffTimer(options.BackOffExp, options.BackOffBase, options.BackOffMax),
		catcher,
	)
	p.Restore()

	return Replica{
		options:      options,
		shard:        shard,
		p:            p,
		blockStorage: blockStorage,

		scheduler: scheduler,
		rebaser:   shardRebaser,
		cache:     newBaseBlockCache(latestBase),

		messageQueue: NewMessageQueue(options.MaxMessageQueueSize),
	}
}

func (replica *Replica) Start() {
	replica.p.Start()
}

func (replica *Replica) HandleMessage(m Message) {
	// Check that Message is from our shard. If it is not, then there is no
	// point processing the message.
	if !replica.shard.Equal(m.Shard) {
		replica.options.Logger.Warnf("bad message: expected shard=%v, got shard=%v", replica.shard, m.Shard)
		return
	}

	// Ignore messagse from heights that the process has already progressed
	// through. Messages at these earlier heights have no affect on consensus,
	// and so there is no point wasting time processing them.
	if m.Message.Height() < replica.p.CurrentHeight() {
		if _, ok := m.Message.(*process.Resync); !ok {
			replica.options.Logger.Debugf("ignore message: expected height>=%v, got height=%v", replica.p.CurrentHeight(), m.Message.Height())
			return
		}
	}
	if m.Message.Type() == process.ResyncMessageType {
		if m.Message.Height() > replica.p.CurrentHeight() {
			// We cannot respond to resync messages from future heights with
			// anything that is useful, so we ignore it.
			replica.options.Logger.Debugf("ignore message: resync height=%v compared to current height=%v", m.Message.Height(), replica.p.CurrentHeight())
			return
		}
		// Filter resync messages by timestamp. If they're too old, or too far
		// in the future, then ignore them. The total window of time is 20
		// seconds, approximately the latency expected for globally distributed
		// message passing.
		now := block.Timestamp(time.Now().Unix())
		timestamp := m.Message.(*process.Resync).Timestamp()
		delta := now - timestamp
		if delta < 0 {
			delta = -delta
		}
		if delta > 10 {
			replica.options.Logger.Debugf("ignore message: resync timestamp=%v compared to now=%v", timestamp, now)
			return
		}
	}
	if m.Message.Type() == process.ProposeMessageType {
		if m.Message.Height() > replica.p.CurrentHeight()+1 {
			// If the Propose is not at the next height, then we need to make
			// sure that no base blocks have been missed. Otherwise, reject the
			// Propose, and wait until the appropriate one has been seen.
			baseBlockHash := replica.blockStorage.LatestBaseBlock(m.Shard).Hash()
			blockHash := m.Message.BlockHash()
			numBlocks := replica.rebaser.blockIterator.BaseBlocksInRange(baseBlockHash, blockHash)
			if numBlocks > 0 {
				// We have missed a base block, so we reject the Propose. This
				// assumes that the first Propose after the base block will
				// eventually be seen by the Process, as per the underlying
				// network assumptions.
				return
			}
		}
	}

	// Check that the Message sender is from our Shard (this can be a moderately
	// expensive operation, so we cache the result until a new `block.Base` is
	// detected)
	replica.cache.fillBaseBlock(replica.blockStorage.LatestBaseBlock(replica.shard))
	if !replica.cache.signatoryInBaseBlock(m.Message.Signatory()) {
		return
	}
	// Verify that the Message is actually signed by the claimed `id.Signatory`
	if err := process.Verify(m.Message); err != nil {
		replica.options.Logger.Warnf("bad message: unverified: %v", err)
		return
	}

	// Messages from future heights are buffered.
	if m.Message.Height() > replica.p.CurrentHeight() && m.Message.Type() != process.ProposeMessageType {
		// We only want to buffer non-Propose messages, because Propose messages
		// can have a LatestCommit message to fast-forward the process.
		replica.messageQueue.Push(m.Message)
	} else {
		// Handle the underlying `process.Message` and immediately save the
		// `process.Process` afterwards to protect against unexpected crashes
		replica.p.HandleMessage(m.Message)
	}
	defer replica.p.Save()

	queued := replica.messageQueue.PopUntil(replica.p.CurrentHeight())
	for queued != nil && len(queued) > 0 {
		for _, message := range queued {
			// Handle the underlying `process.Message` and immediately save the
			// `process.Process` afterwards to protect against unexpected
			// crashes
			replica.p.HandleMessage(message)
		}
		queued = replica.messageQueue.PopUntil(replica.p.CurrentHeight())
	}
}

func (replica *Replica) Rebase(sigs id.Signatories) {
	if len(sigs)%3 != 1 || len(sigs) < 4 {
		panic(fmt.Errorf("invariant violation: number of nodes needs to be 3f +1, got %v", len(sigs)))
	}
	replica.scheduler.Rebase(sigs)
	replica.rebaser.rebase(sigs)
}

type saveRestorer struct {
	pStorage ProcessStorage
	shard    Shard
}

func newSaveRestorer(pStorage ProcessStorage, shard Shard) *saveRestorer {
	return &saveRestorer{
		pStorage: pStorage,
		shard:    shard,
	}
}

func (saveRestorer *saveRestorer) Save(state *process.State) {
	saveRestorer.pStorage.SaveState(state, saveRestorer.shard)
}
func (saveRestorer *saveRestorer) Restore(state *process.State) {
	saveRestorer.pStorage.RestoreState(state, saveRestorer.shard)
}

type baseBlockCache struct {
	lastBaseBlockHeight block.Height
	lastBaseBlockHash   id.Hash
	lastBaseBlockSigs   id.Signatories
	sigsCache           map[id.Signatory]bool
}

func newBaseBlockCache(baseBlock block.Block) baseBlockCache {
	cache := baseBlockCache{
		lastBaseBlockHeight: -1,
		sigsCache:           map[id.Signatory]bool{},
	}
	cache.fillBaseBlock(baseBlock)
	return cache
}

func (cache *baseBlockCache) fillBaseBlock(baseBlock block.Block) {
	if baseBlock.Header().Height() <= cache.lastBaseBlockHeight {
		return
	}
	if baseBlock.Hash().Equal(cache.lastBaseBlockHash) {
		return
	}
	cache.lastBaseBlockHeight = baseBlock.Header().Height()
	cache.lastBaseBlockHash = baseBlock.Hash()
	cache.lastBaseBlockSigs = baseBlock.Header().Signatories()
	cache.sigsCache = map[id.Signatory]bool{}
}

func (cache *baseBlockCache) signatoryInBaseBlock(sig id.Signatory) bool {
	inBaseBlock, ok := cache.sigsCache[sig]
	if ok {
		return inBaseBlock
	}
	for _, baseBlockSig := range cache.lastBaseBlockSigs {
		if baseBlockSig.Equal(sig) {
			cache.sigsCache[sig] = true
			return true
		}
	}
	cache.sigsCache[sig] = false
	return false
}
