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

package fileop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"go.uber.org/zap"
)

const (
	pluginName    = "fileop"
	pluginVersion = "1.0"
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
}

type FileOpPlugin struct {
	logger *zap.SugaredLogger
}

func NewFileOpPlugin(ps types.PluginCall) types.Plugin {
	return &FileOpPlugin{
		logger: logger.NewPluginLogger(pluginName, ps.JobID),
	}
}

func (p *FileOpPlugin) Name() string {
	return pluginName
}

func (p *FileOpPlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *FileOpPlugin) Version() string {
	return pluginVersion
}

func (p *FileOpPlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	action := api.GetStringParameter("action", request, "")
	src := api.GetStringParameter("src", request, "")
	dest := api.GetStringParameter("dest", request, "")

	if action == "" {
		return api.NewFailedResponse("action is required"), nil
	}

	if src == "" {
		return api.NewFailedResponse("src is required"), nil
	}

	p.logger.Infow("fileop started", "action", action, "src", src, "dest", dest)

	var err error
	switch action {
	case "cp":
		err = copyFile(src, dest)
	case "mv":
		err = os.Rename(src, dest)
	case "rm":
		err = os.Remove(src)
	case "rename":
		if dest == "" {
			return api.NewFailedResponse("dest is required for rename action"), nil
		}
		err = os.Rename(src, dest)
	default:
		return api.NewFailedResponse(fmt.Sprintf("unknown action: %s", action)), nil
	}

	if err != nil {
		p.logger.Warnw("fileop failed", "action", action, "src", src, "dest", dest, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("fileop completed", "action", action, "src", src, "dest", dest)
	return api.NewResponse(), nil
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file failed: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source file failed: %w", err)
	}

	destFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create dest file failed: %w", err)
	}
	defer destFile.Close()

	_, err = Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("copy file failed: %w", err)
	}

	return nil
}

func Copy(dst, src *os.File) (int64, error) {
	return CopyBuffer(dst, src, make([]byte, 32*1024))
}

func CopyBuffer(dst, src *os.File, buf []byte) (int64, error) {
	var written int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, fmt.Errorf("short write: %d/%d", nr, nw)
			}
		}
		if er != nil {
			if er.Error() != "EOF" {
				return written, er
			}
			break
		}
	}
	return written, nil
}

func ResolvePath(path string, workingPath string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(workingPath, path), nil
}
