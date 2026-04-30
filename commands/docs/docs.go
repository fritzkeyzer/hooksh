// Package docs lists documentation files for context bootstrapping.
package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fritzkeyzer/hooksh/internal/fsutil"
	"github.com/fritzkeyzer/hooksh/internal/limitutil"
	"github.com/fritzkeyzer/hooksh/internal/xmlutil"
)

type Options struct {
	Kind  string
	Limit int
}

func Run(opts Options) {
	ext := extensionForKind(opts.Kind)
	files := make([]string, 0)

	_ = fsutil.WalkFiles(".", func(path string, info os.FileInfo) error {
		if strings.EqualFold(filepath.Ext(path), ext) {
			files = append(files, fsutil.NormalizePath(path))
		}
		return nil
	})

	files = fsutil.Sorted(files)
	attrKey, attrValue := limitutil.CountOrLimit(len(files), opts.Limit)
	emitCount := limitutil.EffectiveLimit(len(files), opts.Limit)

	fmt.Println(xmlutil.TagOpen("docs",
		xmlutil.Attr{Key: "kind", Value: opts.Kind},
		xmlutil.Attr{Key: attrKey, Value: fmt.Sprintf("%d", attrValue)},
	))

	for _, path := range files[:emitCount] {
		fmt.Println(path)
	}

	fmt.Printf("%s\n\n", xmlutil.TagClose("docs"))
}

func extensionForKind(kind string) string {
	switch strings.ToLower(kind) {
	case "md", "markdown":
		return ".md"
	default:
		if strings.HasPrefix(kind, ".") {
			return strings.ToLower(kind)
		}
		return "." + strings.ToLower(kind)
	}
}
