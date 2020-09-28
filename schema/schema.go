package schema

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
)

// Class represents the type of test vector this instance is.
type Class string

const (
	// ClassMessage tests the VM behaviour and resulting state over one or
	// many messages.
	ClassMessage Class = "message"
	// ClassTipset tests the VM behaviour and resulting state over one or many
	// tipsets and/or null rounds.
	ClassTipset Class = "tipset"
	// ClassBlockSeq tests the state of the system after the arrival of
	// particular blocks at concrete points in time.
	ClassBlockSeq Class = "blockseq"
)

const (
	// HintIncorrect is a standard hint to convey that a vector is knowingly
	// incorrect. Drivers may choose to skip over these vectors, or if it's
	// accompanied by HintNegate, they may perform the assertions as explained
	// in its godoc.
	HintIncorrect = "incorrect"

	// HintNegate is a standard hint to convey to drivers that, if this vector
	// is run, they should negate the postcondition checks (i.e. check that the
	// postcondition state is expressly NOT the one encoded in this vector).
	HintNegate = "negate"
)

// Selector is a predicate the driver can use to determine if this test vector
// is relevant given the capabilities/features of the underlying implementation
// and/or test environment.
type Selector map[string]string

// Metadata provides information on the generation of this test case
type Metadata struct {
	ID      string           `json:"id"`
	Version string           `json:"version,omitempty"`
	Desc    string           `json:"description,omitempty"`
	Comment string           `json:"comment,omitempty"`
	Gen     []GenerationData `json:"gen"`
	Tags    []string         `json:"tags,omitempty"`
}

// GenerationData tags the source of this test case.
type GenerationData struct {
	Source  string `json:"source,omitempty"`
	Version string `json:"version,omitempty"`
}

// StateTree represents a state tree within preconditions and postconditions.
type StateTree struct {
	RootCID cid.Cid `json:"root_cid"`
}

// Base64EncodedBytes is a base64-encoded binary value.
type Base64EncodedBytes []byte

// ChainHead represents a head tipset.
type ChainHead []cid.Cid

// PreconditionsBlockSeq are the preconditions for a blockseq vector.
type PreconditionsBlockSeq struct {
	GenesisTs time.Time  `json:"genesis_ts,omitempty"`
	ChainHead *ChainHead `json:"chain_head,omitempty"`
}

// Preconditions contain the environment that needs to be set before the
// vector's applies are applied.
type Preconditions struct {
	*PreconditionsBlockSeq

	// Epoch must be interpreted by the driver as an abi.ChainEpoch in Lotus, or
	// equivalent type in other implementations.
	Epoch     int64      `json:"epoch,omitempty"`
	StateTree *StateTree `json:"state_tree,omitempty"`

	// CircSupply is optional. If specified, it is the value that will be
	// injected in the VM when running this vector. If absent, the default
	// value will be injected (TotalFilecoin, the maximum supply of Filecoin
	// that will ever exist). It is usually odd to set it, and it's only here
	// for specialized vectors.
	CircSupply *int64 `json:"circ_supply,omitempty"`
}

// Receipt represents a receipt to match against.
type Receipt struct {
	// ExitCode must be interpreted by the driver as an exitcode.ExitCode
	// in Lotus, or equivalent type in other implementations.
	ExitCode    int64              `json:"exit_code"`
	ReturnValue Base64EncodedBytes `json:"return"`
	GasUsed     int64              `json:"gas_used"`
}

// Postconditions contain a representation of VM state at th end of the test
type Postconditions struct {
	ApplyMessageFailures []int      `json:"apply_message_failures,omitempty"`
	StateTree            *StateTree `json:"state_tree"`
	Receipts             []*Receipt `json:"receipts"`
	ReceiptsRoots        []cid.Cid  `json:"receipts_roots,omitempty"`
}

// MarshalJSON implements json.Marshal for Base64EncodedBytes
func (b Base64EncodedBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(b))
}

// UnmarshalJSON implements json.Unmarshal for Base64EncodedBytes
func (b *Base64EncodedBytes) UnmarshalJSON(v []byte) error {
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return err
	}

	bytes, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	*b = bytes
	return nil
}

// Diagnostics contain a representation of VM diagnostics
type Diagnostics struct {
	Format string             `json:"format"`
	Data   Base64EncodedBytes `json:"data"`
}

// OffsetMillis is a type that serializes as uint64 in json, and represents a
// duration in milliseconds.
type OffsetMillis struct {
	time.Duration
}

var (
	_ json.Unmarshaler = (*OffsetMillis)(nil)
	_ json.Marshaler   = (*OffsetMillis)(nil)
)

func (om *OffsetMillis) UnmarshalJSON(b []byte) error {
	var ms uint64
	if err := json.Unmarshal(b, &ms); err != nil {
		return fmt.Errorf("failed to unmarshal milliseconds offset: %w", err)
	}
	*om = OffsetMillis{time.Duration(ms) * time.Millisecond}
	return nil
}

func (om OffsetMillis) MarshalJSON() ([]byte, error) {
	return json.Marshal(om.Duration.Milliseconds())
}

type TimestampedRawBlock struct {
	// OffsetMs is the offset in milliseconds from genesis where this block is
	// received.
	OffsetMs OffsetMillis `json:"offset_ms"`

	// Bytes is the CBOR-encoded types.BlockMsg, the same type that gets
	// sent in the network over pubsub. types.BlockMsg contains the block header
	// and the message CIDs.
	Bytes Base64EncodedBytes `json:"bytes"`
}

// BlockSeq enumerates the blocks to be applied, and provides a message
// repository that contains the message payloads.
type BlockSeq struct {
	// Blocks is the sequence of timestamped blocks that this vector applies.
	Blocks []TimestampedRawBlock `json:"blocks"`
	// MessageRepo is the repository of messages mapping CIDs to message
	// payloads.
	MessageRepo map[cid.Cid]Base64EncodedBytes `json:"message_repo"`
}

// TestVector is a single test case
type TestVector struct {
	Class    `json:"class"`
	Selector `json:"selector,omitempty"`

	// Hints are arbitrary flags that convey information to the driver.
	// Use hints to express facts like this vector is knowingly incorrect
	// (e.g. when the reference implementation is broken), or that drivers
	// should negate the postconditions (i.e. test that they are NOT the ones
	// expressed in the vector), etc.
	//
	// Refer to the Hint* constants for common hints.
	Hints []string `json:"hints,omitempty"`

	Meta *Metadata `json:"_meta"`

	// CAR binary data to be loaded into the test environment, usually a CAR
	// containing multiple state trees, addressed by root CID from the relevant
	// objects.
	CAR Base64EncodedBytes `json:"car"`

	Pre *Preconditions `json:"preconditions"`

	ApplyMessages []Message `json:"apply_messages,omitempty"`
	ApplyTipsets  []Tipset  `json:"apply_tipsets,omitempty"`
	ApplyBlockseq *BlockSeq `json:"apply_blockseq,omitempty"`

	Post        *Postconditions `json:"postconditions"`
	Diagnostics *Diagnostics    `json:"diagnostics,omitempty"`
}

type Message struct {
	Bytes Base64EncodedBytes `json:"bytes"`
	// Epoch must be interpreted by the driver as an abi.ChainEpoch in Lotus, or
	// equivalent type in other implementations.
	Epoch *int64 `json:"epoch,omitempty"`
}

type Tipset struct {
	// Epoch must be interpreted by the driver as an abi.ChainEpoch in Lotus, or
	// equivalent type in other implementations.
	Epoch int64 `json:"epoch"`
	// BaseFee must be interpreted by the driver as an abi.TokenAmount in Lotus,
	// or equivalent type in other implementations.
	BaseFee big.Int `json:"basefee"`
	Blocks  []Block `json:"blocks,omitempty"`
}

type Block struct {
	MinerAddr address.Address      `json:"miner_addr"`
	WinCount  int64                `json:"win_count"`
	Messages  []Base64EncodedBytes `json:"messages"`
}

// Validate validates this test vector against the JSON schema, and applies
// further validation rules that cannot be enforced through JSON Schema.
func (tv TestVector) Validate() error {
	if tv.Class == ClassMessage {
		if len(tv.Post.Receipts) != len(tv.ApplyMessages) {
			return fmt.Errorf("length of postcondition receipts must match length of messages to apply")
		}
	}
	return nil
}

// MustMarshalJSON encodes the test vector to JSON and panics if it errors.
func (tv TestVector) MustMarshalJSON() []byte {
	b, err := json.Marshal(&tv)
	if err != nil {
		panic(err)
	}
	return b
}
