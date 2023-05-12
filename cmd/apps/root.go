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
	"fmt"
	"github.com/basenana/nanafs/cmd/apps/apis"
	configapp "github.com/basenana/nanafs/cmd/apps/config"
	"github.com/basenana/nanafs/cmd/apps/fs"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/plugin"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/spf13/cobra"
	"path"
	"time"
)

func init() {
	RootCmd.AddCommand(daemonCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(configapp.RunCmd)
}

var RootCmd = &cobra.Command{
	Use:   "nanafs",
	Short: "nanafs daemon",
	Long:  `Everything is an object, everything can be thrown into workflows`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	daemonCmd.Flags().StringVar(&config.FilePath, "config", path.Join(config.LocalUserPath(), config.DefaultConfigBase), "nanafs config file")
}

var daemonCmd = &cobra.Command{
	Use:   "serve",
	Short: "NanaFS server run",
	PreRun: func(cmd *cobra.Command, args []string) {

	},
	Run: func(cmd *cobra.Command, args []string) {
		loader := config.NewConfigLoader()
		cfg, err := loader.GetConfig()
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

		if len(cfg.Storages) == 0 {
			panic("storage must config")
		}

		storage.InitLocalCache(cfg)

		ctrl, err := controller.New(loader, meta)
		if err != nil {
			panic(err)
		}
		stop := utils.HandleTerminalSignal()

		if err := plugin.Init(cfg, meta); err != nil {
			panic(err)
		}

		run(ctrl, cfg, stop)
	},
}

func run(ctrl controller.Controller, cfg config.Config, stopCh chan struct{}) {
	log := logger.NewLogger("nanafs")
	log.Infow("starting", "version", config.VersionInfo().Version())
	ctrl.StartBackendTask(stopCh)
	shutdown := ctrl.SetupShutdownHandler(stopCh)

	pathEntryMgr, err := apis.NewPathEntryManager(ctrl)
	if err != nil {
		log.Panicf("init api path entry manager error: %s", err)
	}
	if cfg.Api.Enable {
		s, err := apis.NewApiServer(pathEntryMgr, cfg)
		if err != nil {
			log.Panicw("init http server failed", "err", err.Error())
		}
		go s.Run(stopCh)
	}

	if cfg.Webdav != nil && cfg.Webdav.Enable {
		w, err := apis.NewWebdavServer(pathEntryMgr, cfg)
		if err != nil {
			log.Panicw("init http server failed", "err", err.Error())
		}
		go w.Run(stopCh)
	}

	if cfg.FUSE.Enable {
		fsServer, err := fs.NewNanaFsRoot(cfg.FUSE, ctrl)
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
	Short: "NanaFS version info",
	Run: func(cmd *cobra.Command, args []string) {
		vInfo := config.VersionInfo()
		fmt.Printf("Version: %s\n", vInfo.Version())
		fmt.Printf("GitCommit: %s\n", vInfo.Git)
	},
}
