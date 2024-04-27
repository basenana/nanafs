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
	Short: "Viewing and modifying local configurations",
	Run: func(cmd *cobra.Command, args []string) {
		configPath := localConfigFilePath(WorkSpace)
		fmt.Printf("Workspace Config: %s\n\n", configPath)

		raw, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("read config failed: %s\n", err.Error())
			fmt.Println("Generate local configuration with 'nanafs config init'")
			return
		}

		cfg := config.Bootstrap{}
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
