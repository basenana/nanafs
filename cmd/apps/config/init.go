package config

import (
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/spf13/cobra"
	"os"
)

var WorkSpace string

func init() {
	initCmd.Flags().StringVar(&WorkSpace, "workspace", config.LocalUserPath(), "nanafs workspace")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "generate local configuration",
	Run: func(cmd *cobra.Command, args []string) {
		initDefaultConfig()
	},
}

func initDefaultConfig() {
	fmt.Printf("Workspace: %s\n", WorkSpace)
	if err := mkdir(WorkSpace); err != nil {
		fmt.Printf("init workspace failed: %s\n", err.Error())
		return
	}

	dataDir := localDataDirPath(WorkSpace)
	fmt.Printf("Workspace Data Dir: %s\n", dataDir)
	if err := mkdir(dataDir); err != nil {
		fmt.Printf("init workspace data dir failed: %s\n", err.Error())
		return
	}

	dbFile := localDbFilePath(WorkSpace)
	fmt.Printf("Workspace Database File: %s\n", dbFile)

	conf := config.Config{
		Meta: config.Meta{
			Type: storage.SqliteMeta,
			Path: dbFile,
		},
		Storages: []config.Storage{
			{
				ID:       storage.LocalStorage,
				LocalDir: dataDir,
			},
		},
		Debug: false,
		ApiConfig: config.Api{
			Enable: true,
			Host:   "127.0.0.1",
			Port:   8081,
			Pprof:  true,
		},
		FsConfig: config.Fs{
			Enable:      false,
			RootPath:    "/your/path/to/mount",
			DisplayName: "nanafs",
		},
	}
	configPath := localConfigFilePath(WorkSpace)
	fmt.Printf("Workspace Config: %s\n", configPath)
	raw, _ := json.MarshalIndent(conf, "", "    ")
	if err := os.WriteFile(configPath, raw, 0755); err != nil {
		fmt.Printf("wirteback config file failed: %s\n", err.Error())
		return
	}
	fmt.Println("Generate local configuration succeed")
}

func mkdir(path string) error {
	d, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err != nil && os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}

	if d.IsDir() {
		return nil
	}

	return fmt.Errorf("%s not dir", path)
}
