package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"
)

func TestGenerateSelfSignedCert_Success(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	if cert == nil {
		t.Fatal("GenerateSelfSignedCert() returned nil certificate")
	}

	if len(cert.CertPEM) == 0 {
		t.Error("Certificate PEM is empty")
	}

	if len(cert.KeyPEM) == 0 {
		t.Error("Private key PEM is empty")
	}
}

func TestGenerateSelfSignedCert_ValidityPeriod(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	// Parse the certificate
	block, _ := pem.Decode(cert.CertPEM)
	if block == nil {
		t.Fatal("Failed to decode certificate PEM")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Check validity period is approximately 24 hours
	now := time.Now()
	expectedExpiry := now.Add(24 * time.Hour)

	// Allow 1 minute tolerance for test execution time
	tolerance := 1 * time.Minute

	if x509Cert.NotBefore.After(now.Add(tolerance)) {
		t.Errorf("Certificate NotBefore is in the future: %v", x509Cert.NotBefore)
	}

	if x509Cert.NotBefore.Before(now.Add(-tolerance)) {
		t.Errorf("Certificate NotBefore is too far in the past: %v", x509Cert.NotBefore)
	}

	if x509Cert.NotAfter.After(expectedExpiry.Add(tolerance)) {
		t.Errorf("Certificate NotAfter is too far in the future: %v (expected ~%v)", x509Cert.NotAfter, expectedExpiry)
	}

	if x509Cert.NotAfter.Before(expectedExpiry.Add(-tolerance)) {
		t.Errorf("Certificate NotAfter is too soon: %v (expected ~%v)", x509Cert.NotAfter, expectedExpiry)
	}
}

func TestGenerateSelfSignedCert_LocalhostNames(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	// Parse the certificate
	block, _ := pem.Decode(cert.CertPEM)
	if block == nil {
		t.Fatal("Failed to decode certificate PEM")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Check DNS names
	expectedDNS := []string{"localhost"}
	if len(x509Cert.DNSNames) != len(expectedDNS) {
		t.Errorf("Expected %d DNS names, got %d", len(expectedDNS), len(x509Cert.DNSNames))
	}

	for _, expected := range expectedDNS {
		found := false
		for _, actual := range x509Cert.DNSNames {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected DNS name %s not found in certificate", expected)
		}
	}

	// Check IP addresses
	expectedIPs := []string{"127.0.0.1", "::1"}
	if len(x509Cert.IPAddresses) != len(expectedIPs) {
		t.Errorf("Expected %d IP addresses, got %d", len(expectedIPs), len(x509Cert.IPAddresses))
	}

	for _, expectedIP := range expectedIPs {
		found := false
		expected := net.ParseIP(expectedIP)
		for _, actual := range x509Cert.IPAddresses {
			if actual.Equal(expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected IP address %s not found in certificate", expectedIP)
		}
	}
}

func TestGenerateSelfSignedCert_PEMEncoding(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	// Verify certificate PEM
	certBlock, _ := pem.Decode(cert.CertPEM)
	if certBlock == nil {
		t.Fatal("Failed to decode certificate PEM")
	}
	if certBlock.Type != "CERTIFICATE" {
		t.Errorf("Certificate PEM type = %s, want CERTIFICATE", certBlock.Type)
	}

	// Verify key PEM
	keyBlock, _ := pem.Decode(cert.KeyPEM)
	if keyBlock == nil {
		t.Fatal("Failed to decode key PEM")
	}
	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("Key PEM type = %s, want EC PRIVATE KEY", keyBlock.Type)
	}
}

func TestGenerateSelfSignedCert_CanLoadAsTLSCertificate(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	// Try to load as TLS certificate
	_, err = tls.X509KeyPair(cert.CertPEM, cert.KeyPEM)
	if err != nil {
		t.Errorf("Failed to load certificate as TLS certificate: %v", err)
	}
}

func TestGenerateSelfSignedCert_KeyUsage(t *testing.T) {
	cert, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	// Parse the certificate
	block, _ := pem.Decode(cert.CertPEM)
	if block == nil {
		t.Fatal("Failed to decode certificate PEM")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Check key usage
	expectedKeyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if x509Cert.KeyUsage != expectedKeyUsage {
		t.Errorf("KeyUsage = %v, want %v", x509Cert.KeyUsage, expectedKeyUsage)
	}

	// Check extended key usage
	if len(x509Cert.ExtKeyUsage) != 1 {
		t.Errorf("Expected 1 ExtKeyUsage, got %d", len(x509Cert.ExtKeyUsage))
	}
	if len(x509Cert.ExtKeyUsage) > 0 && x509Cert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Errorf("ExtKeyUsage[0] = %v, want %v", x509Cert.ExtKeyUsage[0], x509.ExtKeyUsageServerAuth)
	}
}

func TestGenerateSelfSignedCert_Uniqueness(t *testing.T) {
	// Generate multiple certificates and ensure they're unique
	cert1, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	cert2, err := GenerateSelfSignedCert()
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() failed: %v", err)
	}

	// Certificates should be different
	if string(cert1.CertPEM) == string(cert2.CertPEM) {
		t.Error("Generated certificates are identical, expected unique certificates")
	}

	if string(cert1.KeyPEM) == string(cert2.KeyPEM) {
		t.Error("Generated keys are identical, expected unique keys")
	}
}

// Made with Bob
