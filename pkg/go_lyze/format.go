package go_lyze

import (
	"fmt"
	"sort"
	"strings"
)

// FormatOptions controls how FormatMarkdown renders output.
type FormatOptions struct {
	// TopPkgs pins specific packages as L0 entrypoints, matched by Dir
	// (e.g. "./cmd/myapp"). When set, the topo algorithm starts from these
	// instead of auto-detecting entrypoints.
	TopPkgs []string

	// TransitiveReduction removes edges that are implied by transitivity
	// (e.g. if A→B→C and A→C exist, A→C is dropped). This significantly
	// reduces visual noise in large graphs.
	TransitiveReduction bool
}

// buildLayers computes the topological layers and package-level dependency map
// for a Result, shared by all formatters.
func buildLayers(result Result, opts FormatOptions) ([][]Package, map[string]map[string]struct{}) {
	pkgSet := make(map[string]bool, len(result.Packages))
	for _, p := range result.Packages {
		pkgSet[p.ImportPath] = true
	}

	pinnedDirs := make(map[string]struct{}, len(opts.TopPkgs))
	for _, d := range opts.TopPkgs {
		d = strings.TrimSuffix(d, "/")
		if !strings.HasPrefix(d, "./") && d != "." {
			d = "./" + d
		}
		pinnedDirs[d] = struct{}{}
	}

	deps := make(map[string]map[string]struct{}, len(result.Packages))
	for _, p := range result.Packages {
		deps[p.ImportPath] = make(map[string]struct{})
	}
	for _, p := range result.Packages {
		for _, dep := range p.Dependencies {
			if pkgSet[dep] {
				deps[p.ImportPath][dep] = struct{}{}
			}
		}
	}

	layers := topoLayers(result.Packages, deps, pinnedDirs)
	if opts.TransitiveReduction {
		deps = transitiveReduction(deps)
	}
	return layers, deps
}

// transitiveReduction removes edges u→v where v is already reachable from u
// via another path. This reduces visual clutter without changing reachability.
func transitiveReduction(deps map[string]map[string]struct{}) map[string]map[string]struct{} {
	reduced := make(map[string]map[string]struct{}, len(deps))
	for u, vs := range deps {
		reduced[u] = make(map[string]struct{}, len(vs))
		for v := range vs {
			if !reachableWithout(u, v, deps) {
				reduced[u][v] = struct{}{}
			}
		}
	}
	return reduced
}

// reachableWithout checks whether target is reachable from start using deps,
// but without traversing the direct start→target edge.
func reachableWithout(start, target string, deps map[string]map[string]struct{}) bool {
	visited := make(map[string]struct{})
	queue := make([]string, 0, len(deps[start]))
	for next := range deps[start] {
		if next != target {
			queue = append(queue, next)
		}
	}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if curr == target {
			return true
		}
		if _, seen := visited[curr]; seen {
			continue
		}
		visited[curr] = struct{}{}
		for next := range deps[curr] {
			queue = append(queue, next)
		}
	}
	return false
}

// layerLabel returns the display label for layer i of n total layers.
func layerLabel(i, n int) string {
	label := fmt.Sprintf("L%d", i)
	if i == 0 {
		label += " (entrypoints)"
	} else if i == n-1 {
		label += " (utilities)"
	}
	return label
}

// FormatMarkdown renders a Result as a layered markdown listing.
// Packages are grouped into topological layers (L0 = entrypoints,
// highest layer = pure utilities).
func FormatMarkdown(result Result, opts FormatOptions) string {
	if len(result.Packages) == 0 {
		return "(no packages found)\n"
	}

	layers, _ := buildLayers(result, opts)

	var sb strings.Builder

	for i, layer := range layers {
		fmt.Fprintf(&sb, "## %s\n\n", layerLabel(i, len(layers)))

		for _, pkg := range layer {
			fmt.Fprintf(&sb, "### %s\n", pkg.Dir)
			if pkg.Doc != "" {
				fmt.Fprintf(&sb, "%s\n", pkg.Doc)
			}
			if len(pkg.Exports) > 0 {
				fmt.Fprintln(&sb)
				for _, exp := range pkg.Exports {
					fmt.Fprintf(&sb, "- `%s`\n", exp)
				}
			}
			fmt.Fprintln(&sb)
		}
	}

	return sb.String()
}

// FormatMermaid renders a Result as a Mermaid flowchart (graph TD).
//
// Node IDs are short sequential integers (n0, n1, …) to minimise textual
// footprint. Each topological layer is introduced with a %% comment, and
// node declarations are grouped under that comment so both humans and LLMs
// can read the structure at a glance without wading through mangled paths.
func FormatMermaid(result Result, opts FormatOptions) string {
	if len(result.Packages) == 0 {
		return "graph TD\n"
	}

	layers, deps := buildLayers(result, opts)

	nodeID := mermaidNodeIDs(result.Packages)

	var sb strings.Builder
	fmt.Fprintln(&sb, "graph TD")

	// declare nodes grouped by layer, each group preceded by a %% comment
	for i, layer := range layers {
		fmt.Fprintf(&sb, "  %%%% %s\n", layerLabel(i, len(layers)))
		for _, pkg := range layer {
			fmt.Fprintf(&sb, "  %s[%s]\n", nodeID[pkg.ImportPath], mermaidQuote(pkg.Name))
		}
	}

	// edges — compact: one per line, short IDs only
	fmt.Fprintln(&sb, "  %% edges")
	type edge struct{ from, to string }
	seen := make(map[edge]struct{})
	for _, layer := range layers {
		for _, pkg := range layer {
			callees := make([]string, 0, len(deps[pkg.ImportPath]))
			for c := range deps[pkg.ImportPath] {
				callees = append(callees, c)
			}
			sort.Strings(callees)
			for _, c := range callees {
				e := edge{pkg.ImportPath, c}
				if _, exists := seen[e]; exists {
					continue
				}
				seen[e] = struct{}{}
				fmt.Fprintf(&sb, "  %s-->%s\n", nodeID[pkg.ImportPath], nodeID[c])
			}
		}
	}

	return sb.String()
}

// FormatDot renders a Result as a Graphviz DOT digraph.
// Topological layers are expressed as rank=same subgraphs so the `dot`
// layout engine places entrypoints at the top and utilities at the bottom.
func FormatDot(result Result, opts FormatOptions) string {
	if len(result.Packages) == 0 {
		return "digraph G {}\n"
	}

	layers, deps := buildLayers(result, opts)

	nodeID := make(map[string]string, len(result.Packages))
	for _, p := range result.Packages {
		nodeID[p.ImportPath] = dotID(p.ImportPath)
	}

	var sb strings.Builder
	fmt.Fprintln(&sb, "digraph G {")
	fmt.Fprintln(&sb, `    rankdir=TB;`)
	fmt.Fprintln(&sb, `    node [shape=box, fontname="Helvetica", fontsize=12];`)
	fmt.Fprintln(&sb, `    edge [fontname="Helvetica", fontsize=10];`)
	fmt.Fprintln(&sb)

	// rank=same groups enforce layer ordering
	for i, layer := range layers {
		fmt.Fprintf(&sb, "    // %s\n", layerLabel(i, len(layers)))
		fmt.Fprintln(&sb, "    { rank=same;")
		for _, pkg := range layer {
			fmt.Fprintf(&sb, "        %s [label=%s];\n", nodeID[pkg.ImportPath], dotQuote(pkg.Name))
		}
		fmt.Fprintln(&sb, "    }")
	}

	fmt.Fprintln(&sb)

	// edges
	type edge struct{ from, to string }
	seen := make(map[edge]struct{})
	for _, layer := range layers {
		for _, pkg := range layer {
			callees := make([]string, 0, len(deps[pkg.ImportPath]))
			for c := range deps[pkg.ImportPath] {
				callees = append(callees, c)
			}
			sort.Strings(callees)
			for _, c := range callees {
				e := edge{pkg.ImportPath, c}
				if _, exists := seen[e]; exists {
					continue
				}
				seen[e] = struct{}{}
				fmt.Fprintf(&sb, "    %s -> %s;\n", nodeID[pkg.ImportPath], nodeID[c])
			}
		}
	}

	fmt.Fprintln(&sb, "}")
	return sb.String()
}

// dotID converts an import path to a safe DOT node identifier.
func dotID(importPath string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_")
	return r.Replace(importPath)
}

// dotQuote wraps a label in double quotes, escaping internal quotes.
func dotQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// mermaidNodeIDs assigns each package a short, human-readable Mermaid node ID.
// When a package name is unique across all packages it is used as-is.
// When multiple packages share a name, the minimal import-path suffix that
// makes each ID unique within that group is used instead
// (e.g. "go_lyze" × 2 → "pkg/go_lyze" and "commands/go_lyze").
func mermaidNodeIDs(packages []Package) map[string]string {
	byName := make(map[string][]Package)
	for _, p := range packages {
		byName[p.Name] = append(byName[p.Name], p)
	}

	nodeID := make(map[string]string, len(packages))
	sanitize := func(s string) string {
		r := strings.NewReplacer("/", "_", ".", "_", "-", "_")
		return r.Replace(s)
	}

	for name, pkgs := range byName {
		if len(pkgs) == 1 {
			nodeID[pkgs[0].ImportPath] = sanitize(name)
			continue
		}
		// split each import path into segments and find the minimum suffix
		// depth at which all IDs within this group are distinct
		segs := make([][]string, len(pkgs))
		for i, p := range pkgs {
			segs[i] = strings.Split(p.ImportPath, "/")
		}
		for depth := 1; ; depth++ {
			ids := make(map[string]struct{}, len(pkgs))
			for _, s := range segs {
				start := len(s) - depth
				if start < 0 {
					start = 0
				}
				ids[strings.Join(s[start:], "/")] = struct{}{}
			}
			if len(ids) == len(pkgs) || depth >= len(segs[0]) {
				for i, p := range pkgs {
					start := len(segs[i]) - depth
					if start < 0 {
						start = 0
					}
					nodeID[p.ImportPath] = sanitize(strings.Join(segs[i][start:], "/"))
				}
				break
			}
		}
	}
	return nodeID
}

// mermaidID converts an import path to a safe Mermaid node identifier.
func mermaidID(importPath string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_")
	return r.Replace(importPath)
}

// mermaidQuote wraps a label in double quotes, escaping any internal quotes.
func mermaidQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `'`) + `"`
}

// importPathFromKey strips the trailing ".FuncName" from a relationship key.
func importPathFromKey(key string) string {
	idx := strings.LastIndex(key, ".")
	if idx < 0 {
		return key
	}
	return key[:idx]
}

// topoLayers groups packages into ordered layers using Kahn's algorithm.
// Layer 0 = entrypoints (nothing depends on them), highest layer = pure utilities.
// Within each layer, packages are sorted by DFS preorder from the entrypoints so
// that the listing reflects the actual traversal order of the dependency graph.
// If pinnedDirs is non-empty, those packages are forced into L0.
func topoLayers(packages []Package, deps map[string]map[string]struct{}, pinnedDirs map[string]struct{}) [][]Package {
	pkgByPath := make(map[string]Package, len(packages))
	for _, p := range packages {
		pkgByPath[p.ImportPath] = p
	}

	inDegree := make(map[string]int, len(packages))
	for _, p := range packages {
		inDegree[p.ImportPath] = 0
	}
	for _, toSet := range deps {
		for to := range toSet {
			inDegree[to]++
		}
	}

	var layers [][]Package
	remaining := make(map[string]struct{}, len(packages))
	for _, p := range packages {
		remaining[p.ImportPath] = struct{}{}
	}

	// if caller pinned specific dirs, force them into L0 first
	if len(pinnedDirs) > 0 {
		var pinned []string
		for _, p := range packages {
			if _, ok := pinnedDirs[p.Dir]; ok {
				pinned = append(pinned, p.ImportPath)
			}
		}
		if len(pinned) > 0 {
			sort.Strings(pinned)
			l0 := make([]Package, 0, len(pinned))
			for _, ip := range pinned {
				l0 = append(l0, pkgByPath[ip])
				delete(remaining, ip)
				for callee := range deps[ip] {
					if _, ok := remaining[callee]; ok {
						inDegree[callee]--
					}
				}
			}
			layers = append(layers, l0)
		}
	}

	for len(remaining) > 0 {
		// collect all with inDegree 0 (nothing depends on them)
		var layer []string
		for ip := range remaining {
			if inDegree[ip] == 0 {
				layer = append(layer, ip)
			}
		}

		if len(layer) == 0 {
			// cycle — dump remainder in one final layer
			for ip := range remaining {
				layer = append(layer, ip)
			}
			sort.Strings(layer)
			pkgs := make([]Package, 0, len(layer))
			for _, ip := range layer {
				pkgs = append(pkgs, pkgByPath[ip])
			}
			layers = append(layers, pkgs)
			break
		}

		pkgs := make([]Package, 0, len(layer))
		for _, ip := range layer {
			pkgs = append(pkgs, pkgByPath[ip])
			delete(remaining, ip)
			for callee := range deps[ip] {
				if _, ok := remaining[callee]; ok {
					inDegree[callee]--
				}
			}
		}
		layers = append(layers, pkgs)
	}

	// assign a DFS preorder rank to each package so within-layer ordering
	// reflects the actual traversal order from entrypoints
	dfsRank := dfsPreorderRank(packages, deps)
	for i, layer := range layers {
		sort.Slice(layer, func(a, b int) bool {
			ra, rb := dfsRank[layer[a].ImportPath], dfsRank[layer[b].ImportPath]
			if ra != rb {
				return ra < rb
			}
			return layer[a].ImportPath < layer[b].ImportPath
		})
		layers[i] = layer
	}

	return layers
}

// dfsPreorderRank assigns each package an integer reflecting its visit order in
// a DFS preorder traversal starting from entrypoints (inDegree-0 nodes).
// Lower rank = visited earlier = closer to entrypoints.
func dfsPreorderRank(packages []Package, deps map[string]map[string]struct{}) map[string]int {
	inDeg := make(map[string]int, len(packages))
	for _, p := range packages {
		inDeg[p.ImportPath] = 0
	}
	for _, toSet := range deps {
		for to := range toSet {
			inDeg[to]++
		}
	}

	roots := make([]string, 0)
	for _, p := range packages {
		if inDeg[p.ImportPath] == 0 {
			roots = append(roots, p.ImportPath)
		}
	}
	sort.Strings(roots)

	rank := make(map[string]int, len(packages))
	counter := 0
	visited := make(map[string]struct{}, len(packages))

	var dfs func(ip string)
	dfs = func(ip string) {
		if _, seen := visited[ip]; seen {
			return
		}
		visited[ip] = struct{}{}
		rank[ip] = counter
		counter++
		callees := make([]string, 0, len(deps[ip]))
		for c := range deps[ip] {
			callees = append(callees, c)
		}
		sort.Strings(callees)
		for _, c := range callees {
			dfs(c)
		}
	}

	for _, r := range roots {
		dfs(r)
	}
	// catch any remaining isolated packages
	all := make([]string, 0, len(packages))
	for _, p := range packages {
		all = append(all, p.ImportPath)
	}
	sort.Strings(all)
	for _, ip := range all {
		dfs(ip)
	}

	return rank
}
