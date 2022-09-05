// Copyright 2021 Nokia
// Licensed under the BSD 3-Clause License.
// SPDX-License-Identifier: BSD-3-Clause

package vr_nxos

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/cloudflare/cfssl/log"
	"github.com/srl-labs/containerlab/nodes"
	"github.com/srl-labs/containerlab/runtime"
	"github.com/srl-labs/containerlab/types"
	"github.com/srl-labs/containerlab/utils"
)

var kindnames = []string{"vr-nxos", "vr-cisco_nxos"}

const (
	configDirName   = "config"
	startupCfgFName = "startup-config.cfg"

	defaultUser     = "admin"
	defaultPassword = "admin"
)

func init() {
	nodes.Register(kindnames, func() nodes.Node {
		return new(vrNXOS)
	})
	nodes.SetDefaultCredentials(kindnames, defaultUser, defaultPassword)
}

type vrNXOS struct {
	cfg     *types.NodeConfig
	mgmt    *types.MgmtNet
	runtime runtime.ContainerRuntime
}

func (s *vrNXOS) Init(cfg *types.NodeConfig, opts ...nodes.NodeOption) error {
	s.cfg = cfg
	for _, o := range opts {
		o(s)
	}
	// env vars are used to set launch.py arguments in vrnetlab container
	defEnv := map[string]string{
		"USERNAME":           defaultUser,
		"PASSWORD":           defaultPassword,
		"CONNECTION_MODE":    nodes.VrDefConnMode,
		"VCPU":               "2",
		"RAM":                "4096",
		"DOCKER_NET_V4_ADDR": s.mgmt.IPv4Subnet,
		"DOCKER_NET_V6_ADDR": s.mgmt.IPv6Subnet,
	}
	s.cfg.Env = utils.MergeStringMaps(defEnv, s.cfg.Env)

	// mount config dir to support startup-config functionality
	s.cfg.Binds = append(s.cfg.Binds, fmt.Sprint(path.Join(s.cfg.LabDir, configDirName), ":/config"))

	s.cfg.Cmd = fmt.Sprintf("--username %s --password %s --hostname %s --connection-mode %s --trace",
		s.cfg.Env["USERNAME"], s.cfg.Env["PASSWORD"], s.cfg.ShortName, s.cfg.Env["CONNECTION_MODE"])

	// set virtualization requirement
	s.cfg.HostRequirements.VirtRequired = true

	return nil
}

func (s *vrNXOS) Config() *types.NodeConfig { return s.cfg }

func (s *vrNXOS) PreDeploy(_, _, _ string) error {
	utils.CreateDirectory(s.cfg.LabDir, 0777)
	return loadStartupConfigFile(s.cfg)
}

func (s *vrNXOS) Deploy(ctx context.Context) error {
	cID, err := s.runtime.CreateContainer(ctx, s.cfg)
	if err != nil {
		return err
	}
	_, err = s.runtime.StartContainer(ctx, cID, s.cfg)
	return err
}

func (s *vrNXOS) GetImages() map[string]string {
	return map[string]string{
		nodes.ImageKey: s.cfg.Image,
	}
}

func (*vrNXOS) PostDeploy(_ context.Context, _ map[string]nodes.Node) error {
	return nil
}

func (s *vrNXOS) WithMgmtNet(mgmt *types.MgmtNet) { s.mgmt = mgmt }
func (s *vrNXOS) WithRuntime(r runtime.ContainerRuntime) {
	s.runtime = r
}
func (s *vrNXOS) GetRuntime() runtime.ContainerRuntime { return s.runtime }

func (s *vrNXOS) Delete(ctx context.Context) error {
	return s.runtime.DeleteContainer(ctx, s.cfg.LongName)
}

func (*vrNXOS) SaveConfig(_ context.Context) error {
	return nil
}

func loadStartupConfigFile(node *types.NodeConfig) error {
	// create config directory that will be bind mounted to vrnetlab container at / path
	utils.CreateDirectory(path.Join(node.LabDir, configDirName), 0777)

	if node.StartupConfig != "" {
		// dstCfg is a path to a file on the clab host that will have rendered configuration
		dstCfg := filepath.Join(node.LabDir, configDirName, startupCfgFName)

		c, err := os.ReadFile(node.StartupConfig)
		if err != nil {
			return err
		}

		cfgTemplate := string(c)

		err = node.GenerateConfig(dstCfg, cfgTemplate)
		if err != nil {
			log.Errorf("node=%s, failed to generate config: %v", node.ShortName, err)
		}
	}
	return nil
}
