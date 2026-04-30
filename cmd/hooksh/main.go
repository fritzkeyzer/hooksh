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
		Name:  "hooksh",
		Usage: "LLM-ready code context generator",
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
