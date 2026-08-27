# GitHub Drive 后端服务 v2.0

GitHub Drive 的本地能力平台。**双击运行，自动打开浏览器控制面板**，为插件提供网络请求、文件操作、命令执行等浏览器做不到的能力。

## ✨ 能力

| 能力 | 说明 | 需授权 |
|---|---|---|
| 🌐 通用网络请求 | 任意 HTTP 请求（GET/POST/PUT/DELETE），自定义 headers/body | 否 |
| 📁 本地文件操作 | 列出目录、读写文件、创建目录、删除文件 | 是 |
| ⚡ 命令执行 | 执行任意系统命令，带超时控制 | 是 |
| 📺 B站视频解析 | 内置 B站视频解析和下载 | 否 |
| 🔄 CORS 代理 | 通用反向代理 | 否 |

## 📦 下载

| 系统 | 文件 |
|---|---|
| Windows | [github-drive-server-v2.2.0-windows.exe](https://cool-zimo.github.io/github-drive-server/dist/github-drive-server-v2.2.0-windows.exe) |
| macOS (Intel) | [github-drive-server-v2.2.0-macos-intel](https://cool-zimo.github.io/github-drive-server/dist/github-drive-server-v2.2.0-macos-intel) |
| macOS (M1/M2/M3) | [github-drive-server-v2.2.0-macos-apple](https://cool-zimo.github.io/github-drive-server/dist/github-drive-server-v2.2.0-macos-apple) |
| Linux | [github-drive-server-linux](https://cool-zimo.github.io/github-drive-server/dist/github-drive-server-v2.2.0-linux) |

## 🚀 使用

1. **下载**对应系统的文件
2. **双击运行** → 浏览器自动打开控制面板（显示6位授权码）
3. 在 GitHub Drive 设置中输入授权码（只需一次）
4. 插件即可使用全部能力

## 🔐 授权机制

- 启动时生成随机 **6位授权码**，显示在控制面板
- 在 GitHub Drive 中输入一次，token 保存在本地
- 文件操作和命令执行需要授权，网络请求不需要
- 关闭后端窗口即停止服务，授权码下次启动会重新生成

## 📡 API 文档

### 通用网络请求
```
POST /api/request
Body: { "method": "GET", "url": "https://...", "headers": {}, "body": "", "timeout": 30 }
Return: { "status": 200, "headers": {}, "body": "..." }
```

### 文件系统（需 X-Auth-Token）
```
GET  /api/fs/list?path=/home/user     - 列出目录
GET  /api/fs/read?path=/home/user/f   - 读取文件（base64）
POST /api/fs/write                    - 写入文件 { path, content, encode: "base64"|"text" }
POST /api/fs/mkdir                    - 创建目录 { path }
POST /api/fs/delete                   - 删除文件/目录 { path }
GET  /api/fs/stat?path=...            - 文件信息
```

### 命令执行（需 X-Auth-Token）
```
POST /api/exec
Body: { "command": "echo", "args": ["hello"], "cwd": "", "timeout": 30 }
Return: { "stdout": "...", "stderr": "...", "code": 0 }
```

### 授权
```
GET  /api/auth/code     - 获取授权码
POST /api/auth/verify   - 验证授权码 { code } → 返回 token
GET  /api/auth/status   - 检查授权状态
```

### 其他
```
GET /health       - 健康检查
GET /api/status   - 详细状态
GET /proxy?url=   - CORS 代理（兼容旧版）
GET /bilibili?url= - B站视频解析
```

## 🔧 开发插件

```javascript
const BACKEND = 'http://localhost:8787';
const TOKEN = localStorage.getItem('gd_backend_token'); // 从 GitHub Drive 获取

// 通用网络请求
const resp = await fetch(`${BACKEND}/api/request`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ method: 'GET', url: 'https://api.example.com/data' })
});

// 读取本地文件
const file = await fetch(`${BACKEND}/api/fs/read?path=${encodeURIComponent('/path/to/file')}`, {
  headers: { 'X-Auth-Token': TOKEN }
});

// 执行命令
const result = await fetch(`${BACKEND}/api/exec`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', 'X-Auth-Token': TOKEN },
  body: JSON.stringify({ command: 'ls', args: ['-la'], timeout: 10 })
});
```

## 📱 在手机上运行（Android / 鸿蒙）

1. 安装 [Termux](https://f-droid.org/packages/com.termux/)（从 F-Droid 下载，不要用 Google Play 版）
2. 打开 Termux，执行：
```bash
pkg update && pkg install wget -y
wget https://ghproxy.com/https://raw.githubusercontent.com/Cool-zimo/github-drive-server/main/dist/github-drive-server-v2.2.0-linux-arm64 -O gd-server
chmod +x gd-server
./gd-server
```
3. 浏览器自动打开控制面板（如未打开，手动访问 `http://localhost:8787`）
4. 在 GitHub Drive 中输入授权码即可

> 鸿蒙手机（兼容 Android 模式）同样可用 Termux。HarmonyOS NEXT（纯血鸿蒙）暂不支持。

## 📝 注意

- 仅在本地运行，**不上传任何数据**
- 命令执行有风险，请只授权可信的插件
- macOS 首次运行需在「系统设置 → 隐私与安全性」允许
- 默认端口 8787，可通过环境变量 `PORT` 修改
