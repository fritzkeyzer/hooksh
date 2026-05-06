// Package go_lyze provides the go-lyze CLI command.
package go_lyze

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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
	HTMLDark            bool
	HTMLHiddenNodes     string
	HTMLRenderRes       string
	HTMLNodeColors      []string
	HTMLLayout          string
	HTMLTitle           string
	HTMLSubtitle        string
}

type htmlConfig struct {
	HiddenNodes  []string          `json:"hiddenNodes"`
	NodeColorMap map[string]string `json:"nodeColorMap"`
	DarkDefault  bool              `json:"darkDefault"`
	Layout       string            `json:"layout"`
	Title        string            `json:"title"`
	Subtitle     string            `json:"subtitle"`
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
	} else if !(opts.Format == "mermaid" && (opts.HTML != "" || opts.HTMLRender != "")) {
		fmt.Print(output)
	}

	if opts.HTML != "" || opts.HTMLRender != "" {
		hiddenNodes := parseCSV(opts.HTMLHiddenNodes)
		nodeColorMap, err := parseNodeColorOverrides(opts.HTMLNodeColors)
		if err != nil {
			fmt.Printf("error parsing --html-node-color: %v\n", err)
			return
		}

		cfg := htmlConfig{
			HiddenNodes:  hiddenNodes,
			NodeColorMap: nodeColorMap,
			DarkDefault:  opts.HTMLDark,
			Layout:       opts.HTMLLayout,
			Title:        opts.HTMLTitle,
			Subtitle:     opts.HTMLSubtitle,
		}
		cfgJSON, err := json.Marshal(cfg)
		if err != nil {
			fmt.Printf("error encoding html config: %v\n", err)
			return
		}

		mermaid := go_lyze.FormatMermaid(result, fmtOpts)
		htmlContent := strings.Replace(graphHTMLTemplate, "<!-- MERMAID_DATA -->", mermaid, 1)
		htmlContent = strings.Replace(htmlContent, "/* HOOKSH_CONFIG */ {}", string(cfgJSON), 1)

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
			width, height, err := parseRenderRes(opts.HTMLRenderRes)
			if err != nil {
				fmt.Printf("error parsing --html-render-res: %v\n", err)
				return
			}

			if err := renderHTMLToPNG(htmlPath, opts.HTMLRender, width, height); err != nil {
				fmt.Printf("error rendering html to png: %v\n", err)
			}
		}

		if isTempHTML {
			os.Remove(htmlPath)
		}
	}
}

func parseCSV(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}

	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func parseNodeColorOverrides(items []string) (map[string]string, error) {
	if len(items) == 0 {
		return map[string]string{}, nil
	}

	out := map[string]string{}
	for i := 0; i < len(items); i++ {
		raw := items[i]
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		name, color, ok := strings.Cut(entry, ",")
		if !ok {
			if i+1 >= len(items) {
				return nil, fmt.Errorf("expected node,hex but got %q", raw)
			}

			name = entry
			i++
			color = strings.TrimSpace(items[i])
		}

		node := strings.TrimSpace(name)
		hex := strings.TrimSpace(color)
		if node == "" || hex == "" {
			return nil, fmt.Errorf("expected non-empty node and hex in %q", raw)
		}

		out[node] = hex
	}

	return out, nil
}

func parseRenderRes(input string) (int, int, error) {
	const defaultW = 800
	const defaultH = 800

	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return defaultW, defaultH, nil
	}

	wRaw, hRaw, ok := strings.Cut(trimmed, ",")
	if !ok {
		return 0, 0, fmt.Errorf("expected width,height")
	}

	width, err := strconv.Atoi(strings.TrimSpace(wRaw))
	if err != nil || width <= 0 {
		return 0, 0, fmt.Errorf("invalid width %q", strings.TrimSpace(wRaw))
	}

	height, err := strconv.Atoi(strings.TrimSpace(hRaw))
	if err != nil || height <= 0 {
		return 0, 0, fmt.Errorf("invalid height %q", strings.TrimSpace(hRaw))
	}

	return width, height, nil
}

func renderHTMLToPNG(htmlPath, pngPath string, width, height int) error {
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return err
	}
	url := "file://" + absPath

	browser := rod.New().MustConnect()
	defer browser.MustClose()

	page := browser.MustPage(url)
	page.MustSetViewport(width, height, 1, false)

	// Wait for mermaid to render and vis-network to stabilize
	// We might need to wait for a specific element or just wait some time
	page.MustWaitStable()
	page.MustWaitIdle()
	//time.Sleep(5 * time.Second)

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
