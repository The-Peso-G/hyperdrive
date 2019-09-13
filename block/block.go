package block

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/renproject/id"
	"golang.org/x/crypto/sha3"
)

// Kind defines the different kinds of Block that exist.
type Kind uint8

const (
	// Invalid defines an invalid Kind that must not be used.
	Invalid Kind = iota

	// Standard Blocks are used when reaching consensus on the ordering of
	// application-specific data. Standard Blocks must have nil Header
	// Signatories. This is the most common Block Kind.
	Standard
	// Rebase Blocks are used when reaching consensus about a change to the
	// Header Signatories that oversee the consensus algorithm. Rebase Blocks
	// must include non-empty Header Signatories.
	Rebase
	// Base Blocks are used to finalise Rebase Blocks. Base Blocks must come
	// immediately after a Rebase Block, must have no Content, and must have the
	// same Header Signatories as their parent.
	Base
)

// String implements the `fmt.Stringer` interface for the Kind type.
func (kind Kind) String() string {
	switch kind {
	case Standard:
		return "standard"
	case Rebase:
		return "rebase"
	case Base:
		return "base"
	default:
		panic(fmt.Errorf("invariant violation: unexpected kind=%d", uint8(kind)))
	}
}

// A Header defines properties of a Block that are not application-specific.
// These properties are required by, or produced by, the consensus algorithm.
type Header struct {
	kind       Kind      // Kind of Block
	parentHash id.Hash   // Hash of the Block parent
	baseHash   id.Hash   // Hash of the Block base
	height     Height    // Height at which the Block was committed
	round      Round     // Round at which the Block was committed
	timestamp  Timestamp // Seconds since Unix Epoch

	// Signatories oversee the consensus algorithm (must be nil unless the Block
	// is a Rebase/Base Block)
	signatories id.Signatories
}

// NewHeader returns a Header. It will panic if a pre-condition for Header
// validity is violated.
func NewHeader(kind Kind, parentHash, baseHash id.Hash, height Height, round Round, timestamp Timestamp, signatories id.Signatories) Header {
	switch kind {
	case Standard:
		if signatories != nil {
			panic("pre-condition violation: standard blocks must not declare signatories")
		}
	case Rebase, Base:
		if len(signatories) == 0 {
			panic(fmt.Sprintf("pre-condition violation: %v blocks must declare signatories", kind))
		}
	default:
		panic(fmt.Errorf("pre-condition violation: unexpected block kind=%v", kind))
	}
	if parentHash.Equal(InvalidHash) {
		panic(fmt.Errorf("pre-condition violation: invalid parent hash=%v", parentHash))
	}
	if baseHash.Equal(InvalidHash) {
		panic(fmt.Errorf("pre-condition violation: invalid base hash=%v", baseHash))
	}
	if height <= InvalidHeight {
		panic(fmt.Errorf("pre-condition violation: invalid height=%v", height))
	}
	if round <= InvalidRound {
		panic(fmt.Errorf("pre-condition violation: invalid round=%v", round))
	}
	if Timestamp(time.Now().Unix()) < timestamp {
		panic("pre-condition violation: timestamp has not passed")
	}
	return Header{
		kind:        kind,
		parentHash:  parentHash,
		baseHash:    baseHash,
		height:      height,
		round:       round,
		timestamp:   timestamp,
		signatories: signatories,
	}
}

// Kind of the Block.
func (header Header) Kind() Kind {
	return header.kind
}

// ParentHash of the Block.
func (header Header) ParentHash() id.Hash {
	return header.parentHash
}

// BaseHash of the Block.
func (header Header) BaseHash() id.Hash {
	return header.baseHash
}

// Height of the Block.
func (header Header) Height() Height {
	return header.height
}

// Round of the Block.
func (header Header) Round() Round {
	return header.round
}

// Timestamp of the Block in seconds since Unix Epoch.
func (header Header) Timestamp() Timestamp {
	return header.timestamp
}

// Signatories of the Block.
func (header Header) Signatories() id.Signatories {
	return header.signatories
}

// String implements the `fmt.Stringer` interface for the Header type.
func (header Header) String() string {
	return fmt.Sprintf(
		"Header(Kind=%v,ParentHash=%v,BaseHash=%v,Height=%v,Round=%v,Timestamp=%v,Signatories=%v)",
		header.kind,
		header.parentHash,
		header.baseHash,
		header.height,
		header.round,
		header.timestamp,
		header.signatories,
	)
}

// MarshalJSON implements the `json.Marshaler` interface for the Header type.
func (header Header) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind        Kind           `json:"kind"`
		ParentHash  id.Hash        `json:"parentHash"`
		BaseHash    id.Hash        `json:"baseHash"`
		Height      Height         `json:"height"`
		Round       Round          `json:"round"`
		Timestamp   Timestamp      `json:"timestamp"`
		Signatories id.Signatories `json:"signatories"`
	}{
		header.kind,
		header.parentHash,
		header.baseHash,
		header.height,
		header.round,
		header.timestamp,
		header.signatories,
	})
}

// UnmarshalJSON implements the `json.Unmarshaler` interface for the Header type.
func (header *Header) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Kind        Kind           `json:"kind"`
		ParentHash  id.Hash        `json:"parentHash"`
		BaseHash    id.Hash        `json:"baseHash"`
		Height      Height         `json:"height"`
		Round       Round          `json:"round"`
		Timestamp   Timestamp      `json:"timestamp"`
		Signatories id.Signatories `json:"signatories"`
	}{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	header.kind = tmp.Kind
	header.parentHash = tmp.ParentHash
	header.baseHash = tmp.BaseHash
	header.height = tmp.Height
	header.round = tmp.Round
	header.timestamp = tmp.Timestamp
	header.signatories = tmp.Signatories
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// Header type.
func (header Header) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, header.kind); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.kind: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, header.parentHash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.parentHash: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, header.baseHash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.baseHash: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, header.height); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.height: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, header.round); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.round: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, header.timestamp); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.timestamp: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(header.signatories))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write header.signatories len: %v", err)
	}
	for _, sig := range header.signatories {
		if err := binary.Write(buf, binary.LittleEndian, sig); err != nil {
			return buf.Bytes(), fmt.Errorf("cannot write header.signatories data: %v", err)
		}
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// Header type.
func (header *Header) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &header.kind); err != nil {
		return fmt.Errorf("cannot read header.kind: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.parentHash); err != nil {
		return fmt.Errorf("cannot read header.parentHash: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.baseHash); err != nil {
		return fmt.Errorf("cannot read header.baseHash: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.height); err != nil {
		return fmt.Errorf("cannot read header.height: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.round); err != nil {
		return fmt.Errorf("cannot read header.round: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &header.timestamp); err != nil {
		return fmt.Errorf("cannot read header.timestamp: %v", err)
	}
	var lenSignatories uint64
	if err := binary.Read(buf, binary.LittleEndian, &lenSignatories); err != nil {
		return fmt.Errorf("cannot read header.signatories len: %v", err)
	}
	if lenSignatories > 0 {
		header.signatories = make(id.Signatories, lenSignatories)
		for i := uint64(0); i < lenSignatories; i++ {
			if err := binary.Read(buf, binary.LittleEndian, &header.signatories[i]); err != nil {
				return fmt.Errorf("cannot read header.signatories data: %v", err)
			}
		}
	}
	return nil
}

// Data stores application-specific information used in Blocks and Notes (must
// be nil in Rebase Blocks and Base Blocks).
type Data []byte

// String implements the `fmt.Stringer` interface for the Data type.
func (data Data) String() string {
	return base64.RawStdEncoding.EncodeToString(data)
}

// State stores application-specific state after the execution of a Block.
type State []byte

// String implements the `fmt.Stringer` interface for the State type.
func (state State) String() string {
	return base64.RawStdEncoding.EncodeToString(state)
}

// Blocks defines a wrapper type around the []Block type.
type Blocks []Block

// A Block is the atomic unit upon which consensus is reached. Consensus
// guarantees a consistent ordering of Blocks that is agreed upon by all members
// in a distributed network, even when some of the members are malicious.
type Block struct {
	hash      id.Hash // Hash of the Header, Data, and State
	header    Header
	data      Data
	prevState State
}

// New Block with the Header, Data, and State of the Block parent. The Block
// Hash will automatically be computed and set.
func New(header Header, data Data, prevState State) Block {
	return Block{
		hash:      ComputeHash(header, data, prevState),
		header:    header,
		data:      data,
		prevState: prevState,
	}
}

// Hash returns the 256-bit SHA3 Hash of the Header and Data.
func (block Block) Hash() id.Hash {
	return block.hash
}

// Header of the Block.
func (block Block) Header() Header {
	return block.header
}

// Data embedded in the Block for application-specific purposes.
func (block Block) Data() Data {
	return block.data
}

// PreviousState embedded in the Block for application-specific state after the
// execution of the Block parent.
func (block Block) PreviousState() State {
	return block.prevState
}

// String implements the `fmt.Stringer` interface for the Block type.
func (block Block) String() string {
	return fmt.Sprintf("Block(Hash=%v,Header=%v,Data=%v,PreviousState=%v)", block.hash, block.header, block.data, block.prevState)
}

// Equal compares one Block with another by checking that their Hashes are the
// equal, and their Notes are equal.
func (block Block) Equal(other Block) bool {
	return block.String() == other.String()
}

// MarshalJSON implements the `json.Marshaler` interface for the Block type.
func (block Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Hash      id.Hash `json:"hash"`
		Header    Header  `json:"header"`
		Data      Data    `json:"data"`
		PrevState State   `json:"prevState"`
	}{
		block.hash,
		block.header,
		block.data,
		block.prevState,
	})
}

// UnmarshalJSON implements the `json.Unmarshaler` interface for the Block type.
func (block *Block) UnmarshalJSON(data []byte) error {
	tmp := struct {
		Hash      id.Hash `json:"hash"`
		Header    Header  `json:"header"`
		Data      Data    `json:"data"`
		PrevState State   `json:"prevState"`
	}{}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	block.hash = tmp.Hash
	block.header = tmp.Header
	block.data = tmp.Data
	block.prevState = tmp.PrevState
	return nil
}

// MarshalBinary implements the `encoding.BinaryMarshaler` interface for the
// Block type.
func (block Block) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, block.hash); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.hash: %v", err)
	}
	headerData, err := block.header.MarshalBinary()
	if err != nil {
		return buf.Bytes(), fmt.Errorf("cannot marshal block.header: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(headerData))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.header len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, headerData); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.header data: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(block.data))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.data len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, block.data); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.data data: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(block.prevState))); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.prevState len: %v", err)
	}
	if err := binary.Write(buf, binary.LittleEndian, block.prevState); err != nil {
		return buf.Bytes(), fmt.Errorf("cannot write block.prevState data: %v", err)
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary implements the `encoding.BinaryUnmarshaler` interface for the
// Block type.
func (block *Block) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &block.hash); err != nil {
		return fmt.Errorf("cannot read block.hash: %v", err)
	}
	var numBytes uint64
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read block.header len: %v", err)
	}
	headerBytes := make([]byte, numBytes)
	if _, err := buf.Read(headerBytes); err != nil {
		return fmt.Errorf("cannot read block.header data: %v", err)
	}
	if err := block.header.UnmarshalBinary(headerBytes); err != nil {
		return fmt.Errorf("cannot unmarshal block.header: %v", err)
	}
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read block.data len: %v", err)
	}
	if numBytes > 0 {
		dataBytes := make([]byte, numBytes)
		if _, err := buf.Read(dataBytes); err != nil {
			return fmt.Errorf("cannot read block.data data: %v", err)
		}
		block.data = dataBytes
	}
	if err := binary.Read(buf, binary.LittleEndian, &numBytes); err != nil {
		return fmt.Errorf("cannot read block.prevState len: %v", err)
	}
	if numBytes > 0 {
		prevStateBytes := make([]byte, numBytes)
		if _, err := buf.Read(prevStateBytes); err != nil {
			return fmt.Errorf("cannot read block.prevState data: %v", err)
		}
		block.prevState = prevStateBytes
	}
	return nil
}

// Timestamp represents seconds since Unix Epoch.
type Timestamp uint64

// Height of a Block.
type Height int64

// Round in which a Block was proposed.
type Round int64

// Define some default invalid values.
var (
	InvalidHash      = id.Hash{}
	InvalidSignature = id.Signature{}
	InvalidSignatory = id.Signatory{}
	InvalidBlock     = Block{}
	InvalidRound     = Round(-1)
	InvalidHeight    = Height(-1)
)

// ComputeHash of a block basing on its header, data and previous state.
func ComputeHash(header Header, data Data, prevState State) id.Hash {
	return sha3.Sum256([]byte(fmt.Sprintf("BlockHash(Header=%v,Data=%v,PreviousState=%v)", header, data, prevState)))
}
