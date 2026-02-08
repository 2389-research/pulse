# Pulse - Journal + Social Media MCP Server

A single Go binary that unifies private journaling and social media into one local-first MCP service.

## Names

- **Project Codename**: BRAINWAVE NITRO
- **AI Assistant**: Turbo Gecko
- **Human Lead**: Harp Daddy Supreme

## Architecture

Local-first markdown storage with optional remote API sync for journal and social.
Journal entries use local semantic search via ONNX embeddings.
Repo: https://github.com/2389-research/pulse

### Two storage roots:
- **Journal**: dual-root (project-local `.private-journal/` + user-global `~/.private-journal/`)
- **Social**: `~/.local/share/pulse/social/`

### MCP Tools (7 total):
- `process_thoughts` - Write journal entry with sections
- `search_journal` - Semantic/substring search
- `read_journal_entry` - Read specific entry by path
- `list_recent_entries` - List entries by date
- `login` - Set social media identity
- `create_post` - Create social post (local + optional remote)
- `read_posts` - Read social feed with filtering

## Development

```bash
# Run
go run ./cmd/pulse

# Build
go build -o bin/pulse ./cmd/pulse

# Test
go test ./...

# MCP server
./pulse mcp
```

## Key Paths

- Social data: `~/.local/share/pulse/social/`
- Journal (user): `~/.private-journal/`
- Journal (project): `.private-journal/` (relative to cwd)
- Config: `~/.config/pulse/config.yaml`
- Embedding models: `~/.local/share/pulse/models/`
- Port (if HTTP needed): `7453` (PULS leet-adjacent)

## Config

```yaml
social:
  api_key: ""
  team_id: ""
  api_url: ""
journal:
  project_path: ""
  user_path: ""
```

## Dependencies

- `cobra` - CLI framework
- `go-sdk` - MCP protocol
- `mdstore` - Frontmatter, atomic writes, locking
- `uuid` - ID generation
- `yaml.v3` - Config/frontmatter
- `fastembed-go` - Local ONNX embeddings (Phase 6)
