package friday

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/basenana/nanafs/pkg/types"
)

// formatFileInfo formats file info for JSON response
type fileInfo struct {
	Name     string `json:"name"`
	Size     string `json:"size"`
	Modified string `json:"modified"`
	IsDir    bool   `json:"is_dir"`
}

// statInfo formats entry info for JSON response
type statInfo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Mode     uint32 `json:"mode"`
	Modified string `json:"modified"`
	IsDir    bool   `json:"is_dir"`
	Kind     string `json:"kind"`
}

// entryToStatInfo converts an entry to statInfo
func entryToStatInfo(entry *types.Entry) statInfo {
	return statInfo{
		ID:       entry.ID,
		Name:     entry.Name,
		Size:     entry.Size,
		Modified: entry.ChangedAt.Format("2006-01-02 15:04:05"),
		IsDir:    entry.IsGroup,
		Kind:     string(entry.Kind),
	}
}

// isNotFoundError checks if the error is a not found error
func isNotFoundError(err error) bool {
	return errors.Is(err, types.ErrNotFound)
}

// splitParentAndName splits a path into parent URI and name
func splitParentAndName(inputPath string) (parentURI, name string) {
	if inputPath == "" {
		return "/", ""
	}

	// Remove leading slash for consistent handling
	cleanPath := strings.TrimPrefix(inputPath, "/")

	// Find last slash
	idx := strings.LastIndex(cleanPath, "/")
	if idx < 0 {
		// No slash, file is in root
		return "/", cleanPath
	}

	return "/" + cleanPath[:idx], cleanPath[idx+1:]
}

func formatSize(size int64) string {
	const (
		unit  = 1024
		units = "KMGTPE"
	)
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), units[exp])
}

// parsePath parses the input path and returns the cleaned entry path.
// Supported formats:
//   - "/path/to/file" - path relative to namespace root
//   - "path/to/file" - path relative to namespace root
func parsePath(inputPath string) (string, error) {
	if !strings.HasPrefix(inputPath, "/") {
		inputPath = path.Join("/", inputPath)
	}
	return sanitizePath(inputPath)
}

// sanitizePath cleans and validates the path to prevent path traversal attacks.
// It removes ".." components and ensures the path stays within the namespace root.
func sanitizePath(inputPath string) (string, error) {
	if inputPath == "" || inputPath == "." {
		return "", nil
	}

	// Split path into components and validate each one
	parts := strings.Split(strings.ReplaceAll(inputPath, "\\", "/"), "/")
	var cleanParts []string

	for _, p := range parts {
		// Skip empty parts (from leading/trailing/consecutive slashes)
		if p == "" {
			continue
		}

		// Reject path traversal attempts
		if p == ".." {
			return "", fmt.Errorf("path traversal not allowed: %s", inputPath)
		}

		// Reject paths starting with "/" (absolute paths)
		if strings.HasPrefix(p, "/") {
			return "", fmt.Errorf("absolute path not allowed: %s", inputPath)
		}

		cleanParts = append(cleanParts, p)
	}

	// Reconstruct the clean path
	cleanPath := path.Join(cleanParts...)

	// Ensure the path doesn't escape the namespace root
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", fmt.Errorf("path escapes namespace root: %s", inputPath)
	}

	return cleanPath, nil
}
