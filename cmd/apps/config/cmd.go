package config

import (
	"encoding/json"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/spf13/cobra"
	"os"
)

var WorkSpace string

func init() {
	RunCmd.AddCommand(initCmd)
	RunCmd.AddCommand(setCmd)
	RunCmd.PersistentFlags().StringVar(&WorkSpace, "workspace", config.LocalUserPath(), "nanafs workspace")
}

var RunCmd = &cobra.Command{
	Use:   "config",
	Short: "nanafs config management",
	Run: func(cmd *cobra.Command, args []string) {
		configPath := localConfigFilePath(WorkSpace)
		fmt.Printf("Workspace Config: %s\n\n", configPath)

		raw, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("read config failed: %s\n", err.Error())
			fmt.Println("Generate local configuration with 'nanafs config init'")
			return
		}

		cfg := config.Config{}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			fmt.Printf("load config failed: %s\n", err.Error())
			return
		}

		raw, err = json.MarshalIndent(cfg, "", "    ")
		if err != nil {
			fmt.Printf("marshal config failed: %s\n", err.Error())
			return
		}
		fmt.Println(string(raw))
	},
}
