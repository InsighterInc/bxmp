// Copyright 2017 The BXMP Authors
// This file is part of BXMP.
//
// BXMP is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// BXMP is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with BXMP. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"unicode"

	cli "gopkg.in/urfave/cli.v1"

	"github.com/InsighterInc/bxmp/cmd/utils"
	"github.com/InsighterInc/bxmp/contracts/release"
	"github.com/InsighterInc/bxmp/bxm"
	"github.com/InsighterInc/bxmp/node"
	"github.com/InsighterInc/bxmp/p2p/discover"
	"github.com/InsighterInc/bxmp/params"
	"github.com/InsighterInc/bxmp/raft"
	whisper "github.com/InsighterInc/bxmp/whisper/whisperv5"
	"github.com/naoina/toml"
	"time"
)

var (
	dumpConfigCommand = cli.Command{
		Action:      utils.MigrateFlags(dumpConfig),
		Name:        "dumpconfig",
		Usage:       "Show configuration values",
		ArgsUsage:   "",
		Flags:       append(append(nodeFlags, rpcFlags...), whisperFlags...),
		Category:    "MISCELLANEOUS COMMANDS",
		Description: `The dumpconfig command shows configuration values.`,
	}

	configFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

type bxmstatsConfig struct {
	URL string `toml:",omitempty"`
}

type gethConfig struct {
	Bxm      bxm.Config
	Shh      whisper.Config
	Node     node.Config
	Bxmstats bxmstatsConfig
}

func loadConfig(file string, cfg *gethConfig) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() node.Config {
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit)
	cfg.HTTPModules = append(cfg.HTTPModules, "bxm", "shh")
	cfg.WSModules = append(cfg.WSModules, "bxm", "shh")
	cfg.IPCPath = "geth.ipc"
	return cfg
}

func makeConfigNode(ctx *cli.Context) (*node.Node, gethConfig) {
	// Load defaults.
	cfg := gethConfig{
		Bxm:  bxm.DefaultConfig,
		Shh:  whisper.DefaultConfig,
		Node: defaultNodeConfig(),
	}

	// Load config file.
	if file := ctx.GlobalString(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}
	utils.SetEthConfig(ctx, stack, &cfg.Bxm)
	if ctx.GlobalIsSet(utils.BxmStatsURLFlag.Name) {
		cfg.Bxmstats.URL = ctx.GlobalString(utils.BxmStatsURLFlag.Name)
	}

	utils.SetShhConfig(ctx, stack, &cfg.Shh)
	cfg.Bxm.RaftMode = ctx.GlobalBool(utils.RaftModeFlag.Name)

	return stack, cfg
}

// enableWhisper returns true in case one of the whisper flags is set.
func enableWhisper(ctx *cli.Context) bool {
	for _, flag := range whisperFlags {
		if ctx.GlobalIsSet(flag.GetName()) {
			return true
		}
	}
	return false
}

func makeFullNode(ctx *cli.Context) *node.Node {
	stack, cfg := makeConfigNode(ctx)

	ethChan := utils.RegisterEthService(stack, &cfg.Bxm)

	if ctx.GlobalBool(utils.RaftModeFlag.Name) {
		RegisterRaftService(stack, ctx, cfg, ethChan)
	}

	// Whisper must be explicitly enabled by specifying at least 1 whisper flag or in dev mode
	shhEnabled := enableWhisper(ctx)
	shhAutoEnabled := !ctx.GlobalIsSet(utils.WhisperEnabledFlag.Name) && ctx.GlobalIsSet(utils.DevModeFlag.Name)
	if shhEnabled || shhAutoEnabled {
		if ctx.GlobalIsSet(utils.WhisperMaxMessageSizeFlag.Name) {
			cfg.Shh.MaxMessageSize = uint32(ctx.Int(utils.WhisperMaxMessageSizeFlag.Name))
		}
		if ctx.GlobalIsSet(utils.WhisperMinPOWFlag.Name) {
			cfg.Shh.MinimumAcceptedPOW = ctx.Float64(utils.WhisperMinPOWFlag.Name)
		}
		utils.RegisterShhService(stack, &cfg.Shh)
	}

	// Add the BitMED Stats daemon if requested.
	if cfg.Bxmstats.URL != "" {
		utils.RegisterBxmStatsService(stack, cfg.Bxmstats.URL)
	}

	// Add the release oracle service so it boots along with node.
	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		config := release.Config{
			Oracle: relOracle,
			Major:  uint32(params.VersionMajor),
			Minor:  uint32(params.VersionMinor),
			Patch:  uint32(params.VersionPatch),
		}
		commit, _ := hex.DecodeString(gitCommit)
		copy(config.Commit[:], commit)
		return release.NewReleaseService(ctx, config)
	}); err != nil {
		utils.Fatalf("Failed to register the Geth release oracle service: %v", err)
	}
	return stack
}

// dumpConfig is the dumpconfig command.
func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)
	comment := ""

	if cfg.Bxm.Genesis != nil {
		cfg.Bxm.Genesis = nil
		comment += "# Note: this config doesn't contain the genesis block.\n\n"
	}

	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}
	io.WriteString(os.Stdout, comment)
	os.Stdout.Write(out)
	return nil
}

func RegisterRaftService(stack *node.Node, ctx *cli.Context, cfg gethConfig, ethChan <-chan *bxm.BitMED) {
	blockTimeMillis := ctx.GlobalInt(utils.RaftBlockTimeFlag.Name)
	datadir := ctx.GlobalString(utils.DataDirFlag.Name)
	joinExistingId := ctx.GlobalInt(utils.RaftJoinExistingFlag.Name)

	raftPort := uint16(ctx.GlobalInt(utils.RaftPortFlag.Name))

	if err := stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
		privkey := cfg.Node.NodeKey()
		strId := discover.PubkeyID(&privkey.PublicKey).String()
		blockTimeNanos := time.Duration(blockTimeMillis) * time.Millisecond
		peers := cfg.Node.StaticNodes()

		var myId uint16
		var joinExisting bool

		if joinExistingId > 0 {
			myId = uint16(joinExistingId)
			joinExisting = true
		} else if len(peers) == 0 {
			utils.Fatalf("Raft-based consensus requires either (1) an initial peers list (in static-nodes.json) including this enode hash (%v), or (2) the flag --raftjoinexisting RAFT_ID, where RAFT_ID has been issued by an existing cluster member calling `raft.addPeer(ENODE_ID)` with an enode ID containing this node's enode hash.", strId)
		} else {
			peerIds := make([]string, len(peers))

			for peerIdx, peer := range peers {
				if !peer.HasRaftPort() {
					utils.Fatalf("raftport querystring parameter not specified in static-node enode ID: %v. please check your static-nodes.json file.", peer.String())
				}

				peerId := peer.ID.String()
				peerIds[peerIdx] = peerId
				if peerId == strId {
					myId = uint16(peerIdx) + 1
				}
			}

			if myId == 0 {
				utils.Fatalf("failed to find local enode ID (%v) amongst peer IDs: %v", strId, peerIds)
			}
		}

		bitmed := <-ethChan

		return raft.New(ctx, bitmed.ChainConfig(), myId, raftPort, joinExisting, blockTimeNanos, bitmed, peers, datadir)
	}); err != nil {
		utils.Fatalf("Failed to register the Raft service: %v", err)
	}

}
