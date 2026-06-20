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
		name                  string
		writeToMultiplier     int
		sendMessageMultiplier int
		wantWriteTo           int
		wantSendMessage       int
		wantErr               string
	}{
		{name: "default", wantWriteTo: 1, wantSendMessage: 1},
		{name: "one", writeToMultiplier: 1, sendMessageMultiplier: 1, wantWriteTo: 1, wantSendMessage: 1},
		{name: "split", writeToMultiplier: 2, sendMessageMultiplier: 3, wantWriteTo: 2, wantSendMessage: 3},
		{name: "one hundred", writeToMultiplier: 100, sendMessageMultiplier: 100, wantWriteTo: 100, wantSendMessage: 100},
		{name: "writeTo one hundred one", writeToMultiplier: 101, sendMessageMultiplier: 1, wantErr: "UDPForwardingRedundancyWriteToMultiplier"},
		{name: "sendMessage one hundred one", writeToMultiplier: 1, sendMessageMultiplier: 101, wantErr: "UDPForwardingRedundancySendMessageMultiplier"},
		{name: "writeTo negative", writeToMultiplier: -1, sendMessageMultiplier: 1, wantErr: "UDPForwardingRedundancyWriteToMultiplier"},
		{name: "sendMessage negative", writeToMultiplier: 1, sendMessageMultiplier: -1, wantErr: "UDPForwardingRedundancySendMessageMultiplier"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := minimalValidServerConfig()
			config.UDPForwardingRedundancyWriteToMultiplier = tt.writeToMultiplier
			config.UDPForwardingRedundancySendMessageMultiplier = tt.sendMessageMultiplier

			err := config.fill()

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantWriteTo, config.UDPForwardingRedundancyWriteToMultiplier)
			assert.Equal(t, tt.wantSendMessage, config.UDPForwardingRedundancySendMessageMultiplier)
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
