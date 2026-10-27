package hy2

import (
	"fmt"
	"net"
	"strings"

	"github.com/InazumaV/V2bX/api/panel"
	"github.com/InazumaV/V2bX/conf"
	"github.com/InazumaV/V2bX/core/hy2/mimic"
	"github.com/apernet/hysteria/core/v2/server"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Hysteria2node struct {
	Hy2server     server.Server
	Tag           string
	Logger        *zap.Logger
	EventLogger   server.EventLogger
	TrafficLogger server.TrafficLogger
	Mimic         *mimic.Instance
}

func (h *Hysteria2) AddNode(tag string, info *panel.NodeInfo, config *conf.Options) error {
	var err error
	hyconfig := &server.Config{}
	var c serverConfig
	v := viper.New()
	if len(config.Hysteria2ConfigPath) != 0 {
		v.SetConfigFile(config.Hysteria2ConfigPath)
		if err := v.ReadInConfig(); err != nil {
			h.Logger.Fatal("failed to read server config", zap.Error(err))
		}
		if err := v.Unmarshal(&c); err != nil {
			h.Logger.Fatal("failed to parse server config", zap.Error(err))
		}
	}
	n := Hysteria2node{
		Tag:    tag,
		Logger: h.Logger,
		EventLogger: &serverLogger{
			Tag:    tag,
			logger: h.Logger,
		},
		TrafficLogger: &HookServer{
			Tag:                   tag,
			logger:                h.Logger,
			ReportMinTrafficBytes: config.ReportMinTraffic * 1024,
		},
	}

	hyconfig, err = n.getHyConfig(info, config, &c)
	if err != nil {
		return err
	}
	hyconfig.Authenticator = h.Auth
	if c.Mimic.Enabled {
		addr, ok := hyconfig.Conn.LocalAddr().(*net.UDPAddr)
		if !ok {
			_ = hyconfig.Conn.Close()
			return fmt.Errorf("mimic: unsupported listen address %s", hyconfig.Conn.LocalAddr())
		}
		n.Mimic, err = mimic.Start(mimic.Config{
			Enabled:   true,
			Interface: c.Mimic.Interface,
			XDPMode:   c.Mimic.XDPMode,
			Path:      c.Mimic.Path,
			ExtraArgs: c.Mimic.ExtraArgs,
		}, mimic.RoleServer, []*net.UDPAddr{addr}, h.Logger, func(err error) {
			h.Logger.Error("mimic stopped, clients of this node can't connect until it is reloaded", zap.String("tag", tag), zap.Error(err))
		})
		if err != nil {
			_ = hyconfig.Conn.Close()
			return fmt.Errorf("start mimic: %w", err)
		}
	}
	s, err := server.NewServer(hyconfig)
	if err != nil {
		_ = n.Mimic.Close()
		return err
	}
	n.Hy2server = s
	h.Hy2nodes[tag] = n
	go func() {
		if err := s.Serve(); err != nil {
			if !strings.Contains(err.Error(), "quic: server closed") {
				h.Logger.Error("Server Error", zap.Error(err))
			}
		}
	}()
	return nil
}

func (h *Hysteria2) DelNode(tag string) error {
	err := h.Hy2nodes[tag].Hy2server.Close()
	if err != nil {
		return err
	}
	if err := h.Hy2nodes[tag].Mimic.Close(); err != nil {
		h.Logger.Warn("failed to stop mimic", zap.String("tag", tag), zap.Error(err))
	}
	delete(h.Hy2nodes, tag)
	return nil
}
