// Package go_lyze provides the go-lyze CLI command.
package go_lyze

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fritzkeyzer/hooksh/pkg/go_lyze"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

//go:embed graph.html
var graphHTMLTemplate string

// Options holds the CLI flags for the go-lyze command.
type Options struct {
	Dir                 string
	MaxDepth            int
	Format              string   // "json", "md", "mermaid", or "dot"
	TopPkgs             []string // dirs to pin as L0 entrypoints
	TransitiveReduction bool
	Output              string
	HTML                string
	HTMLRender          string
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

	var output string
	switch opts.Format {
	case "md", "markdown":
		output = go_lyze.FormatMarkdown(result, fmtOpts)
	case "mermaid":
		output = go_lyze.FormatMermaid(result, fmtOpts)
	case "dot", "graphviz":
		output = go_lyze.FormatDot(result, fmtOpts)
	default: // "json"
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("error: %v\n", err)
			return
		}
		output = string(out)
	}

	if opts.Output != "" {
		if err := os.WriteFile(opts.Output, []byte(output), 0644); err != nil {
			fmt.Printf("error writing output file: %v\n", err)
		}
	} else {
		fmt.Print(output)
	}

	if opts.HTML != "" || opts.HTMLRender != "" {
		mermaid := go_lyze.FormatMermaid(result, fmtOpts)
		htmlContent := strings.Replace(graphHTMLTemplate, "<!-- MERMAID_DATA -->", mermaid, 1)

		htmlPath := opts.HTML
		isTempHTML := false
		if htmlPath == "" {
			tmpFile, err := os.CreateTemp("", "hooksh-*.html")
			if err != nil {
				fmt.Printf("error creating temp html file: %v\n", err)
				return
			}
			htmlPath = tmpFile.Name()
			tmpFile.Close()
			isTempHTML = true
		}

		if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
			fmt.Printf("error writing html file: %v\n", err)
			return
		}

		if opts.HTMLRender != "" {
			if err := renderHTMLToPNG(htmlPath, opts.HTMLRender); err != nil {
				fmt.Printf("error rendering html to png: %v\n", err)
			}
		}

		if isTempHTML {
			os.Remove(htmlPath)
		}
	}
}

func renderHTMLToPNG(htmlPath, pngPath string) error {
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return err
	}
	url := "file://" + absPath

	browser := rod.New().MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(url)
	page.MustSetViewport(1000, 1000, 1, false)

	// Wait for mermaid to render and vis-network to stabilize
	// We might need to wait for a specific element or just wait some time
	time.Sleep(2 * time.Second)

	// Ensure we are in "screenshotting" mode if the HTML supports it
	page.MustEval(`() => document.body.classList.add('is-screenshotting')`)

	img, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
	if err != nil {
		return err
	}

	return os.WriteFile(pngPath, img, 0644)
}
