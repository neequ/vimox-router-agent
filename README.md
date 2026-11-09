# Vimox Router Agent

Open-source Copilot CLI and MCP server for creative work with [Vimox Router](https://router.vimox.cn). The CLI can register or sign in to a Router account, create an API key, browse models, create text/image/video work, and inspect asynchronous tasks. The MCP server exposes the same creative workflows to MCP-compatible agents.

## Install

Download the archive for your operating system from GitHub Releases, then put `vimox-router-agent` on `PATH`.

```sh
vimox-router-agent version
vimox-router-agent account captcha --out captcha.svg
```

For source builds, install Go 1.23 or later and run `go install github.com/neequ/vimox-router-agent/cmd/vimox-router-agent@latest`.

## Account and API key

The CLI uses Router's existing account endpoints. It does not store the account password or the temporary account access token. Registration sends a verification code, then exchanges the code for an account session and creates a Router API key. Phone registration requires the Router captcha first; email registration does not.

```sh
# Get a captcha when registering or logging in with a phone number.
vimox-router-agent account captcha --out captcha.svg

# Send registration code. For phone, include the captcha values.
vimox-router-agent account send-code --channel email --identifier you@example.com
vimox-router-agent account send-code --channel phone --identifier 13800138000 --captcha-id ID --captcha-answer ABCDE

# Register or sign in; the CLI asks for the password without echoing it.
vimox-router-agent account register --channel email --identifier you@example.com --code 123456
vimox-router-agent account login --channel email --identifier you@example.com

vimox-router-agent account status
vimox-router-agent account logout
```

The created API key is saved in the operating system user config directory with owner-only file permissions where supported. You can instead set `VIMOX_ROUTER_API_KEY` for ephemeral or CI use. Never commit a key or paste it into an issue.

## Creative commands

```sh
vimox-router-agent models list
vimox-router-agent create text --model gpt-6.1-sol --prompt "Write three campaign concepts for a tea launch" --confirm
vimox-router-agent create image --model vimox-image --prompt "A product photograph of a ceramic tea cup" --size 1024x1024 --confirm
vimox-router-agent create video --model seedance-2.0 --prompt "A slow orbit around the product" --seconds 5 --confirm
vimox-router-agent task get TASK_ID
```

Generation commands submit paid work. Review the model, prompt, and parameters before running them. The CLI returns Router's response and preserves upstream error messages and HTTP status.

## MCP

The self-hosted server speaks Streamable HTTP at `/mcp` and serves its bilingual connection page at `/`. Configure `VIMOX_ROUTER_API_KEY` on the host; do not put a key in a public URL or MCP configuration file. MCP clients send the key as `Authorization: Bearer ...`.

```sh
VIMOX_ROUTER_API_KEY=sk-your-key vimox-router-agent serve --listen 127.0.0.1:8787
```

Example reverse proxy location:

```nginx
location = /mcp {
    proxy_pass http://127.0.0.1:8787/mcp;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_read_timeout 3600s;
}
```

The intended public URL is `https://router.vimox.cn/mcp`. The Router domain must route `/mcp` and the MCP landing page to this service before that URL is available. Until that deployment is made, use a privately hosted endpoint. The MCP connection page can be previewed from the service root.

MCP tools: `router_models_list`, `router_text_create`, `router_image_create`, `router_video_create`, and `router_task_get`. Generation tools submit paid work and require an explicit `confirmed: true` argument.

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `VIMOX_ROUTER_BASE_URL` | `https://router.vimox.cn` | Router origin |
| `VIMOX_ROUTER_API_KEY` | saved account key | Router API key |
| `VIMOX_ROUTER_CONFIG` | OS user config path | CLI credential file |
| `VIMOX_ROUTER_LISTEN` | `127.0.0.1:8787` | MCP server bind address |
| `VIMOX_ROUTER_CORS_ORIGINS` | empty | Comma-separated browser origins allowed to call the HTTP MCP endpoint |

## Development

```sh
gofmt -w cmd internal
go test ./...
go run ./cmd/vimox-router-agent version
```

## License

MIT. See [LICENSE](LICENSE).
