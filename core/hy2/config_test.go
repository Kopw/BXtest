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
  writeToMultiplier: 3
  sendMessageMultiplier: 4
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
	if c.UDPForwardingRedundancy.WriteToMultiplier == nil || *c.UDPForwardingRedundancy.WriteToMultiplier != 3 {
		t.Fatalf("writeToMultiplier = %v, want 3", c.UDPForwardingRedundancy.WriteToMultiplier)
	}
	if c.UDPForwardingRedundancy.SendMessageMultiplier == nil || *c.UDPForwardingRedundancy.SendMessageMultiplier != 4 {
		t.Fatalf("sendMessageMultiplier = %v, want 4", c.UDPForwardingRedundancy.SendMessageMultiplier)
	}

	got, err := getUDPForwardingRedundancyMultipliers(&c)
	if err != nil {
		t.Fatal(err)
	}
	if got.WriteTo != 3 {
		t.Fatalf("effective writeToMultiplier = %d, want 3", got.WriteTo)
	}
	if got.SendMessage != 4 {
		t.Fatalf("effective sendMessageMultiplier = %d, want 4", got.SendMessage)
	}
}

func TestGetUDPForwardingRedundancyMultipliers(t *testing.T) {
	tests := []struct {
		name    string
		config  serverConfigUDPForwardingRedundancy
		want    udpForwardingRedundancyMultipliers
		wantErr bool
	}{
		{name: "default disabled", want: udpForwardingRedundancyMultipliers{WriteTo: 1, SendMessage: 1}},
		{name: "disabled explicit legacy one", config: serverConfigUDPForwardingRedundancy{Multiplier: ptrInt(1)}, want: udpForwardingRedundancyMultipliers{WriteTo: 1, SendMessage: 1}},
		{name: "disabled ignores valid split multipliers", config: serverConfigUDPForwardingRedundancy{WriteToMultiplier: ptrInt(100), SendMessageMultiplier: ptrInt(100)}, want: udpForwardingRedundancyMultipliers{WriteTo: 1, SendMessage: 1}},
		{name: "enabled legacy fallback", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: ptrInt(3)}, want: udpForwardingRedundancyMultipliers{WriteTo: 3, SendMessage: 3}},
		{name: "enabled split", config: serverConfigUDPForwardingRedundancy{Enabled: true, WriteToMultiplier: ptrInt(2), SendMessageMultiplier: ptrInt(4)}, want: udpForwardingRedundancyMultipliers{WriteTo: 2, SendMessage: 4}},
		{name: "split overrides legacy fallback", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: ptrInt(3), WriteToMultiplier: ptrInt(5), SendMessageMultiplier: ptrInt(6)}, want: udpForwardingRedundancyMultipliers{WriteTo: 5, SendMessage: 6}},
		{name: "partial writeTo override", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: ptrInt(3), WriteToMultiplier: ptrInt(5)}, want: udpForwardingRedundancyMultipliers{WriteTo: 5, SendMessage: 3}},
		{name: "partial sendMessage override", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: ptrInt(3), SendMessageMultiplier: ptrInt(6)}, want: udpForwardingRedundancyMultipliers{WriteTo: 3, SendMessage: 6}},
		{name: "enabled defaults unspecified to one", config: serverConfigUDPForwardingRedundancy{Enabled: true}, want: udpForwardingRedundancyMultipliers{WriteTo: 1, SendMessage: 1}},
		{name: "enabled one hundred", config: serverConfigUDPForwardingRedundancy{Enabled: true, WriteToMultiplier: ptrInt(100), SendMessageMultiplier: ptrInt(100)}, want: udpForwardingRedundancyMultipliers{WriteTo: 100, SendMessage: 100}},
		{name: "legacy zero", config: serverConfigUDPForwardingRedundancy{Enabled: true, Multiplier: ptrInt(0)}, wantErr: true},
		{name: "writeTo zero", config: serverConfigUDPForwardingRedundancy{Enabled: true, WriteToMultiplier: ptrInt(0)}, wantErr: true},
		{name: "sendMessage zero", config: serverConfigUDPForwardingRedundancy{Enabled: true, SendMessageMultiplier: ptrInt(0)}, wantErr: true},
		{name: "writeTo one hundred one", config: serverConfigUDPForwardingRedundancy{Enabled: true, WriteToMultiplier: ptrInt(101)}, wantErr: true},
		{name: "sendMessage negative", config: serverConfigUDPForwardingRedundancy{Enabled: true, SendMessageMultiplier: ptrInt(-1)}, wantErr: true},
		{name: "disabled invalid still rejected", config: serverConfigUDPForwardingRedundancy{WriteToMultiplier: ptrInt(101)}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &serverConfig{UDPForwardingRedundancy: tt.config}

			got, err := getUDPForwardingRedundancyMultipliers(c)

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
				t.Fatalf("multipliers = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func ptrInt(v int) *int {
	return &v
}
