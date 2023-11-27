// Schnorr Signatures
// Unlike RSA, Schnorr Signatures use Elliptic Curves, allowing schnorr public/private keys and signatures to be much smaller
// Schnorr Signatures are faster and simpler than ECDSA so we choose to use them instead of the built-in ECDSA
package schnorr

// We only use ecdsa for generating keypairs
import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log"
	"math/big"
)

// A Public Key, assumes secp256r1
type PublicKey struct {
	X, Y *big.Int
}

// A Private Key, assumes secp256r1
type PrivateKey struct {
	PublicKey PublicKey
	// in elliptic curve terminology this is actually d, not d*g = D but since go is stupid this has to be uppercase
	D         *big.Int
}

type Signature struct {
	Rx *big.Int
	Ry *big.Int
	// Refer to privatekey comments
	S  *big.Int
}

func NewPrivateKey() (*PrivateKey, error) {
	ecdsa_key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	priv_key := PrivateKey{PublicKey{ecdsa_key.PublicKey.X, ecdsa_key.PublicKey.Y}, ecdsa_key.D}
	return &priv_key, nil
}

// To sign a message we perform the following steps
// Sample a random number r
// Calculate R = rG (Both this and generating r can be done using ecdsa.GenerateKey() (not to be confused with the signing key)
// Calculate challenge e = Hash(R || D || m)
// The signature is s = r + ed where d is the private key
// Return (R, s)
func (private_key PrivateKey) Sign(message []byte) Signature {
	r, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	R := r.PublicKey

	// The hash e commits to the randomness R the public key and the message
	// Committing to the public key prevents potential related-key attacks and also makes multisignature schemes like MuSig2 more secure,
	// see https://github.com/bitcoin/bips/blob/master/bip-0340.mediawiki#description key-prefixing
	hasher := sha256.New()
	hasher.Write(R.X.Bytes())
	hasher.Write(R.Y.Bytes())
	hasher.Write(private_key.PublicKey.X.Bytes())
	hasher.Write(private_key.PublicKey.Y.Bytes())
	hasher.Write(message)

	e := big.NewInt(0)
	e = e.SetBytes(hasher.Sum(nil))

	ed := big.NewInt(0)
	ed = ed.Mul(e, private_key.D)
	ed = ed.Mod(ed, elliptic.P256().Params().N)
	s := big.NewInt(0)
	s = s.Add(r.D, ed)
	s = s.Mod(s, elliptic.P256().Params().N)
	return Signature{R.X, R.Y, s}
}

// Signs a message e that's already generated and committed
func (private_key PrivateKey) SignCommittment(r *big.Int, e *big.Int) Signature {
	ed := big.NewInt(0)
	ed = ed.Mul(e, private_key.D)
	ed = ed.Mod(ed, elliptic.P256().Params().N)
	s := big.NewInt(0)
	s = s.Add(r, ed)
	s = s.Mod(s, elliptic.P256().Params().N)
	Rx, Ry := elliptic.P256().ScalarBaseMult(r.Bytes())
	return Signature{Rx, Ry, s}
}

// To verify we take s = r + ed
// and perform one scalar base multiplication sG = G(r + ed) = sG = R + eD
func (public_key PublicKey) Verify(message []byte, signature Signature) bool {
	if public_key.X == nil || public_key.Y == nil {
		log.Printf("Public key is nil\n");
		return false
	}
	if signature.Rx == nil || signature.Ry == nil || signature.S == nil {
		log.Printf("signature has nil values\n");
		return false
	}
	curve := elliptic.P256()
	Sx, Sy := curve.ScalarBaseMult(signature.S.Bytes())

	hasher := sha256.New()
	hasher.Write(signature.Rx.Bytes())
	hasher.Write(signature.Ry.Bytes())
	hasher.Write(public_key.X.Bytes())
	hasher.Write(public_key.Y.Bytes())
	hasher.Write(message)

	eDx, eDy := curve.ScalarMult(public_key.X, public_key.Y, hasher.Sum(nil))

	if !curve.IsOnCurve(signature.Rx, signature.Ry) || !curve.IsOnCurve(eDx, eDy) {
		return false
	}
	x, y := curve.Add(signature.Rx, signature.Ry, eDx, eDy)
	return x.Cmp(Sx) == 0 && y.Cmp(Sy) == 0
}

func (public_key PublicKey) Equal(other PublicKey) bool {
	return public_key.X.Cmp(other.X) == 0 && public_key.Y.Cmp(other.Y) == 0
}

func (public_key *PublicKey) ToAddress() string {
	bytes := make([]byte, 64)
	copy(bytes[:32], public_key.X.Bytes())
	copy(bytes[32:], public_key.Y.Bytes())
	return base64.RawStdEncoding.EncodeToString(bytes)
}

func PublicKeyFromAddress(address string) (*PublicKey, error) {
	if base64.RawStdEncoding.DecodedLen(len(address)) != 64 {
		return nil, errors.New("Expected 64-byte decoded address")
	}
	bytes, err := base64.RawStdEncoding.DecodeString(address)
	if err != nil {
		return nil, err
	}
	X := big.NewInt(0).SetBytes(bytes[0:32])
	Y := big.NewInt(0).SetBytes(bytes[32:])
	public_key := PublicKey{X, Y}
	return &public_key, nil
}
