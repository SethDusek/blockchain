package schnorr

import (
	"fmt"
	"testing"
)

func TestSigning(t *testing.T) {
	private_key, _ := NewPrivateKey()
	private_key2, _ := NewPrivateKey()
	public_key1, public_key2 := private_key.PublicKey, private_key2.PublicKey
	message := []byte("Hello World!")
	signature := private_key.Sign(message)
	signature2 := private_key2.Sign(message)

	if !public_key1.Verify(message, signature) {
		t.Errorf("Error, verification failed for public key 1")
	}
	if !public_key2.Verify(message, signature2) {
		t.Errorf("Error, verification failed for public key 2")
	}

	// Test that a signature produced by one public key is not valid for the other
	if public_key1.Verify(message, signature2) {
		t.Errorf("Error, verification succeeded for public key 1 when it should fail")
	}
	if public_key2.Verify(message, signature) {
		t.Errorf("Error, verification succeeded for public key 2 when it should fail")
	}

	// Try malleating the message
	message[0] = byte('F')
	if public_key1.Verify(message, signature) {
		t.Errorf("Error, verification succeeded for public key 1 when it should fail")
	}
	if public_key2.Verify(message, signature2) {
		t.Errorf("Error, verification succeeded for public key 2 when it should fail")
	}
}

/*
func BenchmarkSchnorr(b *testing.B) {
	private_key, _ := NewPrivateKey()
	inputs := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		for j := 0; j <= i; j++ {
			inputs[i] = append(inputs[i], byte(j))
		}
	}
	signatures := make([]Signature, 1000)
	for i := 0; i < 1000; i++ {
		b.Run(fmt.Sprintf("Signing message of size %d", len(inputs[i])), func(b *testing.B) {
			signatures[i] = private_key.Sign(inputs[i])
		})
	}
	for i := 0; i < 1000; i++ {
		b.Run(fmt.Sprintf("Verifying message of size %d", len(inputs[i])), func(b *testing.B) {
			if !private_key.PublicKey.Verify(inputs[i], signatures[i]) {
				b.Error("Verification failed")
			}
		})
	}
}
*/
