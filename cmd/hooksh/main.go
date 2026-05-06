package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fritzkeyzer/hooksh/commands/docs"
	"github.com/fritzkeyzer/hooksh/commands/entrypoints"
	golyze "github.com/fritzkeyzer/hooksh/commands/go_lyze"
	"github.com/fritzkeyzer/hooksh/commands/packages"
	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:    "hooksh",
		Usage:   "LLM-ready code context generator",
		Version: "v0.0.4",
		Commands: []*cli.Command{
			{
				Name:  "docs",
				Usage: "list documentation files",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "kind",
						Usage: "document kind",
						Value: "md",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "limit number of docs",
						Value: 10,
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					docs.Run(docs.Options{
						Kind:  cmd.String("kind"),
						Limit: cmd.Int("limit"),
					})
					return nil
				},
			},
			{
				Name:  "packages",
				Usage: "list project packages",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "kind",
						Usage: "package output kind",
						Value: "go-package-doc",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "limit number of packages",
						Value: 10,
					},
					&cli.StringFlag{
						Name:  "order",
						Usage: "order of packages",
						Value: "depth",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					packages.Run(packages.Options{
						Kind:  cmd.String("kind"),
						Limit: cmd.Int("limit"),
						Order: cmd.String("order"),
					})
					return nil
				},
			},
			{
				Name:  "entrypoints",
				Usage: "show project entrypoint call tree",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Usage: "entrypoint output format",
						Value: "call-tree",
					},
					&cli.IntFlag{
						Name:  "limit",
						Usage: "limit number of nodes",
						Value: 10,
					},
					&cli.IntFlag{
						Name:  "depth",
						Usage: "always include nodes up to this depth before applying limit",
						Value: 0,
					},
					&cli.StringFlag{
						Name:  "start",
						Usage: "comma-separated package directories to use as entry points",
						Value: "",
					},
					&cli.StringFlag{
						Name:  "skip-packages",
						Usage: "comma-separated package directories to skip from analysis",
						Value: "",
					},
					&cli.BoolFlag{
						Name:  "functions",
						Usage: "switch tree output from packages to functions",
					},
					&cli.BoolFlag{
						Name:  "exported-only",
						Usage: "only include exported functions in functions mode",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if cmd.Bool("functions") && strings.TrimSpace(cmd.String("start")) != "" {
						return fmt.Errorf("--start cannot be used with --functions")
					}

					entrypoints.Run(entrypoints.Options{
						Format:       cmd.String("format"),
						Limit:        cmd.Int("limit"),
						Depth:        cmd.Int("depth"),
						Start:        cmd.String("start"),
						SkipPackages: cmd.String("skip-packages"),
						Functions:    cmd.Bool("functions"),
						ExportedOnly: cmd.Bool("exported-only"),
					})
					return nil
				},
			},
			{
				Name:  "go-lyze",
				Usage: "analyze Go codebase structure and relationships",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "dir",
						Usage: "root directory to analyze",
						Value: ".",
					},
					&cli.IntFlag{
						Name:  "max-depth",
						Usage: "maximum directory depth to walk (0 = unlimited)",
						Value: 0,
					},
					&cli.StringFlag{
						Name:  "format",
						Usage: "output format: json, md, mermaid, or dot",
						Value: "json",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "specify the output file",
					},
					&cli.StringFlag{
						Name:  "html",
						Usage: "output graph.html to specified filename",
					},
					&cli.StringFlag{
						Name:  "html-render",
						Usage: "render graph to png via headless chrome to specified filename",
					},
					&cli.BoolFlag{
						Name:  "html-dark",
						Usage: "set html default theme to dark mode (without this flag default is light)",
					},
					&cli.StringFlag{
						Name:  "html-hidden-nodes",
						Usage: "comma-separated package ids to hide in html output",
					},
					&cli.StringFlag{
						Name:  "html-render-res",
						Usage: "render resolution for --html-render in pixels as width,height",
					},
					&cli.StringFlag{
						Name:  "html-layout",
						Usage: "mermaid layout direction (TD or LR)",
						Value: "TD",
					},
					&cli.StringFlag{
						Name:  "html-title",
						Usage: "title to display in the html graph",
					},
					&cli.StringFlag{
						Name:  "html-subtitle",
						Usage: "subtitle to display in the html graph",
					},
					&cli.StringSliceFlag{
						Name:  "html-node-color",
						Usage: "node color override as node,hex (repeatable)",
					},
					&cli.StringSliceFlag{
						Name:  "top",
						Usage: "comma-separated package dirs to pin as L0 entrypoints (md format only)",
					},
					&cli.BoolFlag{
						Name:  "transitive-reduction",
						Usage: "remove edges implied by transitivity to reduce visual noise",
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					golyze.Run(golyze.Options{
						Dir:                 cmd.String("dir"),
						MaxDepth:            cmd.Int("max-depth"),
						Format:              cmd.String("format"),
						TopPkgs:             cmd.StringSlice("top"),
						TransitiveReduction: cmd.Bool("transitive-reduction"),
						Output:              cmd.String("output"),
						HTML:                cmd.String("html"),
						HTMLRender:          cmd.String("html-render"),
						HTMLDark:            cmd.Bool("html-dark"),
						HTMLHiddenNodes:     cmd.String("html-hidden-nodes"),
						HTMLRenderRes:       cmd.String("html-render-res"),
						HTMLLayout:          cmd.String("html-layout"),
						HTMLNodeColors:      cmd.StringSlice("html-node-color"),
						HTMLTitle:           cmd.String("html-title"),
						HTMLSubtitle:        cmd.String("html-subtitle"),
					})
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
