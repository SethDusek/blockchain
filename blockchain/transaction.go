package blockchain

import (
	"blockchain/schnorr"
	"bytes"
	"encoding/binary"
	"fmt"
)

// An unspent transaction output.
type UTXO struct {
	txout     [32]byte
	outpoint  uint32
}

// An output has a value and a challenge (a public key that must pass verification)
type Output struct {
	value     uint64
	challenge schnorr.PublicKey
}

type Transaction struct {
	inputs  []UTXO
	proofs  []schnorr.Signature
	outputs []Output
}


// Generates the bytes that the signer must commit to
func (tx Transaction) GenerateComittment() {
	buf := new(bytes.Buffer)

	for _, input := range tx.inputs {
		buf.Write(input.txout[:])
		binary.Write(buf, binary.BigEndian, input.outpoint)
	}
	for _, output := range tx.outputs {
		binary.Write(buf, binary.BigEndian, output.value)
		binary.Write(buf, binary.BigEndian, output.challenge)
	}
}
func (tx Transaction) Verify() bool {
	return true
}
