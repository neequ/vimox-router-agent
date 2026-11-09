# Vimox Router Agent

面向 Vimox Router 的开源 Copilot CLI 与 MCP 服务。CLI 支持注册或登录 Router 账户、自动创建并保存 API Key、查询模型、文本创作、图片/视频生成和异步任务查询。MCP 服务把这些创作能力提供给兼容 MCP 的 Agent。

## 安装

从 [GitHub Releases](https://github.com/neequ/vimox-router-agent/releases) 下载对应系统的压缩包，将 `vimox-router-agent` 放入 `PATH`。

也可安装 Go 1.23+ 后运行：

```sh
go install github.com/neequ/vimox-router-agent/cmd/vimox-router-agent@latest
```

## 注册、登录与 API Key

CLI 使用 Router 现有账户接口。账号密码只在内存中用于登录，不会保存；CLI 保存登录后创建的 Router API Key。邮箱注册无需图形验证码，手机号注册前需要先获取图形验证码。

```sh
# 手机注册需要图形验证码；邮箱注册可跳过此步骤
vimox-router-agent account captcha --out captcha.svg
vimox-router-agent account send-code --channel phone --identifier 13800138000 --captcha-id ID --captcha-answer ABCDE

# CLI 在终端中隐藏密码输入
vimox-router-agent account register --channel phone --identifier 13800138000 --code 123456
vimox-router-agent account login --channel email --identifier you@example.com
vimox-router-agent account status
vimox-router-agent account logout
```

API Key 保存于系统用户配置目录；支持的平台会限制为仅当前用户可读。CI 或临时使用可设置 `VIMOX_ROUTER_API_KEY`。不要把密钥提交到代码仓库。

## 创作命令

```sh
vimox-router-agent models list
vimox-router-agent create text --model gpt-6.1-sol --prompt "为茶饮新品写三个推广创意" --confirm
vimox-router-agent create image --model vimox-image --prompt "陶瓷茶杯的产品摄影" --size 1024x1024 --confirm
vimox-router-agent create video --model seedance-2.0 --prompt "镜头环绕产品缓慢移动" --seconds 5 --confirm
vimox-router-agent task get TASK_ID
```

生成命令会产生模型费用。执行前核对模型、提示词和参数；`--confirm` 明确确认本次请求。

## MCP 接入

MCP 服务支持 Streamable HTTP，页面路径为 `/mcp`。登录 Router 后在 API Key 页面创建密钥，将接入地址添加到 MCP 客户端，并将密钥作为 Bearer 凭证配置。CLI 也支持 stdio 模式：

```sh
vimox-router-agent mcp
```

HTTP 服务自托管示例：

```sh
VIMOX_ROUTER_API_KEY=sk-your-key vimox-router-agent serve --listen 127.0.0.1:8787
```

预期公网地址是 `https://router.vimox.cn/mcp`。Router 域名需要将 `/mcp` 路由到该服务后才能使用；完成部署前可先使用自托管地址。浏览器型 MCP 客户端需通过 `VIMOX_ROUTER_CORS_ORIGINS` 配置允许来源。

工具包括模型查询、文本创作、图片生成、视频生成和异步任务查询。所有创作工具都要求 Agent 在用户确认后传入 `confirmed: true`。

## 配置项

| 环境变量 | 默认值 | 用途 |
| --- | --- | --- |
| `VIMOX_ROUTER_BASE_URL` | `https://router.vimox.cn` | Router 服务地址 |
| `VIMOX_ROUTER_API_KEY` | 已保存的账号密钥 | Router API Key |
| `VIMOX_ROUTER_CONFIG` | 系统用户配置目录 | CLI 密钥配置文件 |
| `VIMOX_ROUTER_LISTEN` | `127.0.0.1:8787` | MCP HTTP 监听地址 |
| `VIMOX_ROUTER_CORS_ORIGINS` | 空 | 允许访问 MCP 的浏览器来源，逗号分隔 |

## 开发

```sh
gofmt -w cmd internal
go test ./...
go run ./cmd/vimox-router-agent version
```

## 许可

MIT，见 [LICENSE](LICENSE)。
