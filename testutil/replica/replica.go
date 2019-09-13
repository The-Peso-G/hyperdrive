package testutil_replica

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
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
	"golang.org/x/crypto/sha3"
)

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
}

func NewMockBlockIterator(store *MockPersistentStorage) replica.BlockIterator {
	return &MockBlockIterator{
		store: store,
	}
}

func (m *MockBlockIterator) NextBlock(kind block.Kind, height block.Height, shard replica.Shard) (block.Data, block.State) {
	blockchain := m.store.MockBlockchain(shard)
	state, ok := blockchain.StateAtHeight(height - 1)
	if !ok {
		return testutil.RandomBytesSlice(), nil
	}

	switch kind {
	case block.Standard:
		return testutil.RandomBytesSlice(), state
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

func (m *MockValidator) IsBlockValid(b block.Block, checkHistory bool, shard replica.Shard) bool {
	height := b.Header().Height()
	prevState := b.PreviousState()

	blockchain := m.store.MockBlockchain(shard)
	if !checkHistory {
		return true
	}

	state, ok := blockchain.StateAtHeight(height - 1)
	if !ok {
		return false
	}
	if !bytes.Equal(prevState, state) {
		return false
	}
	return true
}

type MockObserver struct {
	store *MockPersistentStorage
}

func NewMockObserver(store *MockPersistentStorage) replica.Observer {
	return &MockObserver{
		store: store,
	}
}

func (m MockObserver) DidCommitBlock(height block.Height, shard replica.Shard) {
	blockchain := m.store.MockBlockchain(shard)
	block, ok := blockchain.BlockAtHeight(height)
	if !ok {
		panic("DidCommitBlock should be called only when the block has been added to storage")
	}
	digest := sha3.Sum256(block.Data())
	blockchain.InsertBlockStatAtHeight(height, digest[:])

	// Insert executed state of the previous height
	prevBlock, ok := blockchain.BlockAtHeight(height - 1)
	if !ok {
		panic(fmt.Sprintf("cannot find block of height %v, %v", height-1, prevBlock))
	}
	blockchain.InsertBlockStatAtHeight(height-1, prevBlock.PreviousState())
}

type MockBroadcaster struct {
	min, max int

	mu     *sync.RWMutex
	cons   map[id.Signatory]chan replica.Message
	active map[id.Signatory]bool
}

func NewMockBroadcaster(keys []*ecdsa.PrivateKey, min, max int) *MockBroadcaster {
	cons := map[id.Signatory]chan replica.Message{}
	for _, key := range keys {
		sig := id.NewSignatory(key.PublicKey)
		messages := make(chan replica.Message, 128)
		cons[sig] = messages
	}

	return &MockBroadcaster{
		min:    min,
		max:    max,
		mu:     new(sync.RWMutex),
		cons:   cons,
		active: map[id.Signatory]bool{},
	}
}

func (m *MockBroadcaster) Broadcast(message replica.Message) {
	var sender id.Signatory
	switch msg := message.Message.(type) {
	case *process.Propose:
		sender = msg.Signatory()
	case *process.Prevote:
		sender = msg.Signatory()
	case *process.Precommit:
		sender = msg.Signatory()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// If the sender is offline, it cannot send message to other nodes
	if !m.active[sender] {
		return
	}

	// If the receiver is offline, it cannot receive any message from other nodes.
	phi.ParForAll(m.cons, func(sig id.Signatory) {
		if m.active[sig] {
			m.sendMessage(sig, message)
		} else {
			// Retry sending the message three times if the node is offline
			go func() {
				for i := 0; i < 3; i++ {
					m.mu.RLock()
					if m.active[sig] {
						m.sendMessage(sig, message)
					}
					m.mu.RUnlock()
					time.Sleep(3 * time.Second)
				}
			}()
		}
	})
}

func (m *MockBroadcaster) sendMessage(receiver id.Signatory, message replica.Message) {
	messages := m.cons[receiver]
	// Simulate the network latency
	time.Sleep(time.Duration(mrand.Intn(m.max-m.min)+m.min) * time.Millisecond)

	// Drop the message if the node is not online
	select {
	case messages <- message:
	default:
		return
	}
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
