// Schnorr Signatures
// Unlike RSA, Schnorr Signatures use Elliptic Curves, allowing schnorr public/private keys and signatures to be much smaller
// Schnorr Signatures are faster and simpler than ECDSA so we choose to use them instead of the built-in ECDSA
package schnorr

// We only use ecdsa for generating keypairs
import "crypto/ecdsa"
import "crypto/elliptic"
import "math/big"
import "crypto/rand"
import "crypto/sha256"

// A Public Key, assumes secp256r1
type PublicKey struct {
	X, Y *big.Int
}

// A Private Key, assumes secp256r1
type PrivateKey struct {
	PublicKey PublicKey
	d       *big.Int
}

type Signature struct {
	Rx *big.Int
	Ry *big.Int
	s *big.Int
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
	hasher := sha256.New()
	hasher.Write(R.X.Bytes())
	hasher.Write(R.Y.Bytes())
	hasher.Write(private_key.PublicKey.X.Bytes())
	hasher.Write(private_key.PublicKey.Y.Bytes())
	hasher.Write(message)

	e := big.NewInt(0)
	e = e.SetBytes(hasher.Sum(nil))

	ed := big.NewInt(0)
	ed = ed.Mul(e, private_key.d)
	ed = ed.Mod(ed, elliptic.P256().Params().N)
	s := big.NewInt(0)
	s = s.Add(r.D, ed)
	s = s.Mod(s, elliptic.P256().Params().N)
	return Signature{R.X, R.Y, s}
}

// To verify we take s = r + ed
// and perform one scalar base multiplication sG = G(r + ed) = sG = R + eD
func (public_key PublicKey) Verify(message []byte, signature Signature) bool {
	curve := elliptic.P256()
	Sx, Sy := curve.ScalarBaseMult(signature.s.Bytes())

	hasher := sha256.New()
	hasher.Write(signature.Rx.Bytes())
	hasher.Write(signature.Ry.Bytes())
	hasher.Write(public_key.X.Bytes())
	hasher.Write(public_key.Y.Bytes())
	hasher.Write(message)


	eDx, eDy := curve.ScalarMult(public_key.X, public_key.Y, hasher.Sum(nil))

	x, y := curve.Add(signature.Rx, signature.Ry, eDx, eDy)

	return x.Cmp(Sx) == 0 && y.Cmp(Sy) == 0
}
