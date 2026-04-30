// Package go_lyze provides the go-lyze CLI command.
package go_lyze

import (
	"encoding/json"
	"fmt"

	"github.com/fritzkeyzer/hooksh/pkg/go_lyze"
)

// Options holds the CLI flags for the go-lyze command.
type Options struct {
	Dir                 string
	MaxDepth            int
	Format              string   // "json", "md", "mermaid", or "dot"
	TopPkgs             []string // dirs to pin as L0 entrypoints
	TransitiveReduction bool
}

// Run performs the analysis and prints the result in the requested format.
func Run(opts Options) {
	result, err := go_lyze.Analyze(go_lyze.Options{
		RootDir:  opts.Dir,
		MaxDepth: opts.MaxDepth,
		TopPkgs:  opts.TopPkgs,
	})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmtOpts := go_lyze.FormatOptions{
		TopPkgs:             opts.TopPkgs,
		TransitiveReduction: opts.TransitiveReduction,
	}

	switch opts.Format {
	case "md", "markdown":
		fmt.Print(go_lyze.FormatMarkdown(result, fmtOpts))
	case "mermaid":
		fmt.Print(go_lyze.FormatMermaid(result, fmtOpts))
	case "dot", "graphviz":
		fmt.Print(go_lyze.FormatDot(result, fmtOpts))
	default: // "json"
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		fmt.Println(string(out))
	}
}
