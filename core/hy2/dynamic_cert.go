package hy2

import (
	"crypto/tls"
	"sync"

	"go.uber.org/zap"
)

type dynamicCertificateLoader struct {
	certFile string
	keyFile  string
	logger   *zap.Logger

	access sync.RWMutex
	cert   *tls.Certificate
}

func newDynamicCertificateLoader(certFile string, keyFile string, logger *zap.Logger) (*dynamicCertificateLoader, error) {
	loader := &dynamicCertificateLoader{
		certFile: certFile,
		keyFile:  keyFile,
		logger:   logger,
	}
	if err := loader.reload(); err != nil {
		return nil, err
	}
	return loader, nil
}

func (l *dynamicCertificateLoader) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if err := l.reload(); err != nil {
		l.access.RLock()
		defer l.access.RUnlock()
		if l.cert != nil {
			if l.logger != nil {
				l.logger.Warn("failed to reload TLS certificate, using cached certificate",
					zap.String("cert", l.certFile),
					zap.String("key", l.keyFile),
					zap.Error(err))
			}
			return l.cert, nil
		}
		return nil, err
	}
	l.access.RLock()
	defer l.access.RUnlock()
	return l.cert, nil
}

func (l *dynamicCertificateLoader) reload() error {
	cert, err := tls.LoadX509KeyPair(l.certFile, l.keyFile)
	if err != nil {
		return err
	}
	l.access.Lock()
	defer l.access.Unlock()
	l.cert = &cert
	return nil
}
