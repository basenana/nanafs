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
	"fmt"
	"strconv"
	"strings"
)

var (
	gitTag    string
	gitCommit string
)

type Version struct {
	Major   int    `json:"major"`
	Minor   int    `json:"minor"`
	Patch   int    `json:"patch"`
	Release string `json:"release"`
	Git     string `json:"git"`
}

func (v Version) Version() string {
	releaseInfo := ""
	if v.Release != "" {
		releaseInfo = "-" + v.Release
	}
	return fmt.Sprintf("v%d.%d.%d%s", v.Major, v.Minor, v.Patch, releaseInfo)
}

func VersionInfo() Version {
	versionInfo := Version{}
	if strings.HasPrefix(gitTag, "v") {
		gitTag = strings.TrimPrefix(gitTag, "v")
	}
	infoParts := strings.Split(gitTag, "-")

	versionStr := infoParts[0]
	versionParts := strings.Split(versionStr, ".")

	versionInfo.Major, _ = strconv.Atoi(versionParts[0])

	if len(versionParts) > 1 {
		versionInfo.Minor, _ = strconv.Atoi(versionParts[1])
	}

	if len(versionParts) > 2 {
		versionInfo.Patch, _ = strconv.Atoi(versionParts[2])
	}

	if len(infoParts) > 1 {
		versionInfo.Release = infoParts[1]
	}

	versionInfo.Git = gitCommit
	return versionInfo
}
