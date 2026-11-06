package abocado

import "testing"

func TestParseRemoteCodexEntry(t *testing.T) {
	s, err := parseCodexTOML("[mcp_servers.abocado]\nurl = \"https://mcp.abocado.ai/mcp\"\nbearer_token_env_var = \"ABOCADO_TOKEN\"\n[mcp_servers.abocado.env_http_headers]\nX-Api-Key = \"TOKEN_ENV\"\n", "abocado")
	if err != nil {
		t.Fatal(err)
	}
	if s.URL != "https://mcp.abocado.ai/mcp" || s.BearerTokenEnvVar != "ABOCADO_TOKEN" || s.EnvHTTPHeaders["X-Api-Key"] != "TOKEN_ENV" {
		t.Fatalf("config not parsed correctly: %+v", s)
	}
}

func TestParseLocalCodexEntry(t *testing.T) {
	s, err := parseCodexTOML("[mcp_servers.abocado]\ncommand = \"node\"\nargs = [\"server.js\", \"--stdio\"]\n", "abocado")
	if err != nil {
		t.Fatal(err)
	}
	if s.Command != "node" || len(s.Args) != 2 {
		t.Fatalf("stdio config not parsed: %+v", s)
	}
}

func TestParseConfiguredCodexHeaderHelper(t *testing.T) {
	s, err := parseCodexTOML("[mcp_servers.abocado]\nurl = \"https://mcp.abocado.ai/mcp\"\nhttp_headers_helper = [\"C:/tools/auth-helper.exe\", \"--json\"]\n", "abocado")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.HTTPHeadersHelper) != 2 || s.HTTPHeadersHelper[0] != "C:/tools/auth-helper.exe" || s.HTTPHeadersHelper[1] != "--json" {
		t.Fatalf("helper was not parsed: %+v", s.HTTPHeadersHelper)
	}
}

