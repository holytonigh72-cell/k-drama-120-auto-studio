// Package abocado connects to the MCP server configured on this Windows
// workstation. It supports Codex's remote HTTPS MCP entries and stdio servers.
// Credentials stay in process memory and are never logged or persisted.
package abocado

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Server struct {
	Command           string            `json:"command"`
	Args              []string          `json:"args"`
	Env               map[string]string `json:"env"`
	URL               string            `json:"url"`
	BearerTokenEnvVar string            `json:"bearer_token_env_var"`
	HTTPHeaders       map[string]string `json:"http_headers"`
	EnvHTTPHeaders    map[string]string `json:"env_http_headers"`
}
type Config struct {
	MCPServers map[string]Server `json:"mcpServers"`
}
type Client struct {
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     *bufio.Reader
	mu         sync.Mutex
	next       int
	name       string
	endpoint   string
	session    string
	headers    map[string]string
	httpClient *http.Client
}
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func FindConfig() []string {
	var paths []string
	if p := os.Getenv("CODEX_CONFIG"); p != "" {
		paths = append(paths, p)
	}
	if p := os.Getenv("APPDATA"); p != "" {
		paths = append(paths, filepath.Join(p, "Codex", "config.toml"), filepath.Join(p, "Codex", "config.json"))
	}
	if p := os.Getenv("USERPROFILE"); p != "" {
		paths = append(paths, filepath.Join(p, ".codex", "config.toml"), filepath.Join(p, ".codex", "config.json"))
	}
	return paths
}
func FirstAvailableConfig() string {
	for _, p := range FindConfig() {
		if _, e := os.Stat(p); e == nil {
			return p
		}
	}
	return ""
}
func LoadServer(path, name string) (Server, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Server{}, err
	}
	var c Config
	if json.Unmarshal(b, &c) == nil && len(c.MCPServers) > 0 {
		s, ok := c.MCPServers[name]
		if !ok {
			return Server{}, fmt.Errorf("MCP server %q was not found", name)
		}
		return s, nil
	}
	return parseCodexTOML(string(b), name)
}
func parseCodexTOML(src, name string) (Server, error) {
	var out Server
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(src))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[] ")
			continue
		}
		if !strings.HasPrefix(section, "mcp_servers."+name) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if strings.HasSuffix(section, ".env_http_headers") {
			if out.EnvHTTPHeaders == nil {
				out.EnvHTTPHeaders = map[string]string{}
			}
			out.EnvHTTPHeaders[unquote(key)] = unquote(val)
			continue
		}
		if strings.HasSuffix(section, ".http_headers") {
			if out.HTTPHeaders == nil {
				out.HTTPHeaders = map[string]string{}
			}
			out.HTTPHeaders[unquote(key)] = unquote(val)
			continue
		}
		switch key {
		case "command":
			out.Command = unquote(val)
		case "args":
			out.Args = parseStringArray(val)
		case "url":
			out.URL = unquote(val)
		case "bearer_token_env_var":
			out.BearerTokenEnvVar = unquote(val)
		case "http_headers":
			out.HTTPHeaders = parseInlineMap(val)
		case "env_http_headers":
			out.EnvHTTPHeaders = parseInlineMap(val)
		}
	}
	if err := scanner.Err(); err != nil {
		return out, err
	}
	if out.Command == "" && out.URL == "" {
		return out, fmt.Errorf("mcp_servers.%s has neither command nor url", name)
	}
	return out, nil
}
func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') {
		if v, e := strconv.Unquote(s); e == nil {
			return v
		}
		return s[1 : len(s)-1]
	}
	return s
}
func parseStringArray(s string) []string {
	var a []string
	inside := false
	start := 0
	for i, r := range s {
		if r == '"' && !inside {
			inside = true
			start = i + 1
		} else if r == '"' && inside {
			a = append(a, s[start:i])
			inside = false
		}
	}
	return a
}
func parseInlineMap(s string) map[string]string {
	m := map[string]string{}
	s = strings.Trim(strings.TrimSpace(s), "{}")
	for _, entry := range strings.Split(s, ",") {
		p := strings.SplitN(entry, "=", 2)
		if len(p) == 2 {
			m[unquote(strings.TrimSpace(p[0]))] = unquote(strings.TrimSpace(p[1]))
		}
	}
	return m
}

func Start(s Server, name string) (*Client, error) {
	if s.URL != "" {
		return StartRemote(s, name)
	}
	cmd := exec.Command(s.Command, s.Args...)
	cmd.Env = os.Environ()
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+expandEnv(v))
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, stdin: in, stdout: bufio.NewReader(out), next: 1, name: name}
	if _, err = c.call("initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "kdrama120-local-agent", "version": "0.1.0"}}); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}
	if err = c.notify("notifications/initialized", map[string]any{}); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}
func StartRemote(s Server, name string) (*Client, error) {
	u, err := url.Parse(s.URL)
	if err != nil || u.Scheme != "https" {
		return nil, fmt.Errorf("MCP URL must use HTTPS")
	}
	headers := map[string]string{}
	for k, v := range s.HTTPHeaders {
		headers[k] = expandEnv(v)
	}
	for k, env := range s.EnvHTTPHeaders {
		value := os.Getenv(env)
		if value == "" {
			return nil, fmt.Errorf("required MCP header environment variable %s is empty", env)
		}
		headers[k] = value
	}
	if s.BearerTokenEnvVar != "" {
		token := os.Getenv(s.BearerTokenEnvVar)
		if token == "" {
			return nil, fmt.Errorf("required MCP bearer-token environment variable %s is empty", s.BearerTokenEnvVar)
		}
		headers["Authorization"] = "Bearer " + token
	}
	c := &Client{next: 1, name: name, endpoint: s.URL, headers: headers, httpClient: &http.Client{Timeout: 90 * time.Second}}
	if _, err = c.call("initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]string{"name": "kdrama120-local-agent", "version": "0.1.0"}}); err != nil {
		return nil, fmt.Errorf("remote MCP initialize failed: %w", err)
	}
	if err = c.notify("notifications/initialized", map[string]any{}); err != nil {
		return nil, err
	}
	return c, nil
}
func expandEnv(s string) string { return os.Expand(s, func(k string) string { return os.Getenv(k) }) }

func (c *Client) CallTool(name string, args map[string]any) (json.RawMessage, error) {
	v, err := c.call("tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
func (c *Client) ListTools() (json.RawMessage, error) {
	v, err := c.call("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}
func (c *Client) call(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.next
	c.next++
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if c.endpoint != "" {
		return c.remoteRequest(req)
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err = c.stdin.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	for {
		line, e := c.stdout.ReadBytes('\n')
		if e != nil {
			return nil, e
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var resp struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &resp) != nil || len(resp.ID) == 0 {
			continue
		}
		var got int
		if json.Unmarshal(resp.ID, &got) != nil || got != id {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("MCP %s (%d): %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}
func (c *Client) notify(method string, params any) error {
	if c.endpoint != "" {
		c.mu.Lock()
		defer c.mu.Unlock()
		_, err := c.remoteRequest(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
		return err
	}
	b, e := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	if e != nil {
		return e
	}
	_, e = c.stdin.Write(append(b, '\n'))
	return e
}
func (c *Client) remoteRequest(req rpcRequest) (json.RawMessage, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Accept", "application/json, text/event-stream")
	hreq.Header.Set("MCP-Protocol-Version", "2024-11-05")
	for k, v := range c.headers {
		hreq.Header.Set(k, v)
	}
	if c.session != "" {
		hreq.Header.Set("Mcp-Session-Id", c.session)
	}
	resp, err := c.httpClient.Do(hreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("MCP HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.session = sid
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return json.RawMessage("null"), nil
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		for _, line := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(line, "data:") {
				body = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				break
			}
		}
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("MCP %s (%d): %s", req.Method, envelope.Error.Code, envelope.Error.Message)
	}
	return envelope.Result, nil
}
func (c *Client) Close() error {
	if c.endpoint != "" {
		return nil
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	_ = c.cmd.Process.Kill()
	return c.cmd.Wait()
}
func ValidateTools(raw json.RawMessage) error {
	var r struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if e := json.Unmarshal(raw, &r); e != nil {
		return e
	}
	found := map[string]bool{}
	for _, t := range r.Tools {
		found[t.Name] = true
	}
	if !found["abocado_get_history"] || !found["abocado_get_job_status"] {
		return errors.New("configured MCP server does not expose Abocado history/status tools")
	}
	return nil
}

