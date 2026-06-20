package server

import (
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigUDPForwardingRedundancyMultiplierValidation(t *testing.T) {
	tests := []struct {
		name       string
		multiplier int
		want       int
		wantErr    bool
	}{
		{name: "default", multiplier: 0, want: 1},
		{name: "one", multiplier: 1, want: 1},
		{name: "two", multiplier: 2, want: 2},
		{name: "ten", multiplier: 10, want: 10},
		{name: "eleven", multiplier: 11, wantErr: true},
		{name: "negative", multiplier: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := minimalValidServerConfig()
			config.UDPForwardingRedundancyMultiplier = tt.multiplier

			err := config.fill()

			if tt.wantErr {
				assert.ErrorContains(t, err, "UDPForwardingRedundancyMultiplier")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, config.UDPForwardingRedundancyMultiplier)
		})
	}
}

func minimalValidServerConfig() Config {
	return Config{
		TLSConfig: TLSConfig{
			GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
				return nil, nil
			},
		},
		Conn:          noopPacketConn{},
		Authenticator: noopAuthenticator{},
	}
}

type noopPacketConn struct{}

func (noopPacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, nil, io.EOF
}

func (noopPacketConn) WriteTo([]byte, net.Addr) (int, error) {
	return 0, nil
}

func (noopPacketConn) Close() error {
	return nil
}

func (noopPacketConn) LocalAddr() net.Addr {
	return &net.UDPAddr{}
}

func (noopPacketConn) SetDeadline(time.Time) error {
	return nil
}

func (noopPacketConn) SetReadDeadline(time.Time) error {
	return nil
}

func (noopPacketConn) SetWriteDeadline(time.Time) error {
	return nil
}

type noopAuthenticator struct{}

func (noopAuthenticator) Authenticate(net.Addr, string, uint64) (bool, string) {
	return true, "user"
}
