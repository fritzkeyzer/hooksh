# Hooksh

Its a dumb working title, for a little utility im playing around with

## Install

```bash
go install github.com/fritzkeyzer/hooksh/cmd/hooksh@latest
```

## Graph gen commands

### DOT > PNG Output
- Requires the `dot` tool.
- `hooksh go-lyze --format dot --top "cmd/hooksh" | dot -Tpng -o "graph.png"`

![graph.png](graph.png)

### Mermaid output

- Eg: `hooksh go-lyze --format mermaid --top "cmd/hooksh"` 
- Or: `hooksh go-lyze --format mermaid --top "cmd/hooksh" > graph.mermaid`
```mermaid
graph TD
  %% L0 (entrypoints)
  main["main"]
  %% L1
  docs["docs"]
  entrypoints["entrypoints"]
  commands_go_lyze["go_lyze"]
  packages["packages"]
  %% L2
  limitutil["limitutil"]
  xmlutil["xmlutil"]
  pkg_go_lyze["go_lyze"]
  %% L3 (utilities)
  fsutil["fsutil"]
  %% edges
  main-->docs
  main-->entrypoints
  main-->commands_go_lyze
  main-->packages
  docs-->fsutil
  docs-->limitutil
  docs-->xmlutil
  entrypoints-->fsutil
  entrypoints-->limitutil
  entrypoints-->xmlutil
  commands_go_lyze-->pkg_go_lyze
  packages-->fsutil
  packages-->limitutil
  packages-->xmlutil
  pkg_go_lyze-->fsutil
```


## XML Output commands

`hooksh` emits one XML block per command:

Eg: `hooksh docs --kind md --limit 10`
```xml
<docs kind="md" limit="10">
./AGENTS.md
./backend/services/core/README.md
./docs/guidelines/ai-collaborators.md
./docs/guidelines/backend.md
./docs/guidelines/connectors.md
./docs/guidelines/frontend-stores.md
./docs/guidelines/frontend.md
./docs/guidelines/gen_go.md
./docs/guidelines/go.md
</docs>
```

Eg: `hooksh entrypoints --format call-tree --depth 2 --functions --exported-only`
```xml
<entrypoints format="call-tree" depth="2">
main.main
├── docs.Run
│   ├── fsutil.NormalizePath
│   ├── fsutil.Sorted
│   ├── fsutil.WalkFiles
│   ├── limitutil.CountOrLimit
│   ├── limitutil.EffectiveLimit
│   ├── xmlutil.TagClose
│   └── xmlutil.TagOpen
├── entrypoints.Run
│   ├── limitutil.CountOrLimit
│   ├── limitutil.EffectiveLimit
│   ├── xmlutil.TagClose
│   └── xmlutil.TagOpen
├── go_lyze.Run
│   ├── go_lyze.Analyze
│   ├── go_lyze.FormatDot
│   ├── go_lyze.FormatMarkdown
│   └── go_lyze.FormatMermaid
└── packages.Run
    ├── limitutil.CountOrLimit
    ├── limitutil.EffectiveLimit
    ├── xmlutil.TagClose
    └── xmlutil.TagOpen
</entrypoints>
```

Eg: `hooksh entrypoints --format call-tree --depth 2 --start "cmd/hooksh"`
```xml
<entrypoints format="call-tree" depth="2">
./cmd/hooksh
├── ./commands/packages
│   ├── ./internal/xmlutil
│   ├── ./internal/limitutil
│   └── ./internal/fsutil
├── ./commands/go_lyze
│   └── ./pkg/go_lyze
├── ./commands/entrypoints
│   ├── ./internal/xmlutil
│   ├── ./internal/limitutil
│   └── ./internal/fsutil
└── ./commands/docs
    ├── ./internal/xmlutil
    ├── ./internal/limitutil
    └── ./internal/fsutil
</entrypoints>
```

Eg: `hooksh packages --kind go-package-doc --limit 10 --order depth`
```xml
<packages kind="go-package-doc" count="7">
- hooksh: provides utilities for generating LLM-ready project context.
- md: provides utilities for indexing markdown files
- docs: lists documentation files for context bootstrapping.
- entrypoints: prints a main package or function call tree.
- go_lyze: analyzes a Go codebase via AST parsing and returns structured data about packages, their exports, and cross-package call relationships.
- go_lyze: provides the go-lyze CLI command.
- packages: lists project packages with their package docs.
</packages>
```


