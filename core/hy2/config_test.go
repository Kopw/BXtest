package hy2

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestUDPForwardingRedundancyConfigParsing(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(`
udpForwardingRedundancy:
  enabled: true
  multiplier: 3
`)); err != nil {
		t.Fatal(err)
	}

	var c serverConfig
	if err := v.Unmarshal(&c); err != nil {
		t.Fatal(err)
	}
	if !c.UDPForwardingRedundancy.Enabled {
		t.Fatal("expected udpForwardingRedundancy.enabled to be true")
	}
	if c.UDPForwardingRedundancy.Multiplier != 3 {
		t.Fatalf("multiplier = %d, want 3", c.UDPForwardingRedundancy.Multiplier)
	}

	got, err := getUDPForwardingRedundancyMultiplier(&c)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("effective multiplier = %d, want 3", got)
	}
}

func TestGetUDPForwardingRedundancyMultiplier(t *testing.T) {
	tests := []struct {
		name    string
		config  serverConfigUDPForwardingRedundancy
		want    int
		wantErr bool
	}{
		{name: "default disabled", want: 1},
		{name: "disabled explicit one", config: serverConfigUDPForwardingRedundancy{Multiplier: 1}, want: 1},
		{name: "disabled ignores valid multiplier", config: serverConfigUDPForwardingRedundancy{Multiplier: 10}, want: 1},
		{name: "enabled one", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: 1}, want: 1},
		{name: "enabled two", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: 2}, want: 2},
		{name: "enabled ten", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: 10}, want: 10},
		{name: "enabled zero", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: 0}, wantErr: true},
		{name: "enabled eleven", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: 11}, wantErr: true},
		{name: "enabled negative", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: -1}, wantErr: true},
		{name: "disabled eleven", config: serverConfigUDPForwardingRedundancy{Multiplier: 11}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &serverConfig{UDPForwardingRedundancy: tt.config}

			got, err := getUDPForwardingRedundancyMultiplier(c)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("multiplier = %d, want %d", got, tt.want)
			}
		})
	}
}
