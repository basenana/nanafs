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

package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/basenana/plugin/api"
	"github.com/basenana/plugin/logger"
	"github.com/basenana/plugin/types"
	"go.uber.org/zap"
)

const (
	pluginName    = "archive"
	pluginVersion = "1.0"
	unixDIR       = syscall.S_IFDIR
)

var PluginSpec = types.PluginSpec{
	Name:    pluginName,
	Version: pluginVersion,
	Type:    types.TypeProcess,
}

type ArchivePlugin struct {
	logger *zap.SugaredLogger
}

func NewArchivePlugin(ps types.PluginCall) types.Plugin {
	return &ArchivePlugin{
		logger: logger.NewPluginLogger(pluginName, ps.JobID),
	}
}

func (p *ArchivePlugin) Name() string {
	return pluginName
}

func (p *ArchivePlugin) Type() types.PluginType {
	return types.TypeProcess
}

func (p *ArchivePlugin) Version() string {
	return pluginVersion
}

func (p *ArchivePlugin) Run(ctx context.Context, request *api.Request) (*api.Response, error) {
	action := api.GetStringParameter("action", request, "extract")
	format := api.GetStringParameter("format", request, "")

	p.logger.Infow("archive plugin started", "action", action, "format", format)

	if action == "compress" {
		return p.runCompress(request, format)
	}
	return p.runExtract(request, format)
}

func (p *ArchivePlugin) runExtract(request *api.Request, format string) (*api.Response, error) {
	filePath := api.GetStringParameter("file_path", request, "")
	destPath := api.GetStringParameter("dest_path", request, "")

	if filePath == "" {
		return api.NewFailedResponse("file_path is required"), nil
	}

	if format == "" {
		return api.NewFailedResponse("format is required"), nil
	}

	if destPath == "" {
		destPath = "."
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return api.NewFailedResponse(fmt.Sprintf("create dest directory failed: %v", err)), nil
	}

	var err error
	switch format {
	case "zip":
		err = extractZip(filePath, destPath)
	case "tar":
		err = extractTar(filePath, destPath)
	case "gzip":
		err = extractGzip(filePath, destPath)
	default:
		return api.NewFailedResponse(fmt.Sprintf("unsupported format: %s (supported: zip, tar, gzip)", format)), nil
	}

	if err != nil {
		p.logger.Warnw("extract failed", "file_path", filePath, "dest_path", destPath, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	p.logger.Infow("extract completed", "file_path", filePath, "dest_path", destPath)
	return api.NewResponse(), nil
}

func (p *ArchivePlugin) runCompress(request *api.Request, format string) (*api.Response, error) {
	sourcePath := api.GetStringParameter("source_path", request, "")
	archiveName := api.GetStringParameter("archive_name", request, "")
	destPath := api.GetStringParameter("dest_path", request, "")

	if sourcePath == "" {
		return api.NewFailedResponse("source_path is required for compression"), nil
	}

	if format == "" {
		return api.NewFailedResponse("format is required"), nil
	}

	if destPath == "" {
		destPath = "."
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return api.NewFailedResponse(fmt.Sprintf("create dest directory failed: %v", err)), nil
	}

	// Generate archive name if not provided
	if archiveName == "" {
		archiveName = generateArchiveName(sourcePath, format)
	}
	archivePath := filepath.Join(destPath, archiveName)

	var err error
	switch format {
	case "zip":
		err = createZip(sourcePath, archivePath)
	case "tar":
		err = createTar(sourcePath, archivePath)
	case "gzip":
		err = createGzip(sourcePath, archivePath)
	default:
		return api.NewFailedResponse(fmt.Sprintf("unsupported format: %s (supported: zip, tar, gzip)", format)), nil
	}

	if err != nil {
		p.logger.Warnw("compress failed", "source_path", sourcePath, "error", err)
		return api.NewFailedResponse(err.Error()), nil
	}

	// Return archive info
	info, err := os.Stat(archivePath)
	if err != nil {
		p.logger.Infow("compress completed", "archive_path", archivePath)
		return api.NewResponse(), nil
	}

	p.logger.Infow("compress completed", "archive_path", archivePath, "size", info.Size())
	return api.NewResponseWithResult(map[string]any{
		"file_path": archivePath,
		"size":      info.Size(),
	}), nil
}

func generateArchiveName(sourcePath, format string) string {
	baseName := filepath.Base(sourcePath)
	switch format {
	case "zip":
		if !strings.HasSuffix(baseName, ".zip") {
			return baseName + ".zip"
		}
	case "tar":
		if !strings.HasSuffix(baseName, ".tar.gz") && !strings.HasSuffix(baseName, ".tgz") {
			return baseName + ".tar.gz"
		}
	case "gzip":
		if !strings.HasSuffix(baseName, ".gz") {
			return baseName + ".gz"
		}
	}
	return baseName
}

func extractZip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open zip file failed: %w", err)
	}
	defer reader.Close()

	// Ensure base destination directory exists
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create dest directory failed: %w", err)
	}

	for _, file := range reader.File {
		path := filepath.Join(dest, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return fmt.Errorf("create directory failed: %w", err)
			}
			continue
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("create parent directory failed: %w", err)
		}

		// Use 0644 permissions to ensure write access
		destFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("create file failed: %w", err)
		}

		srcFile, err := file.Open()
		if err != nil {
			destFile.Close()
			return fmt.Errorf("open zip entry failed: %w", err)
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()

		if err != nil {
			return fmt.Errorf("extract file failed: %w", err)
		}
	}

	return nil
}

func extractTar(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open tar file failed: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader failed: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header failed: %w", err)
		}

		path := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create directory failed: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return fmt.Errorf("create parent directory failed: %w", err)
			}

			destFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file failed: %w", err)
			}

			_, err = io.Copy(destFile, tarReader)
			destFile.Close()

			if err != nil {
				return fmt.Errorf("extract file failed: %w", err)
			}
		}
	}

	return nil
}

func extractGzip(src, dest string) error {
	// For gzip, we extract to the same directory with the .gz extension removed
	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open gzip file failed: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader failed: %w", err)
	}
	defer gzipReader.Close()

	// Determine output filename (remove .gz extension)
	baseName := filepath.Base(src)
	if strings.HasSuffix(baseName, ".tgz") {
		baseName = baseName[:len(baseName)-3] + "tar"
	} else if strings.HasSuffix(baseName, ".gz") {
		baseName = baseName[:len(baseName)-3]
	}

	outputPath := filepath.Join(dest, baseName)

	// Ensure destination directory exists
	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create dest directory failed: %w", err)
	}

	destFile, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create output file failed: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, gzipReader)
	if err != nil {
		return fmt.Errorf("extract gzip failed: %w", err)
	}

	return nil
}

// Compression functions

func createZip(src, dest string) error {
	// Determine if src is file or directory
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source failed: %w", err)
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create zip file failed: %w", err)
	}
	defer destFile.Close()

	zipWriter := zip.NewWriter(destFile)
	defer zipWriter.Close()

	if info.IsDir() {
		return walkAndZip(src, "", zipWriter)
	}
	return addFileToZip(src, filepath.Base(src), zipWriter)
}

func walkAndZip(root, baseDir string, zw *zip.Writer) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if baseDir != "" {
			relPath = filepath.Join(baseDir, relPath)
		}

		if info.IsDir() {
			// Add directory entry to zip with proper Unix permissions
			relPath = relPath + "/"
			header := &zip.FileHeader{
				Name:          relPath,
				Method:        zip.Deflate,
				ExternalAttrs: (uint32(info.Mode()) << 16) | unixDIR,
			}
			_, err := zw.CreateHeader(header)
			if err != nil {
				return fmt.Errorf("create zip directory entry failed: %w", err)
			}
			return nil
		}

		return addFileToZip(path, relPath, zw)
	})
}

func addFileToZip(filePath, zipPath string, zw *zip.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat file failed: %w", err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("create zip header failed: %w", err)
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry failed: %w", err)
	}

	_, err = io.Copy(writer, file)
	return err
}

func createTar(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source failed: %w", err)
	}

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create tar file failed: %w", err)
	}
	defer destFile.Close()

	gzipWriter := gzip.NewWriter(destFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	if info.IsDir() {
		return walkAndTar(src, "", tarWriter)
	}
	return addFileToTar(src, filepath.Base(src), tarWriter)
}

func walkAndTar(root, baseDir string, tw *tar.Writer) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if baseDir != "" {
			relPath = filepath.Join(baseDir, relPath)
		}

		if info.IsDir() {
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = relPath + "/"
			if err := tw.WriteHeader(header); err != nil {
				return fmt.Errorf("write tar header failed: %w", err)
			}
			return nil
		}

		return addFileToTar(path, relPath, tw)
	})
}

func addFileToTar(filePath, tarPath string, tw *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("create tar header failed: %w", err)
	}
	header.Name = tarPath

	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header failed: %w", err)
	}

	_, err = io.Copy(tw, file)
	return err
}

func createGzip(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source failed: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("gzip compression only supports single files, not directories")
	}

	file, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create gzip file failed: %w", err)
	}
	defer destFile.Close()

	gzipWriter := gzip.NewWriter(destFile)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, file)
	return err
}
