package hy2

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/apernet/quic-go"
)

const statelessResetSecretFile = "hy2_stateless_reset.key"

// loadStatelessResetKey derives a per-node stateless reset key from a secret kept next to the
// hysteria2 config file. A random key (the core's default) changes on every restart or node
// reload, so clients still holding an old connection could not be reset and would have to
// wait for the idle timeout. It returns nil when no config path is set.
func loadStatelessResetKey(configPath, tag string) (*quic.StatelessResetKey, error) {
	if configPath == "" {
		return nil, nil
	}
	secret, err := loadStatelessResetSecret(filepath.Join(filepath.Dir(configPath), statelessResetSecretFile))
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(tag))
	var key quic.StatelessResetKey
	copy(key[:], mac.Sum(nil))
	return &key, nil
}

func loadStatelessResetSecret(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		secret, err := hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil || len(secret) < 32 {
			return nil, fmt.Errorf("invalid stateless reset secret in %s", path)
		}
		return secret, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		// Created concurrently by another node
		return loadStatelessResetSecret(path)
	} else if err != nil {
		return nil, err
	}
	_, err = f.WriteString(hex.EncodeToString(secret) + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return secret, nil
}
