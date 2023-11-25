package blockchain

import (
	"blockchain/schnorr"
	"crypto/rand"
	"math/big"
	"testing"
)

func create_fake_utxo(public_key schnorr.PublicKey, amount uint64) (UTXO, Output) {
	txout := [32]byte{}
	rand.Reader.Read(txout[:])
	utxo := UTXO{txout, 0}
	output := Output{amount, public_key}
	return utxo, output
}

func TestTransactionVerification(t *testing.T) {
	priv_key, _ := schnorr.NewPrivateKey()
	public_key := priv_key.PublicKey

	utxo, output := create_fake_utxo(public_key, 10)

	transaction := Transaction{[]UTXO{utxo}, []schnorr.Signature{}, []Output{}}

	utxo_set := make(map[UTXO]Output)
	utxo_set[utxo] = output

	// We haven't signed the transaction yet so it should fail!
	if transaction.Verify(utxo_set, false, 0) {
		t.Errorf("Error, transaction validation should fail but succeeded")
	}

	transaction.Proofs = append(transaction.Proofs, priv_key.Sign(transaction.GenerateCommitment(0)))
	if !transaction.Verify(utxo_set, false, 0) {
		t.Errorf("Error, transaction validation should succeed")
	}
	// Try creating more than we spend
	transaction.Outputs = append(transaction.Outputs, Output{1000, public_key})
	transaction.Proofs[0] = priv_key.Sign(transaction.GenerateCommitment(0))
	if transaction.Verify(utxo_set, false, 0) {
		t.Errorf("Error, transaction validation should fail but succeeded")
	}
	transaction.Outputs[0].Value = 10
	transaction.Proofs[0] = priv_key.Sign(transaction.GenerateCommitment(0))
	if !transaction.Verify(utxo_set, false, 0) {
		t.Errorf("Error, transaction validation should succeed")
	}
	transaction.Outputs = []Output{}

	// Malleate signature
	transaction.Proofs[0].Rx = transaction.Proofs[0].Rx.Add(transaction.Proofs[0].Rx, big.NewInt(1))
	if transaction.Verify(utxo_set, false, 0) {
		t.Errorf("Error, transaction validation should fail but succeeded")
	}

	//Attempt a double-spend
	transaction.Inputs = append(transaction.Inputs, transaction.Inputs[0])
	if transaction.Verify(utxo_set, false, 0) {
		t.Errorf("Error, transaction validation should fail but succeeded")
	}

}
