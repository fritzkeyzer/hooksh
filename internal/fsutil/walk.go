package fsutil

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func WalkFiles(root string, visit func(path string, info os.FileInfo) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if path != "." && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		return visit(path, info)
	})
}

func NormalizePath(path string) string {
	normalized := filepath.ToSlash(path)
	if normalized == "." {
		return normalized
	}
	if strings.HasPrefix(normalized, "./") {
		return normalized
	}
	return "./" + normalized
}

func Sorted(values []string) []string {
	cloned := append([]string(nil), values...)
	sort.Strings(cloned)
	return cloned
}
