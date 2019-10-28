// Package hyperdrive a high-level package for running multiple instances of the
// Hyperdrive consensus algorithm for over multiple shards. The Hyperdrive
// interface is the main entry point for users.
//
// See [package
// process](https://godoc.org/github.com/renproject/hyperdrive/process) for the
// implementation of the core consensus algorithm.
//
// See [package
// replica](https://godoc.org/github.com/renproject/hyperdrive/replica) for the
// implementation of rebasing/sharding wrapped around the core consensus
// algorithm.
//
// See [package block](https://godoc.org/github.com/renproject/hyperdrive/block)
// for the definition of Blocks and their related data types.
package hyperdrive

import (
	"crypto/ecdsa"

	"github.com/renproject/hyperdrive/block"
	"github.com/renproject/hyperdrive/process"
	"github.com/renproject/hyperdrive/replica"
	"github.com/renproject/id"
	"github.com/renproject/phi"
)

type (
	// Hashes is a wrapper around the []Hash type.
	Hashes = id.Hashes
	// A Hash is the [32]byte output of a hashing function. Hyperdrive uses SHA2
	// for hashing.
	Hash = id.Hash
	// Signatures is a wrapper around the []Signature type.
	Signatures = id.Signatures
	// A Signature is the [65]byte output of an ECDSA signing algorithm.
	// Hyperdrive uses the secp256k1 curve for ECDSA signing.
	Signature = id.Signature
	// Signatories is a wrapper around the []Signatory type.
	Signatories = id.Signatories
	// A Signatory is the [32]byte resulting from hashing an ECDSA public key.
	// It represents the public identity of a content author and can be used to
	// authenticate content that has been signed.
	Signatory = id.Signatory
)

type (
	// Blocks is a wrapper type around the []Block type.
	Blocks = block.Blocks
	// A Block is an atomic unit of data upon which consensus is reached.
	// Everything upon which consensus is needed should be put into a Block, and
	// consensus can only be reached on a Block by Block basis (there is no
	// finer-grained way to express consensus).
	Block = block.Block
	// The Height in a blockchain at which a Block was proposed/committed.
	Height = block.Height
	// The Round in a consensus algorithm at which a Block was
	// proposed/committed.
	Round = block.Round
	// Timestamp is a wrapper around the uint64 type.
	Timestamp = block.Timestamp
	// BlockTxs represent the application-specific transactions that are being
	// proposed as part of a Block. An application that wishes to achieve
	// consensus on activity within the application should represent this
	// activity as transactions, serialise them into bytes, and put them into a
	// Block. No assumptions are made about the format of these transactions.
	BlockTxs = block.Txs
	// A BlockPlan represents application-specific data that is needed to
	// execute the transactions in a Block. No assumptions are made about the
	// format of this plan.
	BlockPlan = block.Plan
	// The BlockState represents application-specific state.
	BlockState = block.State
)

type (
	Blockchain   = process.Blockchain
	Process      = process.Process
	ProcessState = process.State
)

type (
	Messages       = replica.Messages
	Message        = replica.Message
	Shards         = replica.Shards
	Shard          = replica.Shard
	Options        = replica.Options
	Replicas       = replica.Replicas
	Replica        = replica.Replica
	ProcessStorage = replica.ProcessStorage
	BlockStorage   = replica.BlockStorage
	BlockIterator  = replica.BlockIterator
	Validator      = replica.Validator
	Observer       = replica.Observer
	Broadcaster    = replica.Broadcaster
)

var (
	// NewSignatory returns a Signatory from an ECDSA public key by serializing
	// the ECDSA public key into bytes, and then hashing it using a SHA2 hashing
	// function.
	NewSignatory = id.NewSignatory
)

var (
	StandardBlockKind = block.Standard
	RebaseBlockKind   = block.Rebase
	BaseBlockKind     = block.Base
	NewBlock          = block.New
	NewBlockHeader    = block.NewHeader
)

// Hyperdrive manages multiple `Replicas` from different
// `Shards`.
type Hyperdrive interface {
	Start()
	Rebase(sigs Signatories)
	HandleMessage(message Message)
}

type hyperdrive struct {
	replicas map[Shard]Replica
}

// New returns a new Hyperdrive instance that wraps multiple Replica instances.
// One Replica instance will be created per Shard, but all Replica instances
// will use the same interfaces and private key. Replicas will not be created
// for Shards for which the Replica is not a Signatory. This means that rebasing
// can shuffle Signatories, but it cannot introduce new ones or remove existing
// ones (this will be supported in future updates).
//
// ```go
// hyper := hyperdrive.New(
// 	hyperdrive.Options{},
// 	pStorage,
// 	bStorage,
// 	bIter,
// 	validator,
// 	observer,
// 	broadcaster,
// 	shards,
// 	privKey,
// )
// hyper.Start()
// for {
// 	select {
// 	case <-ctx.Done():
// 		break
// 	case message, ok := <-messagesFromNetwork:
// 		if !ok {
// 			break
// 		}
// 		hyper.HandleMessage(message)
// 	}
// }
// ```
func New(options Options, pStorage ProcessStorage, blockStorage BlockStorage, blockIterator BlockIterator, validator Validator, observer Observer, broadcaster Broadcaster, shards Shards, privKey ecdsa.PrivateKey) Hyperdrive {
	replicas := make(map[Shard]Replica, len(shards))
	for _, shard := range shards {
		if observer.IsSignatory(shard) {
			replicas[shard] = replica.New(options, pStorage, blockStorage, blockIterator, validator, observer, broadcaster, shard, privKey)
		}
	}
	return &hyperdrive{
		replicas: replicas,
	}
}

// Start all Replicas in the Hyperdrive instance.
func (hyper *hyperdrive) Start() {
	phi.ParForAll(hyper.replicas, func(shard Shard) {
		replica := hyper.replicas[shard]
		replica.Start()
	})
}

func (hyper *hyperdrive) Rebase(sigs Signatories) {
	for shard, replica := range hyper.replicas {
		replica.Rebase(sigs)
		hyper.replicas[shard] = replica
	}
}

func (hyper *hyperdrive) HandleMessage(message Message) {
	replica, ok := hyper.replicas[message.Shard]
	if !ok {
		return
	}
	defer func() {
		hyper.replicas[message.Shard] = replica
	}()
	replica.HandleMessage(message)
}
