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

package checksum

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"github.com/basenana/plugin/utils"
	"go.uber.org/zap"
)

const (
	pluginName    = "checksum"
	pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
	InitParameters: []types.ParameterSpec{
		{
			Name:        "algorithm",
			Required:    false,
			Default:     "md5",
			Description: "Hash algorithm: md5, sha256",
			Options:     []string{"md5", "sha256"},
		},
	},
	Parameters: []types.ParameterSpec{
		{
			Name:        "file_path",
			Required:    true,
			Description: "Path to file",
		},
	},
}

type ChecksumPlugin struct {
	algorithm string
	logger    *zap.SugaredLogger
	fileRoot  *utils.FileAccess
}

func NewChecksumPlugin(ps types.PluginCall) types.Plugin {
	algorithm := ps.Params["algorithm"]
	if algorithm == "" {
		algorithm = "md5"
	}
	return &ChecksumPlugin{
		logger:    logger.NewPluginLogger(pluginName, ps.JobID),
		algorithm: algorithm,
		fileRoot:  utils.NewFileAccess(ps.WorkingPath),
	}
}

func (p *ChecksumPlugin) Name() string {
	return pluginName
}

func (p *ChecksumPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *ChecksumPlugin) Version() string {
	return pluginVersion
}

func (p *ChecksumPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	filePath := api.GetStringParameter("file_path", request, "")

	if filePath == "" {
		return api.NewFailedResponse("file_path is required"), nil
	}

	p.logger.Infow("checksum started", "file_path", filePath, "algorithm", p.algorithm)

	hash, err := p.computeHash(filePath)
	if err != nil {
		p.logger.Warnw("compute hash failed", "file_path", filePath, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("checksum completed", "file_path", filePath, "hash", hash)

	results := map[string]any{
		"hash": hash,
	}

	return api.NewResponseWithResult(results), nil
}

func (p *ChecksumPlugin) computeHash(filePath string) (string, error) {
	file, err := p.fileRoot.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	var hash interface {
		Write(p []byte) (n int, err error)
		Sum(b []byte) []byte
	}

	switch p.algorithm {
	case "md5":
		hash = md5.New()
	case "sha256":
		hash = sha256.New()
	default:
		return "", fmt.Errorf("unsupported algorithm: %s (supported: md5, sha256)", p.algorithm)
	}

	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("compute hash failed: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
