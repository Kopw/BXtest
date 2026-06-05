package hy2

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/InazumaV/V2bX/conf"
	"go.uber.org/zap"
)

func TestGetTLSConfigUsesDynamicCertificateWithoutStaticCertificates(t *testing.T) {
	certPath, keyPath := tempCertificatePaths(t)
	writeCertificateFiles(t, certPath, keyPath, 1)

	node := &Hysteria2node{Logger: zap.NewNop()}
	tlsConfig, err := node.getTLSConfig(&conf.Options{
		CertConfig: &conf.CertConfig{
			CertMode: "file",
			CertFile: certPath,
			KeyFile:  keyPath,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tlsConfig.Certificates) != 0 {
		t.Fatalf("expected no static certificates, got %d", len(tlsConfig.Certificates))
	}
	if tlsConfig.GetCertificate == nil {
		t.Fatal("expected dynamic GetCertificate")
	}

	cert, err := tlsConfig.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certificateSerial(t, cert); got != 1 {
		t.Fatalf("initial certificate serial = %d, want 1", got)
	}

	writeCertificateFiles(t, certPath, keyPath, 2)
	cert, err = tlsConfig.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certificateSerial(t, cert); got != 2 {
		t.Fatalf("reloaded certificate serial = %d, want 2", got)
	}
}

func TestDynamicCertificateLoaderFallsBackToCachedCertificate(t *testing.T) {
	certPath, keyPath := tempCertificatePaths(t)
	writeCertificateFiles(t, certPath, keyPath, 1)

	loader, err := newDynamicCertificateLoader(certPath, keyPath, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	writeCertificateFiles(t, certPath, keyPath, 2)
	cert, err := loader.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certificateSerial(t, cert); got != 2 {
		t.Fatalf("reloaded certificate serial = %d, want 2", got)
	}

	if err := os.WriteFile(certPath, []byte("broken certificate"), 0644); err != nil {
		t.Fatal(err)
	}
	cert, err = loader.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := certificateSerial(t, cert); got != 2 {
		t.Fatalf("fallback certificate serial = %d, want cached serial 2", got)
	}
}

func TestDynamicCertificateLoaderRequiresInitialValidCertificate(t *testing.T) {
	dir := t.TempDir()
	_, err := newDynamicCertificateLoader(filepath.Join(dir, "missing.pem"), filepath.Join(dir, "missing.key"), zap.NewNop())
	if err == nil {
		t.Fatal("expected missing certificate to fail initial load")
	}

	certPath, keyPath := tempCertificatePaths(t)
	certPEM, _ := newTestCertificatePEM(t, 1)
	_, keyPEM := newTestCertificatePEM(t, 2)
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = newDynamicCertificateLoader(certPath, keyPath, zap.NewNop())
	if err == nil {
		t.Fatal("expected mismatched certificate and key to fail initial load")
	}
}

func TestTLSWithoutSNISkipsGetCertificateWhenStaticCertificatesExist(t *testing.T) {
	oldCert := newTestCertificate(t, 1)
	newCert := newTestCertificate(t, 2)
	called := 0
	config := &tls.Config{
		Certificates: []tls.Certificate{oldCert},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			called++
			return &newCert, nil
		},
	}

	if got := presentedSerialWithoutSNI(t, config); got != 1 {
		t.Fatalf("presented certificate serial = %d, want static serial 1", got)
	}
	if called != 0 {
		t.Fatalf("GetCertificate called %d times, want 0", called)
	}
}

func TestTLSWithoutSNIUsesGetCertificateWhenStaticCertificatesAreEmpty(t *testing.T) {
	newCert := newTestCertificate(t, 2)
	called := 0
	config := &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			called++
			return &newCert, nil
		},
	}

	if got := presentedSerialWithoutSNI(t, config); got != 2 {
		t.Fatalf("presented certificate serial = %d, want dynamic serial 2", got)
	}
	if called == 0 {
		t.Fatal("expected GetCertificate to be called")
	}
}

func tempCertificatePaths(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "cert.pem"), filepath.Join(dir, "cert.key")
}

func writeCertificateFiles(t *testing.T, certPath string, keyPath string, serial int64) {
	t.Helper()
	certPEM, keyPEM := newTestCertificatePEM(t, serial)
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
}

func newTestCertificate(t *testing.T, serial int64) tls.Certificate {
	t.Helper()
	certPEM, keyPEM := newTestCertificatePEM(t, serial)
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func newTestCertificatePEM(t *testing.T, serial int64) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("test-%d", serial),
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func certificateSerial(t *testing.T, cert *tls.Certificate) int64 {
	t.Helper()
	if cert == nil {
		t.Fatal("certificate is nil")
	}
	if len(cert.Certificate) == 0 {
		t.Fatal("certificate chain is empty")
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	return leaf.SerialNumber.Int64()
}

func presentedSerialWithoutSNI(t *testing.T, serverConfig *tls.Config) int64 {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	serverConn := tls.Server(serverSide, serverConfig)
	clientConn := tls.Client(clientSide, &tls.Config{InsecureSkipVerify: true})
	defer serverConn.Close()
	defer clientConn.Close()

	errCh := make(chan error, 2)
	go func() {
		errCh <- serverConn.Handshake()
	}()
	go func() {
		errCh <- clientConn.Handshake()
	}()
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
	state := clientConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		t.Fatal("client did not receive a certificate")
	}
	return state.PeerCertificates[0].SerialNumber.Int64()
}
