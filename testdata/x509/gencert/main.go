// Run from the module root: go run ./testdata/x509/gencert > testdata/x509/chain.in
// Generates the x509 fixtures: a CA and a leaf, with Ed25519 keys from fixed
// seeds, so the output is the same on every run.
package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"os"
	"time"
)

func key(b byte) ed25519.PrivateKey {
	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = b
	}
	return ed25519.NewKeyFromSeed(seed)
}

func main() {
	caKey, leafKey := key(1), key(2)
	ca := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Devtooltui Test CA", Organization: []string{"Devtooltui"}, Country: []string{"CO"}},
		NotBefore:             time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:              time.Date(2036, 1, 1, 0, 0, 0, 0, time.UTC),
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(nil, ca, ca, caKey.Public(), caKey)
	if err != nil {
		panic(err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	u, _ := url.Parse("https://example.com/id")
	leaf := &x509.Certificate{
		SerialNumber:   new(big.Int).SetBytes([]byte{0x0a, 0x1b, 0x2c, 0x3d, 0x4e, 0x5f}),
		Subject:        pkix.Name{CommonName: "example.com", Organization: []string{"Example Ltd"}},
		NotBefore:      time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		NotAfter:       time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		DNSNames:       []string{"example.com", "www.example.com"},
		IPAddresses:    []net.IP{net.ParseIP("192.0.2.10")},
		EmailAddresses: []string{"admin@example.com"},
		URIs:           []*url.URL{u},
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(nil, leaf, caCert, leafKey.Public(), caKey)
	if err != nil {
		panic(err)
	}
	_ = pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	_ = pem.Encode(os.Stdout, &pem.Block{Type: "CERTIFICATE", Bytes: caDER})
}
