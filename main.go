// nvim-quickfix-mcp exposes a single MCP tool for creating Neovim quickfix lists.
//
// Run:
//
//	go mod init example.com/nvim-quickfix-mcp
//	go get github.com/modelcontextprotocol/go-sdk/mcp github.com/neovim/go-client/nvim
//	go run nvim-quickfix-mcp.go /path/to/nvim.sock
//
// Find the socket in Neovim with: :echo v:servername
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/neovim/go-client/nvim"
)

type Location struct {
	Filename string `json:"filename" jsonschema:"File path (absolute, or relative to the MCP server working directory)"`
	Lnum     int    `json:"lnum" jsonschema:"1-based line number obtained from source inspection tools; do not guess"`
	Col      int    `json:"col,omitempty" jsonschema:"1-based column number; 0 means unspecified"`
	Text     string `json:"text,omitempty" jsonschema:"Reason this location supports the answer"`
}

type QuickfixArgs struct {
	Locations []Location `json:"locations" jsonschema:"Source-code locations supporting the findings"`
}

// The Lua code is fixed; user-controlled values are passed separately as RPC arguments.
const setQuickfix = `
local items = ...
vim.fn.setqflist({}, " ", { title = "LLM Evidence", items = items })
return #items
`

func quickfix(ctx context.Context, req *mcp.CallToolRequest, args QuickfixArgs) (*mcp.CallToolResult, any, error) {
	// Each call opens its own short-lived RPC connection.
	socket := os.Args[1]
	v, err := nvim.Dial(socket)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to Neovim: %w", err)
	}
	defer v.Close()

	items := make([]map[string]any, 0, len(args.Locations))
	for i, loc := range args.Locations {
		if loc.Filename == "" || loc.Lnum < 1 || loc.Col < 0 {
			return nil, nil, fmt.Errorf("locations[%d]: filename and positive lnum required; col must be >= 0", i)
		}
		path, err := filepath.Abs(loc.Filename)
		if err != nil {
			return nil, nil, fmt.Errorf("locations[%d]: %w", i, err)
		}
		items = append(items, map[string]any{
			"filename": path,
			"lnum":     loc.Lnum,
			"col":      loc.Col,
			"text":     loc.Text,
		})
	}

	var count int
	if err := v.ExecLua(setQuickfix, &count, items); err != nil {
		return nil, nil, fmt.Errorf("update quickfix: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: fmt.Sprintf("Registered %d source locations", count),
		}},
	}, nil, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: nvim-quickfix-mcp <nvim-socket>")
		os.Exit(2)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "nvim-quickfix", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "evidence",
		Description: "Present the source-code locations that support your findings for the user to inspect. Provide all relevant locations in one call. Line and column numbers are 1-based. Use line numbers obtained from source inspection tools; do not guess line numbers.",
	}, quickfix)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
