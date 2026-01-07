/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package apps

import (
	"context"
	"fmt"
	"time"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/pkg/core"
	"github.com/basenana/nanafs/pkg/types"

	"github.com/spf13/cobra"

	"github.com/basenana/nanafs/cmd/apps/apis"
	fsapi "github.com/basenana/nanafs/cmd/apps/fuse"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

func init() {
	RootCmd.AddCommand(daemonCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(NamespaceCmd)
}

var RootCmd = &cobra.Command{
	Use:   "nanafs",
	Short: "NanaFS engine server",
	Long:  `FS-style workflow engine for unified data management.`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	daemonCmd.Flags().StringVar(&config.FilePath, "config", "", "nanafs config file")
	daemonCmd.Flags().BoolVar(&config.AutoInit, "auto-init", false, "generate default config if not exists")
}

var daemonCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start server service",
	PreRun: func(cmd *cobra.Command, args []string) {

	},
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.NewConfig()
		if err != nil {
			panic(err)
		}

		boot := cfg.GetBootstrapConfig()
		if boot.Debug {
			logger.SetDebug(boot.Debug)
		}

		meta, err := metastore.NewMetaStorage(boot.Meta.Type, boot.Meta)
		if err != nil {
			panic(err)
		}
		if len(boot.Storages) == 0 {
			panic("storage must config")
		}

		if err = cfg.RegisterCMDB(meta); err != nil {
			panic(err)
		}
		depends, err := common.InitDepends(cfg, meta)
		if err != nil {
			panic(err)
		}

		stop := utils.HandleTerminalSignal()
		run(depends, cfg, stop)
	},
}

func run(depends *common.Depends, cfg config.Config, stopCh chan struct{}) {
	log := logger.NewLogger("nanafs")
	log.Infow("starting", "version", config.VersionInfo().Version())

	shutdown := core.SetupShutdownHandler(stopCh)
	boot := cfg.GetBootstrapConfig()

	ctx, canF := context.WithCancel(context.Background())
	defer canF()

	defaultFS, err := core.NewFileSystem(depends.Core, depends.Meta, types.DefaultNamespace)
	if err != nil {
		log.Panic("failed to create default filesystem")
	}
	depends.Workflow.Start(ctx)

	if boot.API.Enable {
		err = apis.RunFSAPI(depends, cfg, stopCh)
		if err != nil {
			log.Panicw("run fsapi failed", "err", err)
		}
	}
	if boot.Webdav.Enable {
		err = apis.RunWebdav(defaultFS, boot.Webdav, stopCh)
		if err != nil {
			log.Panicw("run fsapi failed", "err", err)
		}
	}

	if boot.FUSE.Enable {
		err = fsapi.Run(stopCh, defaultFS, boot.FUSE, boot.Debug)
		if err != nil {
			log.Panicw("run fuse failed", "err", err)
		}
	}

	log.Info("started")
	<-shutdown
	time.Sleep(time.Second * 5)
	log.Info("stopped")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "View version information",
	Run: func(cmd *cobra.Command, args []string) {
		vInfo := config.VersionInfo()
		fmt.Printf("Version: %s\n", vInfo.Version())
		fmt.Printf("GitCommit: %s\n", vInfo.Git)
	},
}

var NamespaceCmd = &cobra.Command{
	Use:   "namespace",
	Short: "create namespace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.NewConfig()
		if err != nil {
			panic(err)
		}

		boot := cfg.GetBootstrapConfig()
		if boot.Debug {
			logger.SetDebug(boot.Debug)
		}

		meta, err := metastore.NewMetaStorage(boot.Meta.Type, boot.Meta)
		if err != nil {
			panic(err)
		}

		if err = cfg.RegisterCMDB(meta); err != nil {
			panic(err)
		}

		if len(boot.Storages) == 0 {
			panic("storage must config")
		}

		depends, err := common.InitDepends(cfg, meta)
		if err != nil {
			panic(err)
		}

		ctx := context.Background()
		namespace := args[0]
		err = createNamespace(ctx, depends.Core, namespace)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Namespace '%s' created successfully\n", namespace)
	},
}

func createNamespace(ctx context.Context, core core.Core, namespace string) error {
	log := logger.NewLogger("nanafs.namespace")
	err := core.CreateNamespace(ctx, namespace)
	if err != nil {
		log.Errorw("create namespace failed", "err", err)
		return err
	}
	return nil
}
