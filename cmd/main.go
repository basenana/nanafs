package main

import (
	"github.com/basenana/nanafs/cmd/apps"
	"github.com/basenana/nanafs/utils/logger"
	"math/rand"
	"time"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	logger.InitLogger()
	defer logger.Sync()

	if err := apps.RootCmd.Execute(); err != nil {
		panic(err)
	}
}
