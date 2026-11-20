package encoders

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec // SHA-1 fingerprint, shown because tools still use it, not for security
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/Judamaro/devtooltui-golang/internal/tool"
	"github.com/Judamaro/devtooltui-golang/internal/tools/converters"
)

// X509 decodes X.509 certificates: PEM (one or a chain) or DER in Base64 or
// hex. Now pins the clock in tests; when nil, time.Now is used.
type X509 struct {
	Now func() time.Time
}

func (X509) Spec() tool.Spec {
	return tool.Spec{
		ID:          "x509",
		Name:        "X.509 certificate",
		Category:    tool.Encoders,
		Description: "Decode X.509 certificates: subject, SAN, validity, key and fingerprints",
		Keywords:    []string{"certificate", "pem", "der", "tls", "ssl", "crt", "fingerprint"},
		Inputs:      []tool.InputSpec{{Key: "input", Label: "PEM, or DER in Base64 or hex", Multiline: true}},
		Output:      tool.PlainText,
		Live:        true,
	}
}

func (x X509) Run(ctx context.Context, req tool.Request) (tool.Result, error) {
	if err := ctx.Err(); err != nil {
		return tool.Result{}, err
	}
	in := req.Inputs["input"]
	if strings.TrimSpace(in) == "" {
		return tool.Result{}, nil
	}
	certs, err := parseCertificates(in)
	if err != nil {
		return tool.Result{}, err
	}
	now := time.Now
	if x.Now != nil {
		now = x.Now
	}
	t := now()
	var blocks []string
	for i, c := range certs {
		var b strings.Builder
		if len(certs) > 1 {
			fmt.Fprintf(&b, "Certificate %d of %d\n", i+1, len(certs))
		}
		describeCert(&b, c, t)
		blocks = append(blocks, strings.TrimRight(b.String(), "\n"))
	}
	return tool.Result{
		Text: strings.Join(blocks, "\n\n"),
		Meta: []tool.KV{
			{Key: "Certificates", Value: fmt.Sprint(len(certs))},
			{Key: "Expires", Value: converters.Relative(certs[0].NotAfter.Sub(t))},
		},
	}, nil
}

// parseCertificates reads every PEM CERTIFICATE block, or else the whole
// input as DER in Base64 or hex.
func parseCertificates(in string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := []byte(in)
	for {
		start := len(in) - len(rest)
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		// The block began at the "-----BEGIN" line after start.
		at := start + strings.Index(in[start:], "-----BEGIN")
		line, _ := tool.Position(in, at)
		if block.Type != "CERTIFICATE" {
			return nil, &tool.InputError{Input: "input", Line: line, Col: 1,
				Msg: fmt.Sprintf("a %s block, not a CERTIFICATE (private keys are not decoded here)", block.Type)}
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, &tool.InputError{Input: "input", Line: line, Col: 1, Msg: "invalid certificate: " + err.Error()}
		}
		certs = append(certs, c)
	}
	if len(certs) > 0 {
		return certs, nil
	}
	if strings.Contains(in, "-----BEGIN") {
		line, _ := tool.Position(in, strings.Index(in, "-----BEGIN"))
		return nil, &tool.InputError{Input: "input", Line: line, Col: 1, Msg: "broken PEM block (check the BEGIN/END lines and the Base64)"}
	}
	const notCert = "not a certificate: expected PEM, or DER in Base64 or hex"
	der, err := derBytes(in)
	if err != nil {
		return nil, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: notCert}
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, &tool.InputError{Input: "input", Line: 1, Col: 1, Msg: notCert + " (" + err.Error() + ")"}
	}
	return []*x509.Certificate{c}, nil
}

func derBytes(in string) ([]byte, error) {
	clean := strings.Join(strings.Fields(in), "")
	if b, err := hex.DecodeString(strings.ReplaceAll(clean, ":", "")); err == nil {
		return b, nil
	}
	for _, urlSafe := range []bool{false, true} {
		if b, err := decodeBase64(clean, urlSafe); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("neither hex nor Base64")
}

func describeCert(b *strings.Builder, c *x509.Certificate, now time.Time) {
	row := func(label, value string) {
		if value != "" {
			fmt.Fprintf(b, "%-12s %s\n", label+":", value)
		}
	}
	row("Subject", c.Subject.String())
	row("Issuer", c.Issuer.String())
	row("Serial", colonHex(c.SerialNumber.Bytes()))
	const layout = "2006-01-02 15:04 UTC"
	status := "valid, expires " + converters.Relative(c.NotAfter.Sub(now))
	switch {
	case now.Before(c.NotBefore):
		status = "NOT YET VALID, starts " + converters.Relative(c.NotBefore.Sub(now))
	case now.After(c.NotAfter):
		status = "EXPIRED " + converters.Relative(c.NotAfter.Sub(now))
	}
	row("Valid", fmt.Sprintf("%s → %s (%s)", c.NotBefore.UTC().Format(layout), c.NotAfter.UTC().Format(layout), status))
	row("SAN", sans(c))
	row("Key", keyDescription(c))
	row("Signature", c.SignatureAlgorithm.String())
	if c.BasicConstraintsValid && c.IsCA {
		ca := "yes"
		if c.MaxPathLen > 0 || c.MaxPathLenZero {
			ca += fmt.Sprintf(" (path length %d)", c.MaxPathLen)
		}
		row("CA", ca)
	} else {
		row("CA", "no")
	}
	row("Key usage", keyUsages(c.KeyUsage))
	row("Ext. usage", extUsages(c.ExtKeyUsage))
	sum256 := sha256.Sum256(c.Raw)
	sum1 := sha1.Sum(c.Raw) //nolint:gosec // see import
	row("SHA-256", colonHex(sum256[:]))
	row("SHA-1", colonHex(sum1[:]))
}

func colonHex(b []byte) string {
	parts := make([]string, len(b))
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02X", x)
	}
	return strings.Join(parts, ":")
}

func sans(c *x509.Certificate) string {
	var parts []string
	add := func(kind string, vals []string) {
		if len(vals) > 0 {
			parts = append(parts, kind+": "+strings.Join(vals, ", "))
		}
	}
	add("DNS", c.DNSNames)
	ips := make([]string, len(c.IPAddresses))
	for i, ip := range c.IPAddresses {
		ips[i] = ip.String()
	}
	add("IP", ips)
	add("Email", c.EmailAddresses)
	uris := make([]string, len(c.URIs))
	for i, u := range c.URIs {
		uris[i] = u.String()
	}
	add("URI", uris)
	return strings.Join(parts, "; ")
}

func keyDescription(c *x509.Certificate) string {
	switch k := c.PublicKey.(type) {
	case *rsa.PublicKey:
		return fmt.Sprintf("RSA %d bits", k.N.BitLen())
	case *ecdsa.PublicKey:
		return "ECDSA " + k.Curve.Params().Name
	case ed25519.PublicKey:
		return "Ed25519"
	}
	return c.PublicKeyAlgorithm.String()
}

var keyUsageNames = []struct {
	bit  x509.KeyUsage
	name string
}{
	{x509.KeyUsageDigitalSignature, "Digital Signature"},
	{x509.KeyUsageContentCommitment, "Content Commitment"},
	{x509.KeyUsageKeyEncipherment, "Key Encipherment"},
	{x509.KeyUsageDataEncipherment, "Data Encipherment"},
	{x509.KeyUsageKeyAgreement, "Key Agreement"},
	{x509.KeyUsageCertSign, "Certificate Sign"},
	{x509.KeyUsageCRLSign, "CRL Sign"},
	{x509.KeyUsageEncipherOnly, "Encipher Only"},
	{x509.KeyUsageDecipherOnly, "Decipher Only"},
}

func keyUsages(u x509.KeyUsage) string {
	var names []string
	for _, k := range keyUsageNames {
		if u&k.bit != 0 {
			names = append(names, k.name)
		}
	}
	return strings.Join(names, ", ")
}

var extUsageNames = map[x509.ExtKeyUsage]string{
	x509.ExtKeyUsageAny:             "Any",
	x509.ExtKeyUsageServerAuth:      "Server Auth",
	x509.ExtKeyUsageClientAuth:      "Client Auth",
	x509.ExtKeyUsageCodeSigning:     "Code Signing",
	x509.ExtKeyUsageEmailProtection: "Email Protection",
	x509.ExtKeyUsageTimeStamping:    "Time Stamping",
	x509.ExtKeyUsageOCSPSigning:     "OCSP Signing",
}

func extUsages(us []x509.ExtKeyUsage) string {
	names := make([]string, 0, len(us))
	for _, u := range us {
		if n, ok := extUsageNames[u]; ok {
			names = append(names, n)
		} else {
			names = append(names, fmt.Sprintf("other (%d)", u))
		}
	}
	return strings.Join(names, ", ")
}

// Detect recognises a PEM certificate.
func (X509) Detect(sample string) float64 {
	if strings.Contains(sample, "-----BEGIN CERTIFICATE-----") {
		return 0.95
	}
	return 0
}
