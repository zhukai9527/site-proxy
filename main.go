package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ProxyConfig 代表一个代理配置
type ProxyConfig struct {
	Name        string
	URL         string
	Description string
	Tags        []string
	Status      string
	Usage       string
}

// Config 应用配置
type Config struct {
	Port         string
	ProxyConfigs []ProxyConfig
}

// 从环境变量或配置文件加载配置
func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // 默认端口
	}

	var proxyConfigs []ProxyConfig
	var configSource string

	// 检查配置文件路径（支持Docker挂载）
	configPaths := []string{
		"./config.json",                 // 本地开发
		"/app/config.json",              // Docker容器内常见路径
		"/etc/proxy-server/config.json", // 系统配置路径
		os.Getenv("CONFIG_FILE"),        // 环境变量指定路径
	}

	// 尝试从各个可能的配置文件路径读取
	for _, configPath := range configPaths {
		if configPath == "" {
			continue
		}

		if configs := loadConfigFromFile(configPath); len(configs) > 0 {
			proxyConfigs = configs
			configSource = fmt.Sprintf("文件: %s", configPath)
			break
		}
	}

	// 如果从文件加载失败，尝试从环境变量加载
	if len(proxyConfigs) == 0 {
		proxyConfigsStr := os.Getenv("PROXY_CONFIGS")
		if proxyConfigsStr != "" {
			configs, err := parseProxyConfigsFromEnv(proxyConfigsStr)
			if err == nil && len(configs) > 0 {
				proxyConfigs = configs
				configSource = "环境变量: PROXY_CONFIGS"
			}
		}
	}

	// 如果仍然没有配置，使用默认配置
	if len(proxyConfigs) == 0 {
		log.Println("警告: 未找到配置文件且 PROXY_CONFIGS 环境变量未设置，使用默认配置")
		proxyConfigs = []ProxyConfig{
			{
				Name:        "百度一下",
				URL:         "https://www.baidu.com",
				Description: "百度一下，你就知道",
				Tags:        []string{"工具", "本地"},
				Status:      "active",
				Usage:       "1000000",
			},
			{
				Name:        "Google",
				URL:         "https://www.google.com",
				Description: "Google搜索",
				Tags:        []string{"工具", "本地"},
				Status:      "active",
				Usage:       "1000000",
			},
		}
		configSource = "默认配置"
	}

	log.Printf("配置来源: %s", configSource)
	log.Printf("加载了 %d 个代理配置", len(proxyConfigs))

	return Config{
		Port:         port,
		ProxyConfigs: proxyConfigs,
	}
}

// 从环境变量字符串解析代理配置
func parseProxyConfigsFromEnv(configStr string) ([]ProxyConfig, error) {
	var proxyConfigs []ProxyConfig
	configPairs := strings.Split(configStr, ",")

	for _, pair := range configPairs {
		parts := strings.SplitN(pair, ":", 3)
		if len(parts) >= 2 {
			config := ProxyConfig{
				Name: strings.TrimSpace(parts[0]),
				URL:  strings.TrimSpace(parts[1]),
			}
			if len(parts) >= 3 {
				config.Description = strings.TrimSpace(parts[2])
			}
			proxyConfigs = append(proxyConfigs, config)
		} else {
			log.Printf("警告: 忽略无效的代理配置项: %s", pair)
		}
	}

	if len(proxyConfigs) == 0 {
		return nil, fmt.Errorf("没有有效的代理配置")
	}

	return proxyConfigs, nil
}

// 从文件加载配置
func loadConfigFromFile(configPath string) []ProxyConfig {
	// 清理路径
	configPath = filepath.Clean(configPath)

	// 检查文件是否存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("调试: 配置文件不存在: %s", configPath)
		return nil
	}

	log.Printf("调试: 找到配置文件: %s", configPath)

	// 读取文件
	file, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("警告: 无法读取配置文件 %s: %v", configPath, err)
		return nil
	}

	var proxyConfigs []ProxyConfig
	err = json.Unmarshal(file, &proxyConfigs)
	if err != nil {
		log.Printf("警告: 解析配置文件 %s 失败: %v", configPath, err)
		return nil
	}

	// 验证配置
	if len(proxyConfigs) == 0 {
		log.Printf("警告: 配置文件 %s 中没有有效的代理配置", configPath)
		return nil
	}

	return proxyConfigs
}

// 创建主站点代理
func createMainProxy(targetURL string, proxyPath string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		// 添加 CORS 头
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "*")
		return nil
	}

	return proxy, nil
}

// 创建外部资源代理处理器
func externalResourceProxy(w http.ResponseWriter, r *http.Request) {
	// 解析路径：/proxy-external/oss.snappdown.com/cdn/font/...
	pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/proxy-external/"), "/", 2)
	if len(pathParts) < 2 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	targetHost, err := url.QueryUnescape(pathParts[0])
	if err != nil {
		http.Error(w, "Invalid host", http.StatusBadRequest)
		return
	}

	// 构建目标URL
	targetURL := fmt.Sprintf("https://%s/%s", targetHost, pathParts[1])
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// 创建代理请求
	target, err := url.Parse(targetURL)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusBadRequest)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	// 修改响应添加 CORS 头
	originalModifyResponse := proxy.ModifyResponse
	proxy.ModifyResponse = func(resp *http.Response) error {
		if originalModifyResponse != nil {
			if err := originalModifyResponse(resp); err != nil {
				return err
			}
		}

		// 添加 CORS 头
		resp.Header.Set("Access-Control-Allow-Origin", "*")
		resp.Header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		resp.Header.Set("Access-Control-Allow-Headers", "*")

		return nil
	}

	proxy.ServeHTTP(w, r)
}

// 渲染首页模板
func renderIndexPage(w http.ResponseWriter, config Config) {
	// 创建模板函数
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	// 尝试多种可能的模板路径
	templatePaths := []string{
		"templates/index.html",
		"./templates/index.html",
		"/app/templates/index.html",
		"index.html",
	}

	var tmpl *template.Template
	var err error

	for _, path := range templatePaths {
		tmpl, err = template.New(filepath.Base(path)).Funcs(funcMap).ParseFiles(path)
		if err == nil {
			log.Printf("模板成功加载从路径: %s", path)
			break
		} else {
			log.Printf("模板加载失败从路径 %s: %v", path, err)
		}
	}

	if err != nil {
		log.Printf("模板加载错误: %v", err)
		// 提供回退的基本页面
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
                <!DOCTYPE html>
                <html>
                <head><title>代理网站</title></head>
                <body>
                    <h1>欢迎使用代理网站</h1>
                    <p>模板加载错误，请检查配置</p>
                    <ul>
            `)
		for _, proxy := range config.ProxyConfigs {
			fmt.Fprintf(w, `<li><a href="/proxy/%s/">%s</a></li>`, strings.ToLower(proxy.Name), proxy.Name)
		}
		fmt.Fprintf(w, `
                    </ul>
                </body>
                </html>
            `)
		return
	}

	err = tmpl.ExecuteTemplate(w, "index.html", config)
	if err != nil {
		log.Printf("模板执行错误: %v", err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
}

func main() {
	config := loadConfig()

	// 设置静态文件服务
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 外部资源代理路由
	http.HandleFunc("/proxy-external/", externalResourceProxy)

	// 首页处理
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// 渲染首页
		renderIndexPage(w, config)
	})

	// 为每个代理配置创建处理器
	for _, proxyConfig := range config.ProxyConfigs {
		func(pc ProxyConfig) {
			proxyPath := "/proxy/" + strings.ToLower(pc.Name) + "/"
			proxyHandler, err := createMainProxy(pc.URL, proxyPath)
			if err != nil {
				log.Printf("创建代理 %s 失败: %v", pc.Name, err)
				return
			}

			// 使用 StripPrefix
			handler := http.StripPrefix(proxyPath, proxyHandler)
			http.Handle(proxyPath, handler)

			// 重定向没有斜杠的路径
			http.HandleFunc("/proxy/"+strings.ToLower(pc.Name), func(w http.ResponseWriter, r *http.Request) {
				// 构建目标URL并进行重定向
				targetURL := pc.URL + r.URL.Path[len("/proxy/"+strings.ToLower(pc.Name)):] // 获取代理路径后面的部分

				// 处理查询参数
				if r.URL.RawQuery != "" {
					targetURL += "?" + r.URL.RawQuery
				}

				// 使用 302 重定向到目标网站
				http.Redirect(w, r, targetURL, http.StatusFound)
			})

			log.Printf("注册代理: %s -> %s", pc.Name, pc.URL)
		}(proxyConfig)
	}

	// 启动服务器
	fmt.Printf("服务器启动在 http://localhost:%s\n", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}
