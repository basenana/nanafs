package main

import (
	"flag"
	"github.com/basenana/nanafs/cmd/apps/fs"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils"
	"time"
)

func init() {
	flag.StringVar(&config.FilePath, "config", "", "nanafs config file")
}

func main() {
	flag.Parse()

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
	stop := utils.HandleTerminalSignal()
	run(ctrl, cfg, stop)
}

func run(ctrl controller.Controller, cfg config.Config, stopCh chan struct{}) {
	shutdown := make(chan struct{})
	go func() {
		<-stopCh
		time.Sleep(time.Second * 5)
		close(shutdown)
	}()

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
	<-shutdown
}
