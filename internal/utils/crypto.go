package utils

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/authelia/authelia/v4/internal/configuration/schema"
	"github.com/authelia/authelia/v4/internal/logging"
)

// PEMBlockType represent an enum of the existing PEM block types.
type PEMBlockType int

const (
	// Certificate block type.
	Certificate PEMBlockType = iota
	// PrivateKey block type.
	PrivateKey
)

// GenerateCertificate generate a certificate given a private key. RSA, Ed25519 and ECDSA are officially supported.
func GenerateCertificate(privateKeyBuilder PrivateKeyBuilder, hosts []string, validFrom time.Time, validFor time.Duration, isCA bool) ([]byte, []byte, error) {
	privateKey, err := privateKeyBuilder.Build()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to build private key: %w", err)
	}

	notBefore := validFrom
	notAfter := validFrom.Add(validFor)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	certDERBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(privateKey), privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %v", err)
	}

	certPEMBytes, err := ConvertDERToPEM(certDERBytes, Certificate)
	if err != nil {
		return nil, nil, fmt.Errorf("faile to convert certificate in DER format into PEM: %v", err)
	}

	keyDERBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %v", err)
	}

	keyPEMBytes, err := ConvertDERToPEM(keyDERBytes, PrivateKey)
	if err != nil {
		return nil, nil, fmt.Errorf("faile to convert certificate in DER format into PEM: %v", err)
	}

	return certPEMBytes, keyPEMBytes, nil
}

// ConvertDERToPEM convert certificate in DER format into PEM format.
func ConvertDERToPEM(der []byte, blockType PEMBlockType) ([]byte, error) {
	var buf bytes.Buffer

	var blockTypeStr string

	switch blockType {
	case Certificate:
		blockTypeStr = "CERTIFICATE"
	case PrivateKey:
		blockTypeStr = "PRIVATE KEY"
	default:
		return nil, fmt.Errorf("unknown PEM block type %d", blockType)
	}

	if err := pem.Encode(&buf, &pem.Block{Type: blockTypeStr, Bytes: der}); err != nil {
		return nil, fmt.Errorf("failed to encode DER data into PEM: %v", err)
	}

	return buf.Bytes(), nil
}

func publicKey(privateKey interface{}) interface{} {
	switch k := privateKey.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}

// PrivateKeyBuilder interface for a private key builder.
type PrivateKeyBuilder interface {
	Build() (interface{}, error)
}

// RSAKeyBuilder builder of RSA private key.
type RSAKeyBuilder struct {
	keySizeInBits int
}

// WithKeySize configure the key size to use with RSA.
func (rkb RSAKeyBuilder) WithKeySize(bits int) RSAKeyBuilder {
	rkb.keySizeInBits = bits
	return rkb
}

// Build a RSA private key.
func (rkb RSAKeyBuilder) Build() (interface{}, error) {
	return rsa.GenerateKey(rand.Reader, rkb.keySizeInBits)
}

// Ed25519KeyBuilder builder of Ed25519 private key.
type Ed25519KeyBuilder struct{}

// Build an Ed25519 private key.
func (ekb Ed25519KeyBuilder) Build() (interface{}, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	return priv, err
}

// ECDSAKeyBuilder builder of ECDSA private key.
type ECDSAKeyBuilder struct {
	curve elliptic.Curve
}

// WithCurve configure the curve to use for the ECDSA private key.
func (ekb ECDSAKeyBuilder) WithCurve(curve elliptic.Curve) ECDSAKeyBuilder {
	ekb.curve = curve
	return ekb
}

// Build an ECDSA private key.
func (ekb ECDSAKeyBuilder) Build() (interface{}, error) {
	return ecdsa.GenerateKey(ekb.curve, rand.Reader)
}

// ParseX509FromPEM parses PEM bytes and returns a PKCS key.
func ParseX509FromPEM(data []byte) (key interface{}, err error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	switch block.Type {
	case BlockTypeRSAPrivateKey:
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case BlockTypeECDSAPrivateKey:
		key, err = x509.ParseECPrivateKey(block.Bytes)
	case BlockTypePKCS8PrivateKey:
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case BlockTypeRSAPublicKey:
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
	case BlockTypePKIXPublicKey:
		key, err = x509.ParsePKIXPublicKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unknown block type: %s", block.Type)
	}

	if err != nil {
		return nil, err
	}

	return key, nil
}

// ParseX509CertificateFromPEM parses PEM bytes and returns a x509.Certificate.
func ParseX509CertificateFromPEM(data []byte) (cert *x509.Certificate, err error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	return x509.ParseCertificate(block.Bytes)
}

// NewTLSConfig generates a tls.Config from a schema.TLSConfig and a x509.CertPool.
func NewTLSConfig(config *schema.TLSConfig, defaultMinVersion uint16, certPool *x509.CertPool) (tlsConfig *tls.Config) {
	minVersion, err := TLSStringToTLSConfigVersion(config.MinimumVersion)
	if err != nil {
		minVersion = defaultMinVersion
	}

	return &tls.Config{
		ServerName:         config.ServerName,
		InsecureSkipVerify: config.SkipVerify, //nolint:gosec // Informed choice by user. Off by default.
		MinVersion:         minVersion,
		RootCAs:            certPool,
	}
}

// NewX509CertPool generates a x509.CertPool from the system PKI and the directory specified.
func NewX509CertPool(directory string) (certPool *x509.CertPool, warnings []error, errors []error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		warnings = append(warnings, fmt.Errorf("could not load system certificate pool which may result in untrusted certificate issues: %v", err))
		certPool = x509.NewCertPool()
	}

	logger := logging.Logger()

	logger.Tracef("Starting scan of directory %s for certificates", directory)

	if directory != "" {
		certsFileInfo, err := os.ReadDir(directory)
		if err != nil {
			errors = append(errors, fmt.Errorf("could not read certificates from directory %v", err))
		} else {
			for _, certFileInfo := range certsFileInfo {
				nameLower := strings.ToLower(certFileInfo.Name())

				if !certFileInfo.IsDir() && (strings.HasSuffix(nameLower, ".cer") || strings.HasSuffix(nameLower, ".crt") || strings.HasSuffix(nameLower, ".pem")) {
					certPath := filepath.Join(directory, certFileInfo.Name())

					logger.Tracef("Found possible cert %s, attempting to add it to the pool", certPath)

					certBytes, err := os.ReadFile(certPath)
					if err != nil {
						errors = append(errors, fmt.Errorf("could not read certificate %v", err))
					} else if ok := certPool.AppendCertsFromPEM(certBytes); !ok {
						errors = append(errors, fmt.Errorf("could not import certificate %s", certFileInfo.Name()))
					}
				}
			}
		}
	}

	logger.Tracef("Finished scan of directory %s for certificates", directory)

	return certPool, warnings, errors
}

// TLSStringToTLSConfigVersion returns a go crypto/tls version for a tls.Config based on string input.
func TLSStringToTLSConfigVersion(input string) (version uint16, err error) {
	switch strings.ToUpper(input) {
	case "TLS1.3", TLS13:
		return tls.VersionTLS13, nil
	case "TLS1.2", TLS12:
		return tls.VersionTLS12, nil
	case "TLS1.1", TLS11:
		return tls.VersionTLS11, nil
	case "TLS1.0", TLS10:
		return tls.VersionTLS10, nil
	}

	return 0, ErrTLSVersionNotSupported
}

// WriteCertificateBytesToPEM writes a certificate/csr to a file in the PEM format.
func WriteCertificateBytesToPEM(cert []byte, path string, csr bool) (err error) {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", path, err)
	}

	defer func() {
		err := out.Close()
		if err != nil {
			fmt.Printf("Error closing %s: %v\n", path, err)
			os.Exit(1)
		}
	}()

	blockType := BlockTypeCertificate
	if csr {
		blockType = BlockTypeCertificateRequest
	}

	return pem.Encode(out, &pem.Block{Bytes: cert, Type: blockType})
}

// WriteKeyToPEM writes a key that can be encoded as a PEM to a file in the PEM format.
func WriteKeyToPEM(key interface{}, path string, pkcs8 bool) (err error) {
	pemBlock, err := PEMBlockFromX509Key(key, pkcs8)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", path, err)
	}

	defer func() {
		err := out.Close()
		if err != nil {
			fmt.Printf("Error closing %s: %v\n", path, err)
			os.Exit(1)
		}
	}()

	return pem.Encode(out, pemBlock)
}

// PEMBlockFromX509Key turns a PublicKey or PrivateKey into a pem.Block.
func PEMBlockFromX509Key(key interface{}, pkcs8 bool) (pemBlock *pem.Block, err error) {
	var (
		data      []byte
		blockType string
	)

	switch k := key.(type) {
	case *rsa.PrivateKey:
		if pkcs8 {
			blockType = BlockTypePKCS8PrivateKey
			data, err = x509.MarshalPKCS8PrivateKey(key)

			break
		}

		blockType = BlockTypeRSAPrivateKey
		data = x509.MarshalPKCS1PrivateKey(k)
	case *ecdsa.PrivateKey:
		if pkcs8 {
			blockType = BlockTypePKCS8PrivateKey
			data, err = x509.MarshalPKCS8PrivateKey(key)

			break
		}

		blockType = BlockTypeECDSAPrivateKey
		data, err = x509.MarshalECPrivateKey(k)
	case ed25519.PrivateKey:
		blockType = BlockTypePKCS8PrivateKey
		data, err = x509.MarshalPKCS8PrivateKey(k)
	case *rsa.PublicKey:
		if pkcs8 {
			blockType = BlockTypePKIXPublicKey
			data, err = x509.MarshalPKIXPublicKey(key)

			break
		}

		blockType = BlockTypeRSAPublicKey
		data = x509.MarshalPKCS1PublicKey(k)
	case *ecdsa.PublicKey, ed25519.PublicKey:
		blockType = BlockTypePKIXPublicKey
		data, err = x509.MarshalPKIXPublicKey(k)
	default:
		err = fmt.Errorf("failed to match key type: %T", k)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal key: %w", err)
	}

	return &pem.Block{
		Type:  blockType,
		Bytes: data,
	}, nil
}

// KeySigAlgorithmFromString returns a x509.PublicKeyAlgorithm and x509.SignatureAlgorithm given a keyAlgorithm and signatureAlgorithm string.
func KeySigAlgorithmFromString(keyAlgorithm, signatureAlgorithm string) (keyAlg x509.PublicKeyAlgorithm, sigAlg x509.SignatureAlgorithm) {
	keyAlg = PublicKeyAlgorithmFromString(keyAlgorithm)

	if keyAlg == x509.UnknownPublicKeyAlgorithm {
		return x509.UnknownPublicKeyAlgorithm, x509.UnknownSignatureAlgorithm
	}

	switch keyAlg {
	case x509.RSA:
		return keyAlg, RSASignatureAlgorithmFromString(signatureAlgorithm)
	case x509.ECDSA:
		return keyAlg, ECDSASignatureAlgorithmFromString(signatureAlgorithm)
	case x509.Ed25519:
		return keyAlg, x509.PureEd25519
	default:
		return keyAlg, x509.UnknownSignatureAlgorithm
	}
}

// PublicKeyAlgorithmFromString returns a x509.PublicKeyAlgorithm given an appropriate string.
func PublicKeyAlgorithmFromString(algorithm string) (alg x509.PublicKeyAlgorithm) {
	switch algorithm {
	case KeyAlgorithmAltRSA, KeyAlgorithmRSA:
		return x509.RSA
	case KeyAlgorithmAltECDSA, KeyAlgorithmECDSA:
		return x509.ECDSA
	case KeyAlgorithmAlt1Ed25519, KeyAlgorithmEd25519, KeyAlgorithmAlt2Ed25519:
		return x509.Ed25519
	default:
		return x509.UnknownPublicKeyAlgorithm
	}
}

// RSASignatureAlgorithmFromString returns a x509.SignatureAlgorithm for the RSA x509.PublicKeyAlgorithm given an
// algorithm string.
func RSASignatureAlgorithmFromString(algorithm string) (alg x509.SignatureAlgorithm) {
	switch algorithm {
	case HashAlgorithmAltSHA1, HashAlgorithmSHA1:
		return x509.SHA1WithRSA
	case HashAlgorithmAltSHA256, HashAlgorithmSHA256:
		return x509.SHA256WithRSA
	case HashAlgorithmAltSHA384, HashAlgorithmSHA384:
		return x509.SHA384WithRSA
	case HashAlgorithmAltSHA512, HashAlgorithmSHA512:
		return x509.SHA512WithRSA
	default:
		return x509.UnknownSignatureAlgorithm
	}
}

// ECDSASignatureAlgorithmFromString returns a x509.SignatureAlgorithm for the ECDSA x509.PublicKeyAlgorithm given an
// algorithm string.
func ECDSASignatureAlgorithmFromString(algorithm string) (alg x509.SignatureAlgorithm) {
	switch algorithm {
	case HashAlgorithmAltSHA1, HashAlgorithmSHA1:
		return x509.ECDSAWithSHA1
	case HashAlgorithmAltSHA256, HashAlgorithmSHA256:
		return x509.ECDSAWithSHA256
	case HashAlgorithmAltSHA384, HashAlgorithmSHA384:
		return x509.ECDSAWithSHA384
	case HashAlgorithmAltSHA512, HashAlgorithmSHA512:
		return x509.ECDSAWithSHA512
	default:
		return x509.UnknownSignatureAlgorithm
	}
}

// EllipticCurveFromString turns a string into an elliptic.Curve.
func EllipticCurveFromString(curveString string) (curve elliptic.Curve) {
	switch curveString {
	case EllipticCurveAltP224, EllipticCurveP224:
		return elliptic.P224()
	case EllipticCurveAltP256, EllipticCurveP256:
		return elliptic.P256()
	case EllipticCurveAltP384, EllipticCurveP384:
		return elliptic.P384()
	case EllipticCurveAltP521, EllipticCurveP521:
		return elliptic.P521()
	default:
		return nil
	}
}

// PublicKeyFromPrivateKey returns a PublicKey when provided with a PrivateKey.
func PublicKeyFromPrivateKey(privateKey interface{}) (publicKey interface{}) {
	switch k := privateKey.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}