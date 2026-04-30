// Package entrypoints prints a main package or function call tree.
package entrypoints

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fritzkeyzer/hooksh/internal/fsutil"
	"github.com/fritzkeyzer/hooksh/internal/limitutil"
	"github.com/fritzkeyzer/hooksh/internal/xmlutil"
)

type Options struct {
	Format       string
	Limit        int
	Depth        int
	Start        string
	SkipPackages string
	Functions    bool
	ExportedOnly bool
}

type packageInfo struct {
	name    string
	hasMain bool
	imports map[string]struct{}
}

type functionInfo struct {
	packagePath string
	name        string
	exported    bool
	calls       map[string]struct{}
}

type treeNode struct {
	name     string
	children []*treeNode
}

func Run(opts Options) {
	if opts.Format != "call-tree" {
		fmt.Println(xmlutil.TagOpen("entrypoints",
			xmlutil.Attr{Key: "format", Value: opts.Format},
			xmlutil.Attr{Key: "count", Value: "0"},
		))
		fmt.Printf("%s\n\n", xmlutil.TagClose("entrypoints"))
		return
	}

	modulePath := readModulePath("go.mod")
	if modulePath == "" {
		fmt.Println(xmlutil.TagOpen("entrypoints",
			xmlutil.Attr{Key: "format", Value: opts.Format},
			xmlutil.Attr{Key: "count", Value: "0"},
		))
		fmt.Println("main")
		fmt.Printf("%s\n\n", xmlutil.TagClose("entrypoints"))
		return
	}

	packages, functions := collectLocalPackages(modulePath)
	skipped := parsePackagePathSet(opts.SkipPackages, modulePath)
	root := buildMainPackageTree(packages, functions, modulePath, opts.Functions, opts.ExportedOnly, opts.Start, skipped)
	totalNodes := countNodes(root) - 1

	emitLimit := limitutil.EffectiveLimit(totalNodes, opts.Limit)
	if opts.Depth > 0 {
		minForDepth := countNodesUpToDepth(root, opts.Depth) - 1
		if minForDepth > emitLimit {
			emitLimit = minForDepth
		}
	}
	trimmed := trimTree(root, &emitLimit)

	attrs := []xmlutil.Attr{{Key: "format", Value: opts.Format}}
	if opts.Depth > 0 {
		attrs = append(attrs, xmlutil.Attr{Key: "depth", Value: fmt.Sprintf("%d", opts.Depth)})
	} else {
		attrKey, attrValue := limitutil.CountOrLimit(totalNodes, opts.Limit)
		attrs = append(attrs, xmlutil.Attr{Key: attrKey, Value: fmt.Sprintf("%d", attrValue)})
	}

	fmt.Println(xmlutil.TagOpen("entrypoints", attrs...))

	fmt.Println(trimmed.name)
	for i, child := range trimmed.children {
		printTreeNode(child, "", i == len(trimmed.children)-1)
	}

	fmt.Printf("%s\n\n", xmlutil.TagClose("entrypoints"))
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

func collectLocalPackages(modulePath string) (map[string]*packageInfo, map[string]*functionInfo) {
	fset := token.NewFileSet()
	packages := make(map[string]*packageInfo)
	functions := make(map[string]*functionInfo)

	_ = fsutil.WalkFiles(".", func(path string, info os.FileInfo) error {
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		dir := filepath.Dir(path)
		if dir == "." {
			dir = ""
		}

		importPath := modulePath
		if dir != "" {
			importPath = modulePath + "/" + filepath.ToSlash(dir)
		}

		pkg := packages[importPath]
		if pkg == nil {
			pkg = &packageInfo{
				name:    node.Name.Name,
				imports: make(map[string]struct{}),
			}
			packages[importPath] = pkg
		}

		localImports := localImportAliases(node, modulePath)

		if node.Name.Name == "main" {
			for _, decl := range node.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				if fn.Name.Name == "main" {
					pkg.hasMain = true
				}

				funcKey := buildFunctionKey(importPath, fn.Name.Name)
				fnInfo := functions[funcKey]
				if fnInfo == nil {
					fnInfo = &functionInfo{
						packagePath: importPath,
						name:        fn.Name.Name,
						exported:    ast.IsExported(fn.Name.Name),
						calls:       make(map[string]struct{}),
					}
					functions[funcKey] = fnInfo
				}

				for _, called := range collectFunctionCalls(fn, importPath, localImports) {
					fnInfo.calls[called] = struct{}{}
				}
			}
		} else {
			for _, decl := range node.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}

				funcKey := buildFunctionKey(importPath, fn.Name.Name)
				fnInfo := functions[funcKey]
				if fnInfo == nil {
					fnInfo = &functionInfo{
						packagePath: importPath,
						name:        fn.Name.Name,
						exported:    ast.IsExported(fn.Name.Name),
						calls:       make(map[string]struct{}),
					}
					functions[funcKey] = fnInfo
				}

				for _, called := range collectFunctionCalls(fn, importPath, localImports) {
					fnInfo.calls[called] = struct{}{}
				}
			}
		}

		for _, imp := range node.Imports {
			importedPath, unquoteErr := strconv.Unquote(imp.Path.Value)
			if unquoteErr != nil {
				continue
			}
			if importedPath == modulePath || strings.HasPrefix(importedPath, modulePath+"/") {
				pkg.imports[importedPath] = struct{}{}
			}
		}

		return nil
	})

	return packages, functions
}

func buildMainPackageTree(packages map[string]*packageInfo, functions map[string]*functionInfo, modulePath string, includeFunctions bool, exportedOnly bool, start string, skipped map[string]struct{}) *treeNode {
	if includeFunctions {
		return buildMainFunctionTree(packages, functions, exportedOnly, skipped)
	}

	topoOrder := computePackageTopoOrder(packages, skipped)

	mainPackages := make([]string, 0)
	startPackages := parsePackagePaths(start, modulePath)
	if len(startPackages) > 0 {
		mainPackages = append(mainPackages, startPackages...)

		if len(mainPackages) == 1 {
			child := buildSubtree(mainPackages[0], packages, modulePath, map[string]struct{}{}, skipped, topoOrder)
			if child != nil {
				return child
			}
			return &treeNode{name: "entrypoints"}
		}

		root := &treeNode{name: "entrypoints"}
		for _, mainPath := range mainPackages {
			child := buildSubtree(mainPath, packages, modulePath, map[string]struct{}{}, skipped, topoOrder)
			if child != nil {
				root.children = append(root.children, child)
			}
		}
		return root
	} else {
		for importPath, pkg := range packages {
			if pkg.hasMain {
				mainPackages = append(mainPackages, importPath)
			}
		}
		sort.Strings(mainPackages)
	}

	root := &treeNode{name: "main"}

	for _, mainPath := range mainPackages {
		for _, dep := range orderedLocalImports(packages[mainPath].imports, packages, skipped, topoOrder) {
			child := buildSubtree(dep, packages, modulePath, map[string]struct{}{}, skipped, topoOrder)
			if child != nil {
				root.children = append(root.children, child)
			}
		}
	}

	return root
}

func parsePackagePaths(value, modulePath string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, entry := range strings.Split(value, ",") {
		rel := strings.TrimSpace(filepath.ToSlash(entry))
		rel = strings.TrimPrefix(rel, "./")
		rel = strings.Trim(rel, "/")

		importPath := modulePath
		if rel != "" {
			importPath = modulePath + "/" + rel
		}

		if _, exists := seen[importPath]; exists {
			continue
		}
		seen[importPath] = struct{}{}
		result = append(result, importPath)
	}

	return result
}

func parsePackagePathSet(value, modulePath string) map[string]struct{} {
	paths := parsePackagePaths(value, modulePath)
	if len(paths) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}

	return set
}

func buildMainFunctionTree(packages map[string]*packageInfo, functions map[string]*functionInfo, exportedOnly bool, skipped map[string]struct{}) *treeNode {
	root := &treeNode{name: "main.main"}

	mainFunctions := make([]string, 0)
	for importPath, pkg := range packages {
		if !pkg.hasMain {
			continue
		}
		mainFunc := buildFunctionKey(importPath, "main")
		if _, exists := functions[mainFunc]; exists {
			mainFunctions = append(mainFunctions, mainFunc)
		}
	}
	sort.Strings(mainFunctions)

	visited := make(map[string]struct{})
	for _, mainFunc := range mainFunctions {
		visited[mainFunc] = struct{}{}
		for _, called := range orderedLocalCalls(functions[mainFunc].calls, functions, exportedOnly, skipped) {
			child := buildFunctionSubtree(called, packages, functions, cloneVisited(visited), exportedOnly, skipped)
			if child != nil {
				root.children = append(root.children, child)
			}
		}
		delete(visited, mainFunc)
	}

	return root
}

func buildSubtree(importPath string, packages map[string]*packageInfo, modulePath string, visited map[string]struct{}, skipped map[string]struct{}, topoOrder map[string]int) *treeNode {
	if _, skip := skipped[importPath]; skip {
		return nil
	}
	if _, seen := visited[importPath]; seen {
		return nil
	}
	visited[importPath] = struct{}{}
	defer delete(visited, importPath)

	node := &treeNode{name: displayPath(importPath, modulePath, packages)}

	pkg := packages[importPath]
	if pkg == nil {
		return node
	}

	for _, dep := range orderedLocalImports(pkg.imports, packages, skipped, topoOrder) {
		child := buildSubtree(dep, packages, modulePath, visited, skipped, topoOrder)
		if child != nil {
			node.children = append(node.children, child)
		}
	}

	return node
}

func buildFunctionSubtree(functionKey string, packages map[string]*packageInfo, functions map[string]*functionInfo, visited map[string]struct{}, exportedOnly bool, skipped map[string]struct{}) *treeNode {
	if _, seen := visited[functionKey]; seen {
		return nil
	}
	visited[functionKey] = struct{}{}
	defer delete(visited, functionKey)

	fn := functions[functionKey]
	if fn == nil {
		return nil
	}
	if _, skip := skipped[fn.packagePath]; skip {
		return nil
	}
	if exportedOnly && !fn.exported {
		return nil
	}

	node := &treeNode{name: displayFunction(functionKey, packages, functions)}
	for _, called := range orderedLocalCalls(fn.calls, functions, exportedOnly, skipped) {
		child := buildFunctionSubtree(called, packages, functions, visited, exportedOnly, skipped)
		if child != nil {
			node.children = append(node.children, child)
		}
	}

	return node
}

func cloneVisited(visited map[string]struct{}) map[string]struct{} {
	copy := make(map[string]struct{}, len(visited))
	for key := range visited {
		copy[key] = struct{}{}
	}
	return copy
}

func orderedLocalImports(imports map[string]struct{}, packages map[string]*packageInfo, skipped map[string]struct{}, topoOrder map[string]int) []string {
	deps := make([]string, 0, len(imports))
	for dep := range imports {
		if _, skip := skipped[dep]; skip {
			continue
		}
		if _, exists := packages[dep]; exists {
			deps = append(deps, dep)
		}
	}

	sort.Slice(deps, func(i, j int) bool {
		leftRank, leftOK := topoOrder[deps[i]]
		rightRank, rightOK := topoOrder[deps[j]]

		switch {
		case leftOK && rightOK && leftRank != rightRank:
			return leftRank < rightRank
		case leftOK && !rightOK:
			return true
		case !leftOK && rightOK:
			return false
		default:
			return deps[i] < deps[j]
		}
	})

	return deps
}

func computePackageTopoOrder(packages map[string]*packageInfo, skipped map[string]struct{}) map[string]int {
	allPackages := make([]string, 0, len(packages))
	for importPath := range packages {
		if _, skip := skipped[importPath]; skip {
			continue
		}
		allPackages = append(allPackages, importPath)
	}
	sort.Strings(allPackages)

	state := make(map[string]uint8, len(allPackages))
	postorder := make([]string, 0, len(allPackages))

	var visit func(string)
	visit = func(importPath string) {
		if _, skip := skipped[importPath]; skip {
			return
		}
		if _, exists := packages[importPath]; !exists {
			return
		}

		switch state[importPath] {
		case 1:
			return
		case 2:
			return
		}

		state[importPath] = 1
		for _, dep := range orderedLocalImports(packages[importPath].imports, packages, skipped, nil) {
			visit(dep)
		}
		state[importPath] = 2
		postorder = append(postorder, importPath)
	}

	for _, importPath := range allPackages {
		visit(importPath)
	}

	rank := make(map[string]int, len(postorder))
	for i := len(postorder) - 1; i >= 0; i-- {
		rank[postorder[i]] = len(postorder) - 1 - i
	}

	return rank
}

func orderedLocalCalls(calls map[string]struct{}, functions map[string]*functionInfo, exportedOnly bool, skipped map[string]struct{}) []string {
	deps := make([]string, 0, len(calls))
	for dep := range calls {
		fn, exists := functions[dep]
		if !exists {
			continue
		}
		if _, skip := skipped[fn.packagePath]; skip {
			continue
		}
		if !exportedOnly || fn.exported {
			deps = append(deps, dep)
		}
	}

	sort.Strings(deps)

	return deps
}

func displayPath(importPath, modulePath string, packages map[string]*packageInfo) string {
	if pkg := packages[importPath]; pkg != nil && pkg.name == "main" {
		rel := strings.TrimPrefix(strings.TrimPrefix(importPath, modulePath), "/")
		if rel == "" {
			return "./"
		}
		return "./" + rel
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(importPath, modulePath), "/")
	if rel == "" {
		return "./"
	}
	return "./" + rel
}

func displayFunction(functionKey string, packages map[string]*packageInfo, functions map[string]*functionInfo) string {
	fn := functions[functionKey]
	if fn == nil {
		return functionKey
	}

	pkgName := "main"
	if pkg := packages[fn.packagePath]; pkg != nil && pkg.name != "" {
		pkgName = pkg.name
	}

	return pkgName + "." + fn.name
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

func collectFunctionCalls(fn *ast.FuncDecl, packagePath string, localImports map[string]string) []string {
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
			called[buildFunctionKey(packagePath, target.Name)] = struct{}{}
		case *ast.SelectorExpr:
			pkgIdent, ok := target.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath, exists := localImports[pkgIdent.Name]
			if !exists {
				return true
			}
			called[buildFunctionKey(importPath, target.Sel.Name)] = struct{}{}
		}

		return true
	})

	result := make([]string, 0, len(called))
	for key := range called {
		result = append(result, key)
	}

	return result
}

func buildFunctionKey(packagePath, functionName string) string {
	return packagePath + "." + functionName
}

func countNodes(root *treeNode) int {
	if root == nil {
		return 0
	}
	total := 1
	for _, child := range root.children {
		total += countNodes(child)
	}
	return total
}

func countNodesUpToDepth(root *treeNode, maxDepth int) int {
	if root == nil || maxDepth < 0 {
		return 0
	}

	type queueItem struct {
		node  *treeNode
		depth int
	}

	count := 0
	queue := []queueItem{{node: root, depth: 0}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		count++

		if current.depth >= maxDepth {
			continue
		}

		for _, child := range current.node.children {
			queue = append(queue, queueItem{node: child, depth: current.depth + 1})
		}
	}

	return count
}

func trimTree(root *treeNode, remaining *int) *treeNode {
	if root == nil {
		return nil
	}
	clone := &treeNode{name: root.name}
	if *remaining <= 0 {
		return clone
	}

	type queueItem struct {
		source *treeNode
		target *treeNode
	}

	queue := []queueItem{{source: root, target: clone}}
	for len(queue) > 0 && *remaining > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, child := range current.source.children {
			if *remaining <= 0 {
				break
			}

			childClone := &treeNode{name: child.name}
			current.target.children = append(current.target.children, childClone)
			*remaining--
			queue = append(queue, queueItem{source: child, target: childClone})
		}
	}

	return clone
}

func printTreeNode(node *treeNode, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	fmt.Printf("%s%s%s\n", prefix, connector, node.name)

	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	for i, child := range node.children {
		printTreeNode(child, childPrefix, i == len(node.children)-1)
	}
}
