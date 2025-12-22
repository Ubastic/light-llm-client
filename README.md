# Light LLM Desktop Client

一个轻量级、快速启动、数据本地化的桌面 LLM 聊天客户端（Go + Fyne，非 Electron）。

如果你用过 Cherry Studio、AnythingLLM、ChatGPT-Next-Web 这类工具，但更想要一个“秒开、低内存、随时呼出、离线也能用（Ollama）”的桌面端，那么这个项目可能适合你：它支持 OpenAI 兼容接口，并内置 Claude、Gemini、Ollama，同时把你的聊天记录保存在本地 SQLite 里。

## 适合谁

- 想把“OpenAI Compatible Base URL + API Key”从现有工具搬到一个原生桌面客户端里的人（包括各种 OpenAI 兼容网关/代理）。
- 使用本地模型（Ollama）但不想用浏览器或臃肿客户端的人。
- 需要多对话、多标签、全文搜索、导出备份的人。
- 对隐私敏感，希望数据默认本地存储、日志可匿名化的人。

## 你会得到什么

- 轻量、启动快：目标内存占用 40–60MB，冷启动 < 500ms（持续优化中）。
- 多 Provider：OpenAI 兼容接口、Anthropic Claude、Google Gemini、Ollama（可混用，支持流式输出）。
- 多模态与附件：支持图片与文本文件附件（不同 Provider 以各自格式发送）。
- 本地优先：聊天记录使用 SQLite 保存，内置 FTS5 全文搜索。
- Markdown 原生渲染：基于 Fyne RichText。
- 常用工具链：对话导出/导入（JSON / Markdown），便于备份与迁移。
- 隐私与排障：支持对日志与导出内容进行匿名化（API Key、URL、邮箱、IP、路径等）。
- 桌面体验：多标签、快捷键、设置界面、系统托盘（按配置/平台支持）。

## 快速上手

建议从这两个文档开始：

- 新手安装与运行：`QUICKSTART.md`
- 详细环境搭建：`SETUP.md`

Windows（推荐使用仓库自带脚本）：

```powershell
.\check-env.ps1
.\build.ps1 -Target deps
.\build.ps1 -Target run
```

## 配置：从你现有工具迁移过来

很多桌面/网页端（包括 Cherry Studio、AnythingLLM、ChatGPT-Next-Web 的常见用法）最终都落在“OpenAI 兼容接口”上：只要你手里有 `Base URL`、`API Key`、`Model`，就可以直接在本项目里使用。

配置文件默认位置：

- Windows：`%APPDATA%\light-llm-client\config.json`
- macOS：`~/Library/Application Support/light-llm-client/config.json`
- Linux：`~/.config/light-llm-client/config.json`

也可以通过 `-config` 指定路径：

```bash
.\light-llm-client.exe -config .\my-config.json
```

OpenAI 兼容接口示例（你也可以在“设置”界面里直接填）：

```json
{
  "llm_providers": {
    "openai_like": {
      "display_name": "OpenAI Compatible",
      "api_key": "YOUR_KEY",
      "base_url": "https://example.com/v1",
      "default_model": "gpt-4o-mini",
      "enabled": true
    }
  }
}
```

Ollama 示例：

```json
{
  "llm_providers": {
    "ollama": {
      "display_name": "Ollama",
      "base_url": "http://localhost:11434",
      "default_model": "llama3.2",
      "enabled": true
    }
  }
}
```

提示：仓库的 `config/default.json` 里包含多种 Provider 的可用配置模板（包括 OpenRouter、Kimi 等 OpenAI 兼容入口）。

## 常用功能

- 搜索：内置 SQLite FTS5，全库全文检索对话内容（`ui/search.go`）。
- 导出/导入：对话可导出为 JSON/Markdown，支持批量导入导出（`utils/export.go`）。
- 附件：支持上传图片/文本文件，也支持从剪贴板粘贴截图或复制的文件（Windows 优先，`ui/file_upload.go`）。
- 数据与清理：可设置最大历史条数、按天数清理、Vacuum 优化数据库（设置界面）。
- 隐私：可一键匿名化敏感信息（设置界面，`utils/anonymizer.go`）。

## 构建与开发

前置：

- Go 1.21+
- GCC（Windows 推荐 TDM-GCC 或 MinGW-w64，用于 CGO/SQLite）

从源码运行：

```bash
go mod download
go run .
```

更多开发说明：`DEVELOPMENT.md`。

## 路线图（简版）

- 分叉对话与更强的上下文管理
- 拖拽上传、更多附件类型（如 PDF）
- 进一步优化性能与打包体验

## 贡献

欢迎提交 Issue / Pull Request。对于复现类问题，建议开启匿名化后附上日志或导出文件（避免泄露密钥和隐私信息）。

## 许可证

MIT，详见 `LICENSE`。

