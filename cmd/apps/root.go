package apps

import (
	"github.com/basenana/nanafs/cmd/apps/apis"
	configapp "github.com/basenana/nanafs/cmd/apps/config"
	"github.com/basenana/nanafs/cmd/apps/fs"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/files"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/pkg/workflow"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/spf13/cobra"
	"path"
	"time"
)

func init() {
	RootCmd.AddCommand(daemonCmd)
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
	Short: "nanafs daemon run",
	PreRun: func(cmd *cobra.Command, args []string) {

	},
	Run: func(cmd *cobra.Command, args []string) {
		loader := config.NewConfigLoader()
		cfg, err := loader.GetConfig()
		if err != nil {
			panic(err)
		}

		meta, err := storage.NewMetaStorage(cfg.Meta.Type, cfg.Meta)
		if err != nil {
			panic(err)
		}

		if len(cfg.Storages) != 1 {
			panic("storage must config one")
		}

		sto, err := storage.NewStorage(cfg.Storages[0].ID, cfg.Storages[0])
		if err != nil {
			panic(err)
		}

		ctrl := controller.New(loader, meta, sto)
		wfMgr, err := workflow.NewWorkflowManager(ctrl)
		if err != nil {
			panic(err)
		}
		go wfMgr.Run()

		stop := utils.HandleTerminalSignal()
		files.InitFileIoChain(cfg, sto, stop)
		run(ctrl, cfg, stop)
	},
}

func run(ctrl controller.Controller, cfg config.Config, stopCh chan struct{}) {
	log := logger.NewLogger("nanafs")
	log.Info("starting")
	shutdown := make(chan struct{})
	go func() {
		<-stopCh
		log.Info("shutdown after 5s")
		time.Sleep(time.Second * 5)
		close(shutdown)
	}()

	if cfg.ApiConfig.Enable {
		s, err := apis.NewApiServer(ctrl, cfg.ApiConfig)
		if err != nil {
			log.Panicw("init http server failed", "err", err.Error())
		}
		go s.Run(stopCh)
	}

	if cfg.FsConfig.Enable {
		fsServer, err := fs.NewNanaFsRoot(cfg.FsConfig, ctrl)
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
	log.Info("stopped")
}
