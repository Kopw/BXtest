package hy2

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStatelessResetKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "hy2config.yaml")

	a1, err := loadStatelessResetKey(configPath, "node-a")
	if err != nil || a1 == nil {
		t.Fatalf("first load: key=%v err=%v", a1, err)
	}
	info, err := os.Stat(filepath.Join(dir, statelessResetSecretFile))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); os.PathSeparator == '/' && perm&0o077 != 0 {
		t.Fatalf("secret file permission = %o", perm)
	}

	// The same node keeps its key across restarts, other nodes get a different one
	a2, err := loadStatelessResetKey(configPath, "node-a")
	if err != nil || *a2 != *a1 {
		t.Fatalf("reload changed the key: err=%v", err)
	}
	b, err := loadStatelessResetKey(configPath, "node-b")
	if err != nil || *b == *a1 {
		t.Fatalf("node-b shares node-a's key: err=%v", err)
	}

	// A different secret gives different keys
	other, err := loadStatelessResetKey(filepath.Join(t.TempDir(), "hy2config.yaml"), "node-a")
	if err != nil || *other == *a1 {
		t.Fatalf("different secret gave the same key: err=%v", err)
	}

	if key, err := loadStatelessResetKey("", "node-a"); key != nil || err != nil {
		t.Fatalf("empty config path: key=%v err=%v", key, err)
	}

	if err := os.WriteFile(filepath.Join(dir, statelessResetSecretFile), []byte("bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadStatelessResetKey(configPath, "node-a"); err == nil {
		t.Fatal("invalid secret should fail")
	}
}

func TestGetUDPForwardingRedundancyExcludePort(t *testing.T) {
	c := &serverConfig{UDPForwardingRedundancy: serverConfigUDPForwardingRedundancy{Enabled: true, ExcludePorts: "53,123,3478-3481"}}
	exclude, err := getUDPForwardingRedundancyExcludePort(c)
	if err != nil || exclude == nil {
		t.Fatalf("exclude is nil=%v err=%v", exclude == nil, err)
	}
	for port, want := range map[uint16]bool{53: true, 123: true, 3478: true, 3481: true, 3482: false, 3659: false, 27015: false} {
		if exclude(port) != want {
			t.Errorf("port %d excluded=%v, want %v", port, !want, want)
		}
	}

	c.UDPForwardingRedundancy.ExcludePorts = "53,abc"
	if _, err := getUDPForwardingRedundancyExcludePort(c); err == nil {
		t.Error("invalid port union should fail")
	}
	c.UDPForwardingRedundancy = serverConfigUDPForwardingRedundancy{ExcludePorts: "53"}
	if exclude, err := getUDPForwardingRedundancyExcludePort(c); exclude != nil || err != nil {
		t.Error("disabled redundancy should exclude nothing")
	}
}
