package testutil_replica

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	mrand "math/rand"
	"sync"
	"time"

	"github.com/renproject/hyperdrive/block"
	"github.com/renproject/hyperdrive/process"
	"github.com/renproject/hyperdrive/replica"
	"github.com/renproject/hyperdrive/testutil"
	"github.com/renproject/id"
	"github.com/renproject/phi"
)

func Contain(list []int, target int) bool {
	for _, num := range list {
		if num == target {
			return true
		}
	}
	return false
}

func SleepRandomSeconds(min, max int) {
	if max == min {
		time.Sleep(time.Duration(min) * time.Second)
	} else {
		duration := time.Duration(mrand.Intn(max-min) + min)
		time.Sleep(duration * time.Second)
	}
}

func RandomShard() replica.Shard {
	shard := replica.Shard{}
	_, err := rand.Read(shard[:])
	if err != nil {
		panic(fmt.Sprintf("cannot create random shard, err = %v", err))
	}
	return shard
}

type MockBlockIterator struct {
	store *MockPersistentStorage
	err   error
}

func NewMockBlockIterator(store *MockPersistentStorage, err error) *MockBlockIterator {
	return &MockBlockIterator{
		store: store,
		err:   err,
	}
}

func (m *MockBlockIterator) NextBlock(kind block.Kind, height block.Height, shard replica.Shard) (block.Txs, block.Plan, block.State) {
	// Return an invalid block if we are a faulty proposer.
	if m.err != nil {
		return nil, nil, nil
	}

	blockchain := m.store.MockBlockchain(shard)
	state, ok := blockchain.StateAtHeight(height - 1)
	if !ok {
		return testutil.RandomBytesSlice(), testutil.RandomBytesSlice(), nil
	}

	switch kind {
	case block.Standard:
		return testutil.RandomBytesSlice(), testutil.RandomBytesSlice(), state
	default:
		panic("unknown block kind")
	}
}

type MockValidator struct {
	store *MockPersistentStorage
}

func NewMockValidator(store *MockPersistentStorage) replica.Validator {
	return &MockValidator{
		store: store,
	}
}

func (m *MockValidator) IsBlockValid(b block.Block, checkHistory bool, shard replica.Shard) (process.NilReasons, error) {
	height := b.Header().Height()
	prevState := b.PreviousState()

	blockchain := m.store.MockBlockchain(shard)
	if !checkHistory {
		return nil, nil
	}

	state, ok := blockchain.StateAtHeight(height - 1)
	if !ok {
		return nil, fmt.Errorf("failed to get state at height %d", height-1)
	}
	if !bytes.Equal(prevState, state) {
		return nil, fmt.Errorf("invalid previous state")
	}
	return nil, nil
}

type MockObserver struct {
	store       *MockPersistentStorage
	isSignatory bool
}

func NewMockObserver(store *MockPersistentStorage, isSignatory bool) replica.Observer {
	return &MockObserver{
		store:       store,
		isSignatory: isSignatory,
	}
}

func (m MockObserver) DidCommitBlock(height block.Height, shard replica.Shard) {
	blockchain := m.store.MockBlockchain(shard)
	b, ok := blockchain.BlockAtHeight(height)
	if !ok {
		panic("DidCommitBlock should be called only when the block has been added to storage")
	}
	digest := sha256.Sum256(b.Txs())
	blockchain.InsertBlockStatAtHeight(height, digest[:])

	// Insert executed state of the previous height
	prevBlock, ok := blockchain.BlockAtHeight(height - 1)
	if !ok {
		panic(fmt.Sprintf("cannot find block of height %v, %v", height-1, prevBlock))
	}
	blockchain.InsertBlockStatAtHeight(height-1, prevBlock.PreviousState())
}

func (observer *MockObserver) IsSignatory(replica.Shard) bool {
	return observer.isSignatory
}

func (observer *MockObserver) DidReceiveSufficientNilPrevotes(process.Messages, int) {
}

type latestMessages struct {
	Mu        *sync.RWMutex
	Height    block.Height
	Propose   replica.Message
	Prevote   replica.Message
	Precommit replica.Message
}

type MockBroadcaster struct {
	min, max int

	mu     *sync.RWMutex
	cons   map[id.Signatory]chan replica.Message
	active map[id.Signatory]bool

	signatories map[id.Signatory]int
}

func NewMockBroadcaster(keys []*ecdsa.PrivateKey, min, max int) *MockBroadcaster {
	cons := map[id.Signatory]chan replica.Message{}
	signatories := map[id.Signatory]int{}
	for i, key := range keys {
		sig := id.NewSignatory(key.PublicKey)
		messages := make(chan replica.Message, 128)
		cons[sig] = messages
		signatories[sig] = i
	}

	return &MockBroadcaster{
		min: min,
		max: max,

		mu:          new(sync.RWMutex),
		cons:        cons,
		active:      map[id.Signatory]bool{},
		signatories: signatories,
	}
}

func (m *MockBroadcaster) Broadcast(message replica.Message) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// If the sender is offline, it cannot send messages to other nodes.
	if !m.active[message.Message.Signatory()] {
		return
	}

	phi.ParForAll(m.cons, func(sig id.Signatory) {
		// If the receiver is offline, it cannot receive any messages from other
		// nodes.
		if m.active[sig] {
			m.sendMessage(sig, message)
		}
	})
}

func (m *MockBroadcaster) Cast(to id.Signatory, message replica.Message) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.active[message.Message.Signatory()] {
		return
	}

	if m.active[to] {
		m.sendMessage(to, message)
	}
}

func (m *MockBroadcaster) sendMessage(receiver id.Signatory, message replica.Message) {
	messages := m.cons[receiver]
	time.Sleep(time.Duration(mrand.Intn(m.max-m.min)+m.min) * time.Millisecond) // Simulate network latency.
	messages <- message
}

func (m *MockBroadcaster) Messages(sig id.Signatory) chan replica.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.cons[sig]
}

func (m *MockBroadcaster) EnablePeer(sig id.Signatory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.active[sig] = true
}

func (m *MockBroadcaster) DisablePeer(sig id.Signatory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.active[sig] = false
}
