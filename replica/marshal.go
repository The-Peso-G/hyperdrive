package replica

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/renproject/hyperdrive/process"
	"github.com/renproject/id"
	"github.com/renproject/surge"
)

// SigHash of the Message including the Shard.
func (message Message) SigHash() id.Hash {
	m := surge.MaxBytes
	buf := new(bytes.Buffer)
	buf.Grow(surge.SizeHint(message.Message.Type()) +
		surge.SizeHint(message.Message) +
		surge.SizeHint(message.Shard))
	m, err := surge.Marshal(buf, uint64(message.Message.Type()), m)
	if err != nil {
		panic(fmt.Errorf("bad message sighash: bad marshal: %v", err))
	}
	if m, err = surge.Marshal(buf, message.Message, m); err != nil {
		panic(fmt.Errorf("bad message sighash: bad marshal: %v", err))
	}
	if m, err = surge.Marshal(buf, message.Shard, m); err != nil {
		panic(fmt.Errorf("bad message sighash: bad marshal: %v", err))
	}
	return id.Hash(sha256.Sum256(buf.Bytes()))
}

// SizeHint returns the number of bytes requires to store this message in
// binary.
func (message Message) SizeHint() int {
	return surge.SizeHint(message.Message.Type()) +
		surge.SizeHint(message.Message) +
		surge.SizeHint(message.Shard) +
		surge.SizeHint(message.Signature)
}

// Marshal this message into binary.
func (message Message) Marshal(w io.Writer, m int) (int, error) {
	m, err := surge.Marshal(w, uint64(message.Message.Type()), m)
	if err != nil {
		return m, err
	}
	if m, err = surge.Marshal(w, message.Message, m); err != nil {
		return m, err
	}
	if m, err = surge.Marshal(w, message.Shard, m); err != nil {
		return m, err
	}
	return surge.Marshal(w, message.Signature, m)
}

// Unmarshal into this message from binary.
func (message *Message) Unmarshal(r io.Reader, m int) (int, error) {
	var messageType process.MessageType
	m, err := surge.Unmarshal(r, &messageType, m)
	if err != nil {
		return m, err
	}

	switch messageType {
	case process.ProposeMessageType:
		propose := new(process.Propose)
		m, err = propose.Unmarshal(r, m)
		message.Message = propose
	case process.PrevoteMessageType:
		prevote := new(process.Prevote)
		m, err = prevote.Unmarshal(r, m)
		message.Message = prevote
	case process.PrecommitMessageType:
		precommit := new(process.Precommit)
		m, err = precommit.Unmarshal(r, m)
		message.Message = precommit
	case process.ResyncMessageType:
		resync := new(process.Resync)
		m, err = resync.Unmarshal(r, m)
		message.Message = resync
	default:
		return m, fmt.Errorf("unexpected message type %d", messageType)
	}
	if err != nil {
		return m, err
	}

	if m, err = surge.Unmarshal(r, &message.Shard, m); err != nil {
		return m, err
	}
	return surge.Unmarshal(r, &message.Signature, m)
}
