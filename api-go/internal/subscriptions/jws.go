package subscriptions

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

//go:embed resources/AppleRootCA-G3.cer
var appleRootCAG3DER []byte

// JWSVerificationError signals a JWS that failed structural, certificate-chain,
// or signature validation. The message is surfaced in the 401 invalid_transaction
// / invalid_notification response body.
type JWSVerificationError struct {
	Message string
}

func (e *JWSVerificationError) Error() string { return e.Message }

// jwsHeader is the decoded JWS protected header. Apple App Store payloads carry
// the signing certificate chain in x5c (leaf first) and always sign with ES256.
type jwsHeader struct {
	Alg string   `json:"alg"`
	X5c []string `json:"x5c"`
}

// JWSVerifier verifies an Apple StoreKit JWS without any network call to Apple
// (ADR 0010): the x5c chain is validated against the configured trusted root(s),
// then the ES256 signature is verified with the leaf certificate's public key.
type JWSVerifier struct {
	roots *x509.CertPool
	now   func() time.Time
}

// NewJWSVerifier returns a verifier trusting the given root certificate(s). At
// least one root is required. now supplies the current time for certificate
// validity checks (injected for deterministic tests).
func NewJWSVerifier(trustedRoots []*x509.Certificate, now func() time.Time) (*JWSVerifier, error) {
	if len(trustedRoots) == 0 {
		return nil, errors.New("at least one trusted root certificate is required")
	}
	pool := x509.NewCertPool()
	for _, r := range trustedRoots {
		pool.AddCert(r)
	}
	return &JWSVerifier{roots: pool, now: now}, nil
}

// LoadAppleRootCertificates returns the trusted Apple root certificate(s) for
// JWS chain validation. The DER-encoded "Apple Root CA - G3" is embedded as a
// package resource so verification needs no filesystem or network access.
func LoadAppleRootCertificates() ([]*x509.Certificate, error) {
	cert, err := x509.ParseCertificate(appleRootCAG3DER)
	if err != nil {
		return nil, fmt.Errorf("parse embedded Apple root certificate: %w", err)
	}
	return []*x509.Certificate{cert}, nil
}

// VerifyAndDecode validates the compact-serialized JWS and returns its decoded
// JSON payload. Any structural, chain, or signature failure yields a
// *JWSVerificationError.
func (v *JWSVerifier) VerifyAndDecode(signedPayload string) (string, error) {
	if strings.TrimSpace(signedPayload) == "" {
		return "", &JWSVerificationError{Message: "The signed payload is empty."}
	}

	parts := strings.Split(signedPayload, ".")
	if len(parts) != 3 {
		return "", &JWSVerificationError{Message: "The signed payload is not a JWS compact serialization (expected three parts)."}
	}

	header, err := parseJWSHeader(parts[0])
	if err != nil {
		return "", err
	}

	chain, err := parseCertificateChain(header)
	if err != nil {
		return "", err
	}

	if err := v.verifyChainTrust(chain); err != nil {
		return "", err
	}
	if err := verifySignature(chain[0], parts[0], parts[1], parts[2]); err != nil {
		return "", err
	}

	payload, err := base64URLDecode(parts[1])
	if err != nil {
		return "", &JWSVerificationError{Message: "The JWS payload is not valid base64url."}
	}
	return string(payload), nil
}

func parseJWSHeader(encoded string) (jwsHeader, error) {
	raw, err := base64URLDecode(encoded)
	if err != nil {
		return jwsHeader{}, &JWSVerificationError{Message: "The JWS header is not valid base64url."}
	}
	var header jwsHeader
	if err := json.Unmarshal(raw, &header); err != nil {
		return jwsHeader{}, &JWSVerificationError{Message: "The JWS header is not valid JSON."}
	}
	return header, nil
}

func parseCertificateChain(header jwsHeader) ([]*x509.Certificate, error) {
	if len(header.X5c) == 0 {
		return nil, &JWSVerificationError{Message: "The JWS header does not contain an x5c certificate chain."}
	}
	if header.Alg != "ES256" {
		return nil, &JWSVerificationError{Message: fmt.Sprintf("Unsupported JWS algorithm '%s'. Apple App Store payloads use ES256.", header.Alg)}
	}

	chain := make([]*x509.Certificate, 0, len(header.X5c))
	for _, encoded := range header.X5c {
		// x5c entries are standard base64 (not base64url) per RFC 7515.
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, &JWSVerificationError{Message: "An x5c entry is not valid base64."}
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, &JWSVerificationError{Message: "An x5c entry is not a valid X.509 certificate."}
		}
		chain = append(chain, cert)
	}
	return chain, nil
}

// verifyChainTrust builds the leaf -> ... -> root path against the trusted
// roots. Intermediates travel in the x5c header, so they are supplied as the
// intermediate pool. ExtKeyUsageAny is used (no EKU constraint); revocation is
// not checked.
func (v *JWSVerifier) verifyChainTrust(chain []*x509.Certificate) error {
	intermediates := x509.NewCertPool()
	for _, cert := range chain[1:] {
		intermediates.AddCert(cert)
	}

	_, err := chain[0].Verify(x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: intermediates,
		CurrentTime:   v.now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	if err != nil {
		return &JWSVerificationError{Message: fmt.Sprintf("The certificate chain did not validate to a trusted Apple root: %s", err)}
	}
	return nil
}

// verifySignature checks the ES256 signature over "header.payload" using the
// leaf certificate's ECDSA public key. The JWS signature is the fixed-field
// concatenation r||s (RFC 7515 / IEEE P1363), not ASN.1 DER.
func verifySignature(leaf *x509.Certificate, encodedHeader, encodedPayload, encodedSignature string) error {
	pub, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return &JWSVerificationError{Message: "The leaf certificate does not contain an ECDSA public key."}
	}

	sig, err := base64URLDecode(encodedSignature)
	if err != nil {
		return &JWSVerificationError{Message: "The JWS signature is not valid base64url."}
	}

	fieldSize := (pub.Curve.Params().BitSize + 7) / 8
	if len(sig) != 2*fieldSize {
		return &JWSVerificationError{Message: "The JWS signature does not match the payload."}
	}
	r := new(big.Int).SetBytes(sig[:fieldSize])
	s := new(big.Int).SetBytes(sig[fieldSize:])

	digest := sha256.Sum256([]byte(encodedHeader + "." + encodedPayload))
	if !ecdsa.Verify(pub, digest[:], r, s) {
		return &JWSVerificationError{Message: "The JWS signature does not match the payload."}
	}
	return nil
}

func base64URLDecode(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}
