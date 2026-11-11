package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/md101/kwat/internal/llm"
)

// Client coordinates communication with an external Model Context Protocol (MCP) server.
type Client struct {
	serverName string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Scanner
	mu         sync.Mutex
	reqID      int64
	pending    map[int64]chan *JSONRPCResponse
}

// StartClient spawns an external process and performs initial MCP handshake.
func StartClient(ctx context.Context, serverName string, cfg ServerConfig) (*Client, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("mcp server command cannot be empty")
	}

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range cfg.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe for mcp server %s: %w", serverName, err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdout pipe for mcp server %s: %w", serverName, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mcp server %s: %w", serverName, err)
	}

	c := &Client{
		serverName: serverName,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     bufio.NewScanner(stdoutPipe),
		pending:    make(map[int64]chan *JSONRPCResponse),
	}

	go c.readLoop()

	// Perform initialize handshake
	initParams := map[string]any{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]any{
			"name":    "kwat",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
	}

	_, err = c.Call(ctx, "initialize", initParams)
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("mcp initialize failed for %s: %w", serverName, err)
	}

	// Send initialized notification
	_ = c.Notify("notifications/initialized", map[string]any{})

	return c, nil
}

func (c *Client) readLoop() {
	for c.stdout.Scan() {
		line := c.stdout.Bytes()
		if len(line) == 0 {
			continue
		}

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		if resp.ID != nil {
			var id int64
			switch v := resp.ID.(type) {
			case float64:
				id = int64(v)
			case int64:
				id = v
			}

			c.mu.Lock()
			ch, ok := c.pending[id]
			if ok {
				delete(c.pending, id)
				ch <- &resp
			}
			c.mu.Unlock()
		}
	}
}

// Call sends a JSON-RPC request and blocks until a response is received or context canceled.
func (c *Client) Call(ctx context.Context, method string, params any) (*JSONRPCResponse, error) {
	id := atomic.AddInt64(&c.reqID, 1)
	respChan := make(chan *JSONRPCResponse, 1)

	c.mu.Lock()
	c.pending[id] = respChan
	c.mu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	reqBytes = append(reqBytes, '\n')

	c.mu.Lock()
	_, err = c.stdin.Write(reqBytes)
	c.mu.Unlock()

	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("failed to write to mcp stdin: %w", err)
	}

	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp, nil
	}
}

// Notify sends a JSON-RPC notification (no response expected).
func (c *Client) Notify(method string, params any) error {
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}
	reqBytes = append(reqBytes, '\n')

	c.mu.Lock()
	defer c.mu.Unlock()
	_, err = c.stdin.Write(reqBytes)
	return err
}

// ListTools discovers tools exposed by the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]llm.ToolDefinition, error) {
	resp, err := c.Call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}

	resBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, err
	}

	var listResult ListToolsResult
	if err := json.Unmarshal(resBytes, &listResult); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools/list result: %w", err)
	}

	var defs []llm.ToolDefinition
	for _, t := range listResult.Tools {
		// Namespace MCP tools with serverName (e.g. mcp__postgres__query)
		toolName := fmt.Sprintf("mcp__%s__%s", c.serverName, t.Name)
		defs = append(defs, llm.ToolDefinition{
			Name:        toolName,
			Description: fmt.Sprintf("[%s MCP] %s", c.serverName, t.Description),
			Parameters:  t.InputSchema,
		})
	}

	return defs, nil
}

// CallTool invokes an MCP tool on the server.
func (c *Client) CallTool(ctx context.Context, name string, argsJSON string) (string, error) {
	var args map[string]any
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &args)
	}

	params := map[string]any{
		"name":      name,
		"arguments": args,
	}

	resp, err := c.Call(ctx, "tools/call", params)
	if err != nil {
		return "", err
	}

	resBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return "", err
	}

	var callResult CallToolResult
	if err := json.Unmarshal(resBytes, &callResult); err != nil {
		return string(resBytes), nil
	}

	if len(callResult.Content) > 0 {
		return callResult.Content[0].Text, nil
	}

	return string(resBytes), nil
}

// Close terminates the MCP process.
func (c *Client) Close() error {
	_ = c.stdin.Close()
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	return nil
}

