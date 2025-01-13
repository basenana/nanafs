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
	"github.com/basenana/nanafs/services"
	"path"
	"time"

	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/rule"

	"github.com/spf13/cobra"

	"github.com/basenana/nanafs/cmd/apps/apis"
	configapp "github.com/basenana/nanafs/cmd/apps/config"
	fsapi "github.com/basenana/nanafs/cmd/apps/fuse"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/bio"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

func init() {
	RootCmd.AddCommand(daemonCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(NamespaceCmd)
	RootCmd.AddCommand(configapp.RunCmd)
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
	daemonCmd.Flags().StringVar(&config.FilePath, "config", path.Join(config.LocalUserPath(), config.DefaultConfigBase), "nanafs config file")
	NamespaceCmd.Flags().StringVar(&config.FilePath, "config", path.Join(config.LocalUserPath(), config.DefaultConfigBase), "nanafs config file")
}

var daemonCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start server service",
	PreRun: func(cmd *cobra.Command, args []string) {

	},
	Run: func(cmd *cobra.Command, args []string) {
		loader := config.NewConfigLoader()
		cfg, err := loader.GetBootstrapConfig()
		if err != nil {
			panic(err)
		}

		if cfg.Debug {
			logger.SetDebug(cfg.Debug)
		}

		meta, err := metastore.NewMetaStorage(cfg.Meta.Type, cfg.Meta)
		if err != nil {
			panic(err)
		}

		err = loader.InitCMDB(meta)
		if err != nil {
			panic(err)
		}

		if len(cfg.Storages) == 0 {
			panic("storage must config")
		}

		bio.InitPageCache(cfg.FS)
		storage.InitLocalCache(cfg)
		rule.InitQuery(meta)

		fridayClient := friday.NewFridayClient(cfg.FridayConfig)
		depends, err := services.InitDepends(loader, meta, fridayClient)
		if err != nil {
			panic(err)
		}

		ctrl, err := controller.New(loader, meta, fridayClient)
		if err != nil {
			panic(err)
		}

		fsSvc, err := services.NewService(depends)
		if err != nil {
			panic(err)
		}
		stop := utils.HandleTerminalSignal()

		run(ctrl, fsSvc, depends, loader, cfg, stop)
	},
}

func run(ctrl controller.Controller, fsSvc *services.Service, depends *services.Depends, cfgLoader config.Loader, cfg config.Bootstrap, stopCh chan struct{}) {
	log := logger.NewLogger("nanafs")
	log.Infow("starting", "version", config.VersionInfo().Version())
	ctrl.StartBackendTask(stopCh)
	shutdown := ctrl.SetupShutdownHandler(stopCh)

	ctx, canF := context.WithCancel(context.Background())
	defer canF()

	depends.Workflow.Start(ctx)

	pathEntryMgr, err := apis.NewPathEntryManager(ctrl)
	if err != nil {
		log.Panicf("init api path entry manager error: %s", err)
	}
	err = apis.Setup(ctrl, fsSvc, depends, pathEntryMgr, cfgLoader, stopCh)
	if err != nil {
		log.Panicw("setup api servers failed", "err", err.Error())
	}
	if cfg.FUSE.Enable {
		fsServer, err := fsapi.NewNanaFsRoot(cfg.FUSE, ctrl)
		if err != nil {
			panic(err)
		}
		fsServer.SetDebug(cfg.Debug)
		err = fsServer.Start(stopCh)
		if err != nil {
			panic(err)
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
	Run: func(cmd *cobra.Command, args []string) {
		loader := config.NewConfigLoader()
		cfg, err := loader.GetBootstrapConfig()
		if err != nil {
			panic(err)
		}

		if cfg.Debug {
			logger.SetDebug(cfg.Debug)
		}

		meta, err := metastore.NewMetaStorage(cfg.Meta.Type, cfg.Meta)
		if err != nil {
			panic(err)
		}

		err = loader.InitCMDB(meta)
		if err != nil {
			panic(err)
		}

		if len(cfg.Storages) == 0 {
			panic("storage must config")
		}

		bio.InitPageCache(cfg.FS)
		storage.InitLocalCache(cfg)
		rule.InitQuery(meta)

		fridayClient := friday.NewFridayClient(cfg.FridayConfig)

		ctrl, err := controller.New(loader, meta, fridayClient)
		if err != nil {
			panic(err)
		}

		_, err = ctrl.CreateNamespace(context.Background(), args[0])
		if err != nil {
			panic(err)
		}
	},
}
