package schnorr

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
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

func TestAddressRoundTrip(t *testing.T) {
	private_key, _ := NewPrivateKey()
	public_key := private_key.PublicKey

	public_key_decoded, err := PublicKeyFromAddress(public_key.ToAddress())
	if err != nil {
		t.Error(err)
	}
	if public_key_decoded.X.Cmp(public_key.X) != 0 || public_key_decoded.Y.Cmp(public_key.Y) != 0 {
		t.Errorf("Error decoding, actual (X, Y): (%x, %x), found (%x, %x)\n", public_key.X, public_key.Y, public_key_decoded.X, public_key_decoded.Y)
	}

}

// A simple (insecure) schnorr multisig test.
// Schnorr signatures are linear, so to sign a message together here is how it works
// Signer A and B generate their randomness r_a and r_b and compute the final nonce (r_a + r_b)G = R
// Signer A and B combine their public keys A + B = C
// They compute the public R = (r_a + r_b)G and commit to it in Hash(R || C || msg)
// Signer A now generates a partial signature s_a = r_a + Hash(R || C || msg)a
// Signer B computes signature s_b = r_b + Hash(R || C || msg)b
// Adding the signatures gives:
// s_ab = (r_a + r_b) + Hash(R || C || msg)(a + b)
// Which can be verified using combined public key C as
// s_ab * G = (r_a + r_b)G + Hash(R || C || msg)(a + b)G
// S_ab == R + Hash(R || C || msg)C
func TestMultiSig(t *testing.T) {
	sk1, _ := NewPrivateKey()
	pk1 := sk1.PublicKey
	sk2, _ := NewPrivateKey()
	pk2 := sk2.PublicKey
	t.Log("Public keys,",pk1.ToAddress(), pk2.ToAddress())
	agg_pk_x, agg_pk_y := elliptic.P256().Add(pk1.X, pk1.Y, pk2.X, pk2.Y)
	agg_pk := PublicKey{agg_pk_x, agg_pk_y}
	t.Log("Aggregated public key,", agg_pk.ToAddress())
	message := []byte{1, 2, 3, 4, 5}

	sig1 := sk1.Sign(message)
	sig2 := sk2.Sign(message)
	if agg_pk.Verify(message, sig1) || agg_pk.Verify(message, sig2) {
		t.Error("Signature verification should fail but succeeded")
	}

	// Generate random nonces r1 and r2
	r1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Error(err)
	}
	r2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Error(err)
	}

	agg_Rx, agg_Ry := elliptic.P256().Add(r1.PublicKey.X, r1.PublicKey.Y, r2.PublicKey.X, r2.PublicKey.Y)

	hasher := sha256.New()
	hasher.Write(agg_Rx.Bytes())
	hasher.Write(agg_Ry.Bytes())
	hasher.Write(agg_pk.X.Bytes())
	hasher.Write(agg_pk.Y.Bytes())
	hasher.Write(message)
	e := big.NewInt(0).SetBytes(hasher.Sum(nil))

	sig1 = sk1.SignCommittment(r1.D, e)
	sig2 = sk2.SignCommittment(r2.D, e)
	agg_sig_s := sig1.S.Add(sig1.S, sig2.S)


	agg_sig := Signature{agg_Rx, agg_Ry, agg_sig_s}
	if !agg_pk.Verify(message, agg_sig) {
		t.Error("Verifying signature of aggregated keys failed")
	}
	if agg_pk.Verify([]byte{byte('h')}, agg_sig) {
		t.Error("Signature can be changed!!")
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
