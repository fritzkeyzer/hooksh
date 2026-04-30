// Package go_lyze analyzes a Go codebase via AST parsing and returns
// structured data about packages, their exports, and cross-package call relationships.
package go_lyze

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fritzkeyzer/hooksh/internal/fsutil"
)

// Options configures the analysis.
type Options struct {
	RootDir  string   // directory to analyze (default ".")
	TopPkgs  []string // dirs to start BFS from; if empty, walk everything
	MaxDepth int      // maximum directory depth; 0 = unlimited (walk mode only)
}

// Result is the complete structured output of an analysis.
type Result struct {
	Packages      []Package      `json:"packages"`
	Relationships []Relationship `json:"relationships"`
}

// Package represents a single Go package with its metadata and exports.
type Package struct {
	ImportPath   string   `json:"import_path"`
	Dir          string   `json:"dir"`
	Name         string   `json:"name"`
	Doc          string   `json:"doc,omitempty"`
	Exports      []string `json:"exports"`      // "pkgname.FuncName"
	Dependencies []string `json:"dependencies"` // import paths of local packages this package imports
}

// Relationship is a direct cross-package call from an exported function in one
// package to an exported function in another.
type Relationship struct {
	From     string   `json:"from"`      // "import/path/a.FuncA"
	To       string   `json:"to"`        // "import/path/b.FuncB"
	CallPath []string `json:"call_path"` // intermediate function keys
}

type pkgData struct {
	name    string
	dir     string
	doc     string
	imports map[string]struct{} // local import paths this package uses
}

type funcData struct {
	pkgPath  string
	name     string
	exported bool
	calls    map[string]struct{} // function keys this function calls (local only)
}

// Analyze runs the full analysis and returns a Result.
func Analyze(opts Options) (Result, error) {
	root := opts.RootDir
	if root == "" {
		root = "."
	}

	modulePath := readModulePath(filepath.Join(root, "go.mod"))

	fset := token.NewFileSet()
	pkgs := make(map[string]*pkgData)
	funcs := make(map[string]*funcData)

	if len(opts.TopPkgs) > 0 {
		// BFS from the specified top packages, following local imports.
		// Each queue entry carries its BFS depth so MaxDepth can be enforced.
		type bfsItem struct {
			dir   string
			depth int
		}
		visited := make(map[string]struct{})
		queue := make([]bfsItem, 0, len(opts.TopPkgs))
		for _, d := range opts.TopPkgs {
			queue = append(queue, bfsItem{normDir(d), 0})
		}

		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]
			if _, seen := visited[item.dir]; seen {
				continue
			}
			visited[item.dir] = struct{}{}

			imported := parseDir(fset, root, item.dir, modulePath, pkgs, funcs)
			if opts.MaxDepth > 0 && item.depth >= opts.MaxDepth {
				continue
			}
			for _, ip := range imported {
				rel := strings.TrimPrefix(strings.TrimPrefix(ip, modulePath), "/")
				nextDir := filepath.ToSlash(filepath.Join(root, rel))
				if _, seen := visited[nextDir]; !seen {
					queue = append(queue, bfsItem{nextDir, item.depth + 1})
				}
			}
		}
	} else {
		// walk the entire tree
		rootDepth := strings.Count(filepath.ToSlash(root), "/")
		_ = fsutil.WalkFiles(root, func(path string, info os.FileInfo) error {
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if opts.MaxDepth > 0 {
				depth := strings.Count(filepath.ToSlash(path), "/") - rootDepth
				if depth > opts.MaxDepth {
					return nil
				}
			}
			dir := filepath.ToSlash(filepath.Dir(path))
			parseDir(fset, root, dir, modulePath, pkgs, funcs)
			return nil
		})
	}

	// fallback doc from markdown files
	for importPath, pkg := range pkgs {
		if pkg.doc == "" {
			pkg.doc = findMarkdownDoc(root, importPath, modulePath, pkg.name)
		}
	}

	// build sorted package list
	importPaths := make([]string, 0, len(pkgs))
	for ip := range pkgs {
		importPaths = append(importPaths, ip)
	}
	sort.Strings(importPaths)

	packages := make([]Package, 0, len(pkgs))
	for _, ip := range importPaths {
		pkg := pkgs[ip]

		exports := make([]string, 0)
		for _, fd := range funcs {
			if fd.pkgPath == ip && fd.exported {
				exports = append(exports, pkg.name+"."+fd.name)
			}
		}
		sort.Strings(exports)

		deps := make([]string, 0, len(pkg.imports))
		for dep := range pkg.imports {
			deps = append(deps, dep)
		}
		sort.Strings(deps)

		packages = append(packages, Package{
			ImportPath:   ip,
			Dir:          pkg.dir,
			Name:         pkg.name,
			Doc:          pkg.doc,
			Exports:      exports,
			Dependencies: deps,
		})
	}

	// build relationships: exported fn in A → exported fn in B (direct calls)
	var relationships []Relationship
	for key, fd := range funcs {
		if !fd.exported {
			continue
		}
		fromPkg := pkgs[fd.pkgPath]
		if fromPkg == nil {
			continue
		}
		fromLabel := fromPkg.name + "." + fd.name

		for calledKey := range fd.calls {
			calledFd, exists := funcs[calledKey]
			if !exists || !calledFd.exported {
				continue
			}
			if calledFd.pkgPath == fd.pkgPath {
				continue
			}
			toPkg := pkgs[calledFd.pkgPath]
			if toPkg == nil {
				continue
			}
			toLabel := toPkg.name + "." + calledFd.name

			relationships = append(relationships, Relationship{
				From:     key,
				To:       calledKey,
				CallPath: []string{fromLabel, toLabel},
			})
		}
	}

	sort.Slice(relationships, func(i, j int) bool {
		if relationships[i].From != relationships[j].From {
			return relationships[i].From < relationships[j].From
		}
		return relationships[i].To < relationships[j].To
	})

	return Result{
		Packages:      packages,
		Relationships: relationships,
	}, nil
}

// parseDir parses all non-test .go files in dir, populates pkgs and funcs,
// and returns the set of local import paths discovered.
func parseDir(fset *token.FileSet, root, dir, modulePath string, pkgs map[string]*pkgData, funcs map[string]*funcData) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var discovered []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		path := filepath.ToSlash(filepath.Join(dir, e.Name()))
		node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		// derive import path from dir relative to root
		rel, _ := filepath.Rel(root, dir)
		rel = filepath.ToSlash(rel)
		importPath := modulePath
		if rel != "" && rel != "." {
			importPath = modulePath + "/" + rel
		}

		pkg := pkgs[importPath]
		if pkg == nil {
			displayDir := "./" + rel
			if rel == "" || rel == "." {
				displayDir = "."
			}
			pkg = &pkgData{
				name:    node.Name.Name,
				dir:     displayDir,
				imports: make(map[string]struct{}),
			}
			pkgs[importPath] = pkg
		}

		if pkg.doc == "" && node.Doc != nil {
			pkg.doc = strings.TrimSpace(node.Doc.Text())
		}

		localImports := localImportAliases(node, modulePath)

		for _, imp := range node.Imports {
			ip, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if ip == modulePath || strings.HasPrefix(ip, modulePath+"/") {
				if _, seen := pkg.imports[ip]; !seen {
					pkg.imports[ip] = struct{}{}
					discovered = append(discovered, ip)
				}
			}
		}

		for _, decl := range node.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}

			key := importPath + "." + fn.Name.Name
			fd := funcs[key]
			if fd == nil {
				fd = &funcData{
					pkgPath:  importPath,
					name:     fn.Name.Name,
					exported: ast.IsExported(fn.Name.Name) || (fn.Name.Name == "main" && node.Name.Name == "main"),
					calls:    make(map[string]struct{}),
				}
				funcs[key] = fd
			}

			for _, called := range collectCalls(fn, importPath, localImports) {
				fd.calls[called] = struct{}{}
			}
		}
	}

	return discovered
}

// normDir normalises a user-supplied package dir to a slash path without trailing slash.
func normDir(d string) string {
	d = filepath.ToSlash(strings.TrimSuffix(d, "/"))
	if strings.HasPrefix(d, "./") {
		d = d[2:]
	}
	return d
}

func readModulePath(goModPath string) string {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func localImportAliases(file *ast.File, modulePath string) map[string]string {
	aliases := make(map[string]string)
	for _, imp := range file.Imports {
		importedPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if importedPath != modulePath && !strings.HasPrefix(importedPath, modulePath+"/") {
			continue
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if alias == "" {
			alias = filepath.Base(importedPath)
		}
		if alias == "_" || alias == "." {
			continue
		}
		aliases[alias] = importedPath
	}
	return aliases
}

func collectCalls(fn *ast.FuncDecl, packagePath string, localImports map[string]string) []string {
	if fn.Body == nil {
		return nil
	}
	called := make(map[string]struct{})
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch target := call.Fun.(type) {
		case *ast.Ident:
			called[packagePath+"."+target.Name] = struct{}{}
		case *ast.SelectorExpr:
			pkgIdent, ok := target.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath, exists := localImports[pkgIdent.Name]
			if !exists {
				return true
			}
			called[importPath+"."+target.Sel.Name] = struct{}{}
		}
		return true
	})
	result := make([]string, 0, len(called))
	for key := range called {
		result = append(result, key)
	}
	return result
}

func findMarkdownDoc(rootDir, importPath, modulePath, pkgName string) string {
	rel := strings.TrimPrefix(strings.TrimPrefix(importPath, modulePath), "/")
	dir := filepath.Join(rootDir, rel)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	var mdFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			mdFiles = append(mdFiles, name)
		}
	}

	if len(mdFiles) == 0 {
		return ""
	}

	priorities := []string{"readme.md", "package.md", pkgName + ".md"}
	for _, target := range priorities {
		for _, name := range mdFiles {
			if strings.ToLower(name) == target {
				return readFirstLine(filepath.Join(dir, name))
			}
		}
	}
	if len(mdFiles) == 1 {
		return readFirstLine(filepath.Join(dir, mdFiles[0]))
	}

	return ""
}

func readFirstLine(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(content))
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
