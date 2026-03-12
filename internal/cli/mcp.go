package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/spf13/cobra"
	// "github.com/ScriptonBasestar/dva/internal/runner"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP (Model Context Protocol) JSON-RPC Server via stdio",
	Long:  "Acts as an MCP server to allow LLMs (Cursor, Claude) to natively use DVA commands.",
	Run:   runMcp,
}

// Minimal JSON-RPC structures for MCP
type rpcRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func sendResponse(id json.RawMessage, result interface{}) {
	if id == nil {
		return // Notification, no response needed
	}
	resp := rpcResponse{
		Jsonrpc: "2.0",
		Id:      id,
		Result:  result,
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", string(b))
}

func sendError(id json.RawMessage, code int, message string) {
	if id == nil {
		return
	}
	resp := rpcResponse{
		Jsonrpc: "2.0",
		Id:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
	b, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", string(b))
}

// runMcp reads stdin line by line parsing JSON-RPC requests
func runMcp(cmd *cobra.Command, args []string) {
	// Silence regular logs to not corrupt stdout JSON-RPC
	log.SetOutput(io.Discard)

	c, err := config.Load(".")
	hasConfig := (err == nil)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Invaid JSON
			sendError(json.RawMessage("null"), -32700, "Parse error")
			continue
		}

		switch req.Method {
		case "initialize":
			sendResponse(req.Id, map[string]interface{}{
				"protocolVersion": "2024-11-05", // Standard MCP protocol version requirement
				"serverInfo": map[string]interface{}{
					"name":    "dva-mcp",
					"version": "0.1.0",
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
			})
		case "notifications/initialized":
			// no-op

		case "tools/list":
			var tools []map[string]interface{}
			if hasConfig {
				// We map all Interaction commands as tools
				for name, meta := range c.Interaction {
					tools = append(tools, map[string]interface{}{
						"name":        "dva_" + name,
						"description": fmt.Sprintf("DVA Command: %s", meta.Description),
						"inputSchema": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					})
				}
			}
			sendResponse(req.Id, map[string]interface{}{
				"tools": tools,
			})

		case "tools/call":
			// Tool execution
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			if err := json.Unmarshal(req.Params, &params); err != nil {
				sendError(req.Id, -32602, "Invalid params")
				continue
			}

			if !strings.HasPrefix(params.Name, "dva_") {
				sendError(req.Id, -32601, "Tool not found")
				continue
			}

			cmdName := strings.TrimPrefix(params.Name, "dva_")
			if !hasConfig || c.Interaction[cmdName] == nil {
				sendResponse(req.Id, map[string]interface{}{
					"content": []map[string]interface{}{
						{"type": "text", "text": "Command not found or no dva.yml"},
					},
					"isError": true,
				})
				continue
			}

			// Capture DVA execution output
			output, execErr := executeDvaCommandCapture(cmdName)
			isError := execErr != nil
			if execErr != nil {
				output = fmt.Sprintf("Error: %v\nOutput: %s", execErr, output)
			}

			sendResponse(req.Id, map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": output},
				},
				"isError": isError,
			})

		default:
			// Ignore unsupported methods
			if req.Id != nil {
				sendError(req.Id, -32601, "Method not found")
			}
		}
	}
}

// executeDvaCommandCapture runs a standard DVA command and captures stdout/stderr
func executeDvaCommandCapture(cmdName string) (string, error) {
	// Re-route stdout/err, execute via shell or subprocess
	// To keep things independent from os.Exit(1) embedded in cli,
	// we just spawn our *own* binary!

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	cmd := exec.Command(exe, "run", cmdName)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err = cmd.Run()
	return strings.TrimSpace(out.String()), err
}
