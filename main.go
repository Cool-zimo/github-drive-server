package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const version = "2.3.0"

var (
	listenPort      = "8787"
	startTime       = time.Now()
	requestCount    = 0
	requestLogs     = make([]LogEntry, 0, 100)
	logMutex        sync.Mutex
	// OAuth 配置（从环境变量读取）
	oauthClientID     = os.Getenv("GITHUB_CLIENT_ID")
	oauthClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	oauthRedirectURI  = os.Getenv("GITHUB_REDIRECT_URI")
)

type LogEntry struct {
	Time   string `json:"time"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	IP     string `json:"ip"`
}

func main() {
	if p := os.Getenv("PORT"); p != "" {
		listenPort = p
	}
	mux := http.NewServeMux()
	// 公开接口
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/api/status", handleStatus)
	// 管理接口
	mux.HandleFunc("/api/logs", handleLogs)
	mux.HandleFunc("/api/shutdown", handleShutdown)
	mux.HandleFunc("/api/self/path", handleSelfPath)
	mux.HandleFunc("/api/update", handleUpdate)
	// 通用网络请求（公开，本地运行风险低）
	mux.HandleFunc("/api/request", handleAPIRequest)
	mux.HandleFunc("/proxy", handleProxy)
	mux.HandleFunc("/bilibili", handleBilibili)
	mux.HandleFunc("/bilibili/download", handleBilibiliDownload)
	// 需要授权的接口
	mux.HandleFunc("/api/fs/list", handleFSList)
	mux.HandleFunc("/api/fs/read", handleFSRead)
	mux.HandleFunc("/api/fs/write", handleFSWrite)
	mux.HandleFunc("/api/fs/delete", handleFSDelete)
	mux.HandleFunc("/api/fs/mkdir", handleFSMkdir)
	mux.HandleFunc("/api/fs/stat", handleFSStat)
	mux.HandleFunc("/api/exec", handleExec)
	// GitHub OAuth
	mux.HandleFunc("/api/oauth/url", handleOAuthURL)
	mux.HandleFunc("/api/oauth/login", handleOAuthLogin)
	mux.HandleFunc("/api/oauth/callback", handleOAuthCallback)

	// 日志中间件
	loggedMux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &statusRecorder{ResponseWriter: w, status: 200}
		mux.ServeHTTP(wrapped, r)
		if r.URL.Path != "/health" && r.URL.Path != "/api/status" {
			addLog(r, wrapped.status)
		}
	})

	server := &http.Server{
		Addr:         ":" + listenPort,
		Handler:      loggedMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://localhost:" + listenPort)
	}()

	fmt.Printf("GitHub Drive 后端服务 v%s\n", version)
	fmt.Printf("正在启动... http://localhost:%s\n", listenPort)
	fmt.Printf("按 Ctrl+C 停止服务\n\n")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start()
}

// ==================== 状态接口 ====================

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func addLog(r *http.Request, status int) {
	logMutex.Lock()
	defer logMutex.Unlock()
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		ip = ip[:idx]
	}
	entry := LogEntry{
		Time:   time.Now().Format("15:04:05"),
		Method: r.Method,
		Path:   r.URL.Path,
		Status: status,
		IP:     ip,
	}
	requestLogs = append(requestLogs, entry)
	if len(requestLogs) > 100 {
		requestLogs = requestLogs[1:]
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	exePath, _ := os.Executable()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "running",
		"version":      version,
		"port":         listenPort,
		"uptime":       int(time.Since(startTime).Seconds()),
		"requestCount": requestCount,
		"platform":     runtime.GOOS + "/" + runtime.GOARCH,
		"exePath":      exePath,
		"logCount":     len(requestLogs),
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "running",
		"version": version,
	})
}

// ==================== Web 控制面板 ====================

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	requestCount++
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>GitHub Drive 后端服务</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px}
.card{background:#fff;border-radius:20px;box-shadow:0 20px 60px rgba(0,0,0,.3);width:100%;max-width:520px;overflow:hidden}
.header{background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);color:#fff;padding:28px;text-align:center}
.header .icon{font-size:48px;margin-bottom:10px}
.header h1{font-size:20px;margin-bottom:4px}
.header p{opacity:.9;font-size:12px}
.body{padding:20px}
.auth-box{background:linear-gradient(135deg,#fef3c7,#fde68a);border-radius:12px;padding:16px;margin-bottom:16px;text-align:center}
.auth-label{font-size:11px;color:#92400e;text-transform:uppercase;letter-spacing:1px;margin-bottom:6px}
.auth-code{font-size:32px;font-weight:700;color:#92400e;letter-spacing:8px;font-family:monospace}
.auth-hint{font-size:11px;color:#b45309;margin-top:6px}
.status-row{display:flex;align-items:center;justify-content:space-between;padding:10px 0;border-bottom:1px solid #f3f4f6}
.status-row:last-child{border-bottom:none}
.status-label{font-size:12px;color:#6b7280}
.status-value{font-size:13px;font-weight:600;color:#111827}
.status-value.ok{color:#16a34a}
.dot{display:inline-block;width:8px;height:8px;border-radius:50%;background:#16a34a;margin-right:6px;animation:pulse 2s infinite}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}
.btn{display:block;width:100%;padding:12px;border:none;border-radius:10px;font-size:14px;font-weight:600;cursor:pointer;margin-top:12px;text-decoration:none;text-align:center;transition:transform .15s}
.btn:hover{transform:translateY(-1px)}
.btn-primary{background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);color:#fff}
.btn-secondary{background:#f3f4f6;color:#374151}
.api-list{margin-top:12px;padding:12px;background:#f9fafb;border-radius:8px}
.api-list h4{font-size:11px;color:#6b7280;margin-bottom:8px;text-transform:uppercase}
.api-item{font-size:11px;color:#374151;padding:3px 0;font-family:monospace}
.api-item .method{display:inline-block;min-width:42px;font-weight:600}
.method-get{color:#16a34a}.method-post{color:#2563eb}.method-del{color:#dc2626}
.footer{padding:14px 20px;background:#f9fafb;text-align:center;font-size:10px;color:#9ca3af}
</style>
</head>
<body>
<div class="card">
<div class="header">
<div class="icon">🚀</div>
<h1>GitHub Drive 后端服务</h1>
<p>v%s · 本地能力平台</p>
</div>
<div class="body">
<div class="auth-box">
<div class="auth-label">GitHub Drive 授权码</div>
<div class="auth-hint" style="font-size:13px;color:#92400e;">所有 API 已开放，无需授权码</div>
</div>
<div class="status-row"><span class="status-label">服务状态</span><span class="status-value ok"><span class="dot"></span>运行中</span></div>
<div class="status-row"><span class="status-label">端口</span><span class="status-value">%s</span></div>
<div class="status-row"><span class="status-label">运行时长</span><span class="status-value" id="uptime">--</span></div>
<div class="status-row"><span class="status-label">请求次数</span><span class="status-value" id="reqCount">--</span></div>
<div class="status-row"><span class="status-label">已授权客户端</span><span class="status-value" id="authCount">0</span></div>
<a href="https://cool-zimo.github.io/github_drive" target="_blank" class="btn btn-primary">📁 打开 GitHub Drive</a>
<div class="api-list">
<h4>可用 API</h4>
<div class="api-item"><span class="method method-get">GET</span> /api/request - 通用网络请求</div>
<div class="api-item"><span class="method method-get">GET</span> /api/fs/list - 列出目录</div>
<div class="api-item"><span class="method method-get">GET</span> /api/fs/read - 读取文件</div>
<div class="api-item"><span class="method method-post">POST</span> /api/fs/write - 写入文件</div>
<div class="api-item"><span class="method method-post">POST</span> /api/fs/mkdir - 创建目录</div>
<div class="api-item"><span class="method method-del">DEL</span> /api/fs/delete - 删除文件</div>
<div class="api-item"><span class="method method-post">POST</span> /api/exec - 执行命令</div>
</div>
</div>
<div class="footer">本地运行 · 不上传任何数据 · 关闭窗口停止服务</div>
</div>
<script>
function updateStatus(){fetch('/api/status').then(r=>r.json()).then(d=>{
document.getElementById('uptime').textContent=fmt(d.uptime);
document.getElementById('reqCount').textContent=d.requestCount;
document.getElementById('authCount').textContent=d.authorizedClients;
}).catch(()=>{});}
function fmt(s){var h=Math.floor(s/3600),m=Math.floor((s%%3600)/60),sec=s%%60;return(h>0?h+'时':'')+(m>0?m+'分':'')+sec+'秒';}
updateStatus();setInterval(updateStatus,3000);
</script>
</body>
</html>`, version, listenPort)
}

// ==================== 通用网络请求 ====================

func handleAPIRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	requestCount++

	var req struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
		Timeout int               `json:"timeout"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "请求体解析失败: " + err.Error()})
		return
	}
	if req.URL == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 url"})
		return
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	timeout := 30
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}
	httpReq, err := http.NewRequest(strings.ToUpper(req.Method), req.URL, bodyReader)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "请求创建失败: " + err.Error()})
		return
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "GitHub-Drive-Backend/"+version)
	}

	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": "请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  resp.StatusCode,
		"headers": respHeaders,
		"body":    string(respBody),
	})
}

// ==================== 兼容旧接口 ====================

func handleProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	requestCount++
	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		http.Error(w, "缺少 url 参数", http.StatusBadRequest)
		return
	}
	decoded, err := url.QueryUnescape(targetURL)
	if err == nil {
		targetURL = decoded
	}
	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "请求创建失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	for key, values := range r.Header {
		if strings.ToLower(key) == "host" || strings.ToLower(key) == "origin" || strings.ToLower(key) == "referer" {
			continue
		}
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "代理请求失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for key, values := range resp.Header {
		if strings.ToLower(key) == "access-control-allow-origin" {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func handleBilibili(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	videoURL := r.URL.Query().Get("url")
	bvid := r.URL.Query().Get("bvid")
	qn := r.URL.Query().Get("qn")
	if qn == "" {
		qn = "64"
	}
	if videoURL == "" && bvid == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 url 或 bvid"})
		return
	}
	if videoURL != "" && bvid == "" {
		bvid = extractBV(videoURL)
	}
	if bvid == "" {
		json.NewEncoder(w).Encode(map[string]string{"error": "无法提取 BV 号"})
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	infoReq, _ := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/view?bvid="+bvid, nil)
	infoReq.Header.Set("User-Agent", "Mozilla/5.0")
	infoReq.Header.Set("Referer", "https://www.bilibili.com")
	infoResp, err := client.Do(infoReq)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{"error": "获取视频信息失败: " + err.Error()})
		return
	}
	defer infoResp.Body.Close()
	var infoResult map[string]interface{}
	json.NewDecoder(infoResp.Body).Decode(&infoResult)
	if infoResult["code"].(float64) != 0 {
		json.NewEncoder(w).Encode(map[string]string{"error": "视频不存在"})
		return
	}
	infoData := infoResult["data"].(map[string]interface{})
	cid := fmt.Sprintf("%.0f", infoData["cid"].(float64))
	playReq, _ := http.NewRequest("GET", "https://api.bilibili.com/x/player/playurl?bvid="+bvid+"&cid="+cid+"&qn="+qn+"&fnval=16", nil)
	playReq.Header.Set("User-Agent", "Mozilla/5.0")
	playReq.Header.Set("Referer", "https://www.bilibili.com")
	playResp, err := client.Do(playReq)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"title": infoData["title"], "error": "获取播放地址失败"})
		return
	}
	defer playResp.Body.Close()
	var playResult map[string]interface{}
	json.NewDecoder(playResp.Body).Decode(&playResult)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"title": infoData["title"], "pic": infoData["pic"], "desc": infoData["desc"],
		"bvid": bvid, "owner": infoData["owner"], "stat": infoData["stat"],
		"play": playResult["data"],
	})
}

func handleBilibiliDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	bvid := r.URL.Query().Get("bvid")
	if videoURL := r.URL.Query().Get("url"); videoURL != "" && bvid == "" {
		bvid = extractBV(videoURL)
	}
	if bvid == "" {
		http.Error(w, "无法提取 BV 号", http.StatusBadRequest)
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}
	infoReq, _ := http.NewRequest("GET", "https://api.bilibili.com/x/web-interface/view?bvid="+bvid, nil)
	infoReq.Header.Set("User-Agent", "Mozilla/5.0")
	infoReq.Header.Set("Referer", "https://www.bilibili.com")
	infoResp, err := client.Do(infoReq)
	if err != nil {
		http.Error(w, "获取信息失败", http.StatusBadGateway)
		return
	}
	defer infoResp.Body.Close()
	var infoResult map[string]interface{}
	json.NewDecoder(infoResp.Body).Decode(&infoResult)
	if infoResult["code"].(float64) != 0 {
		http.Error(w, "视频不存在", http.StatusNotFound)
		return
	}
	infoData := infoResult["data"].(map[string]interface{})
	cid := fmt.Sprintf("%.0f", infoData["cid"].(float64))
	title := infoData["title"].(string)
	qn := r.URL.Query().Get("qn")
	if qn == "" {
		qn = "64"
	}
	playReq, _ := http.NewRequest("GET", "https://api.bilibili.com/x/player/playurl?bvid="+bvid+"&cid="+cid+"&qn="+qn, nil)
	playReq.Header.Set("User-Agent", "Mozilla/5.0")
	playReq.Header.Set("Referer", "https://www.bilibili.com")
	playResp, err := client.Do(playReq)
	if err != nil {
		http.Error(w, "获取播放地址失败", http.StatusBadGateway)
		return
	}
	defer playResp.Body.Close()
	var playResult map[string]interface{}
	json.NewDecoder(playResp.Body).Decode(&playResult)
	playData := playResult["data"].(map[string]interface{})
	durl := playData["durl"].([]interface{})
	if len(durl) == 0 {
		http.Error(w, "未找到视频地址", http.StatusNotFound)
		return
	}
	firstURL := durl[0].(map[string]interface{})["url"].(string)
	videoReq, _ := http.NewRequest("GET", firstURL, nil)
	videoReq.Header.Set("User-Agent", "Mozilla/5.0")
	videoReq.Header.Set("Referer", "https://www.bilibili.com")
	videoResp, err := client.Do(videoReq)
	if err != nil {
		http.Error(w, "下载失败", http.StatusBadGateway)
		return
	}
	defer videoResp.Body.Close()
	safeTitle := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_").Replace(title)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.mp4\"", safeTitle))
	io.Copy(w, videoResp.Body)
}

// ==================== 文件系统操作 ====================

func handleFSList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	entries, err := os.ReadDir(absPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	files := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		info, _ := entry.Info()
		files = append(files, map[string]interface{}{
			"name":    entry.Name(),
			"isDir":   entry.IsDir(),
			"size":    func() int64 { if info != nil { return info.Size() }; return 0 }(),
			"modTime": func() string { if info != nil { return info.ModTime().Format(time.RFC3339) }; return "" }(),
		})
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":  absPath,
		"files": files,
	})
}

func handleFSRead(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	path := r.URL.Query().Get("path")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 path"})
		return
	}
	absPath, _ := filepath.Abs(path)
	data, err := os.ReadFile(absPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    absPath,
		"size":    len(data),
		"content": base64.StdEncoding.EncodeToString(data),
	})
}

func handleFSWrite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Encode  string `json:"encode"` // "base64" or "text"
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 path"})
		return
	}
	absPath, _ := filepath.Abs(req.Path)
	os.MkdirAll(filepath.Dir(absPath), 0755)
	var data []byte
	if req.Encode == "base64" {
		var err error
		data, err = base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "base64 解码失败: " + err.Error()})
			return
		}
	} else {
		data = []byte(req.Content)
	}
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path": absPath,
		"size": len(data),
	})
}

func handleFSDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	var req struct {
		Path string `json:"path"`
	}
	if r.Method == "GET" {
		req.Path = r.URL.Query().Get("path")
	} else {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 path"})
		return
	}
	absPath, _ := filepath.Abs(req.Path)
	if err := os.RemoveAll(absPath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": absPath})
}

func handleFSMkdir(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	var req struct {
		Path string `json:"path"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 path"})
		return
	}
	absPath, _ := filepath.Abs(req.Path)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": absPath})
}

func handleFSStat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	path := r.URL.Query().Get("path")
	if path == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 path"})
		return
	}
	absPath, _ := filepath.Abs(path)
	info, err := os.Stat(absPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    absPath,
		"name":    info.Name(),
		"isDir":   info.IsDir(),
		"size":    info.Size(),
		"modTime": info.ModTime().Format(time.RFC3339),
		"mode":    info.Mode().String(),
	})
}

// ==================== 命令执行 ====================

func handleExec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	requestCount++
	var req struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
		Cwd     string   `json:"cwd"`
		Timeout int      `json:"timeout"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Command == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "缺少 command"})
		return
	}
	timeout := 30
	if req.Timeout > 0 {
		timeout = req.Timeout
	}
	cmd := exec.Command(req.Command, req.Args...)
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		code := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				code = -1
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stdout": stdout.String(),
			"stderr": stderr.String(),
			"code":   code,
		})
	case <-time.After(time.Duration(timeout) * time.Second):
		cmd.Process.Kill()
		w.WriteHeader(http.StatusRequestTimeout)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stdout": stdout.String(),
			"stderr": stderr.String(),
			"error":  "命令执行超时",
			"code":   -1,
		})
	}
}

// ==================== 管理接口 ====================

func handleLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	logMutex.Lock()
	defer logMutex.Unlock()
	// 返回倒序（最新的在前）
	reversed := make([]LogEntry, len(requestLogs))
	for i, e := range requestLogs {
		reversed[len(requestLogs)-1-i] = e
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  reversed,
		"total": len(requestLogs),
	})
}

func handleSelfPath(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	exePath, _ := os.Executable()
	json.NewEncoder(w).Encode(map[string]string{
		"path": exePath,
		"dir":  filepath.Dir(exePath),
	})
}

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "shutting_down"})
	go func() {
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	exePath, err := os.Executable()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 确定下载 URL
	baseURL := "https://ghproxy.com/https://raw.githubusercontent.com/Cool-zimo/github-drive-server/main/dist/"
	var fileName string
	switch runtime.GOOS {
	case "windows":
		fileName = "github-drive-server-v2.2.0-windows.exe"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			fileName = "github-drive-server-v2.2.0-macos-apple"
		} else {
			fileName = "github-drive-server-v2.2.0-macos-intel"
		}
	default:
		if runtime.GOARCH == "arm64" {
			fileName = "github-drive-server-v2.2.0-linux-arm64"
		} else {
			fileName = "github-drive-server-v2.2.0-linux"
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "updating",
		"currentPath": exePath,
		"downloadUrl": baseURL + fileName,
		"message":     "正在下载新版本，完成后请手动重启服务",
	})

	// 后台下载新版本
	go func() {
		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Get(baseURL + fileName)
		if err != nil {
			log.Println("更新下载失败:", err)
			return
		}
		defer resp.Body.Close()
		tmpPath := exePath + ".new"
		out, err := os.Create(tmpPath)
		if err != nil {
			log.Println("创建临时文件失败:", err)
			return
		}
		io.Copy(out, resp.Body)
		out.Close()
		os.Chmod(tmpPath, 0755)
		// 替换旧文件（Windows 需要先退出进程）
		os.Rename(tmpPath, exePath)
		log.Println("更新完成，请重启服务")
	}()
}

func extractBV(s string) string {
	for i := 0; i+12 <= len(s); i++ {
		if s[i:i+2] == "BV" {
			end := i + 2
			for end < len(s) && isAlnum(s[end]) {
				end++
			}
			if end-i >= 12 {
				return s[i:end]
			}
		}
	}
	return ""
}

func isAlnum(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// ==================== GitHub OAuth ====================

func handleOAuthURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	if oauthClientID == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
			"error":   "OAuth not configured. Please set GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, GITHUB_REDIRECT_URI environment variables.",
		})
		return
	}
	
	redirectURI := oauthRedirectURI
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("http://localhost:%s/api/oauth/callback", listenPort)
	}
	
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=repo,workflow&state=gd-%d",
		oauthClientID,
		url.QueryEscape(redirectURI),
		time.Now().Unix(),
	)
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":  true,
		"auth_url": authURL,
	})
}

func handleOAuthLogin(w http.ResponseWriter, r *http.Request) {
	if oauthClientID == "" {
		http.Error(w, "OAuth not configured", http.StatusServiceUnavailable)
		return
	}
	
	redirectURI := oauthRedirectURI
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("http://localhost:%s/api/oauth/callback", listenPort)
	}
	
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=repo,workflow&state=gd-%d",
		oauthClientID,
		url.QueryEscape(redirectURI),
		time.Now().Unix(),
	)
	
	http.Redirect(w, r, authURL, http.StatusFound)
}

func handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}
	
	// 用 code 换 access token
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{
		"client_id":     {oauthClientID},
		"client_secret": {oauthClientSecret},
		"code":          {code},
	}
	
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Failed to exchange code for token", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	
	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	
	if result.Error != "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<html><body><script>
			window.opener.postMessage({type:'gd-oauth',error:'%s'},'*');
			window.close();
		</script><p>OAuth failed: %s</p></body></html>`, result.ErrorDesc, result.ErrorDesc)
		return
	}
	
	// 返回 HTML 页面，通过 postMessage 把 token 传给前端窗口
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>GitHub Drive - Login Success</title>
<style>
body{font-family:-apple-system,sans-serif;background:linear-gradient(135deg,#667eea,#764ba2);min-height:100vh;display:flex;align-items:center;justify-content:center;margin:0}
.card{background:#fff;border-radius:16px;padding:40px;text-align:center;box-shadow:0 20px 60px rgba(0,0,0,.3)}
.icon{font-size:48px;margin-bottom:16px}
h1{color:#111827;margin:0 0 8px}
p{color:#6b7280;margin:0}
</style></head>
<body>
<div class="card">
<div class="icon">✅</div>
<h1>Login Successful!</h1>
<p>You can close this window now.</p>
</div>
<script>
(function(){
	var token = '%s';
	var msg = {type:'gd-oauth',token:token};
	// 发送给 opener（主窗口）
	if(window.opener){
		window.opener.postMessage(msg,'*');
	}
	// 也广播给所有窗口
	window.postMessage(msg,'*');
	// 1秒后自动关闭
	setTimeout(function(){window.close();},1500);
})();
</script>
</body></html>`, result.AccessToken)
}
