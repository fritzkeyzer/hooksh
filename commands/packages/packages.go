// Package packages lists project packages with their package docs.
package packages

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fritzkeyzer/hooksh/internal/fsutil"
	"github.com/fritzkeyzer/hooksh/internal/limitutil"
	"github.com/fritzkeyzer/hooksh/internal/xmlutil"
)

type Options struct {
	Kind  string
	Limit int
	Order string
}

type item struct {
	name  string
	doc   string
	depth int
}

func Run(opts Options) {
	if opts.Kind != "go-package-doc" {
		fmt.Println(xmlutil.TagOpen("packages",
			xmlutil.Attr{Key: "kind", Value: opts.Kind},
			xmlutil.Attr{Key: "count", Value: "0"},
		))
		fmt.Printf("%s\n\n", xmlutil.TagClose("packages"))
		return
	}

	packages := collect(opts.Order)
	attrKey, attrValue := limitutil.CountOrLimit(len(packages), opts.Limit)
	emitCount := limitutil.EffectiveLimit(len(packages), opts.Limit)

	attrs := []xmlutil.Attr{
		{Key: "kind", Value: opts.Kind},
		{Key: attrKey, Value: fmt.Sprintf("%d", attrValue)},
	}
	if opts.Order != "depth" {
		attrs = append(attrs, xmlutil.Attr{Key: "order", Value: opts.Order})
	}
	fmt.Println(xmlutil.TagOpen("packages", attrs...))

	for _, pkg := range packages[:emitCount] {
		fmt.Printf("- %s: %s\n", pkg.name, pkg.doc)
	}

	fmt.Printf("%s\n\n", xmlutil.TagClose("packages"))
}

func collect(order string) []item {
	fset := token.NewFileSet()
	byDir := make(map[string]item)

	_ = fsutil.WalkFiles(".", func(path string, info os.FileInfo) error {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly|parser.ParseComments)
		if err != nil || node == nil || node.Name == nil {
			return nil
		}

		dir := filepath.Dir(path)
		relDir := strings.TrimPrefix(filepath.ToSlash(dir), "./")
		if relDir == "." {
			relDir = ""
		}

		entry, exists := byDir[relDir]
		if !exists {
			entry = item{
				name:  node.Name.Name,
				doc:   "",
				depth: strings.Count(relDir, "/"),
			}
		}

		if entry.doc == "" && node.Doc != nil {
			doc := strings.TrimSpace(node.Doc.Text())
			doc = strings.ReplaceAll(doc, "\n", " ")
			doc = strings.Join(strings.Fields(doc), " ")
			entry.doc = normalizeDoc(entry.name, doc)
		}

		byDir[relDir] = entry
		return nil
	})

	packages := make([]item, 0, len(byDir))
	for _, value := range byDir {
		if value.doc == "" {
			continue
		}
		packages = append(packages, value)
	}

	sort.Slice(packages, func(i, j int) bool {
		if order == "name" {
			return packages[i].name < packages[j].name
		}
		if packages[i].depth != packages[j].depth {
			return packages[i].depth < packages[j].depth
		}
		return packages[i].name < packages[j].name
	})

	return packages
}

func normalizeDoc(packageName, doc string) string {
	prefix := fmt.Sprintf("Package %s ", packageName)
	if strings.HasPrefix(doc, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(doc, prefix))
	}
	return doc
}
