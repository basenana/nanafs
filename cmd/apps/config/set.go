package config

import (
	"fmt"
	"github.com/spf13/cobra"
)

var setCmd = &cobra.Command{
	Use:   "set",
	Short: "edit local configuration",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("do nothing")
	},
}
