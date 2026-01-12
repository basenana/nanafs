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

package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/basenana/nanafs/cmd/apps"
	"github.com/basenana/nanafs/utils/logger"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	logger.InitLogger()
	defer logger.Sync()

	if err := apps.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
