package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// ProxyConfig 代表一个代理配置
type ProxyConfig struct {
	Name string
	URL  string
}

// Config 应用配置
type Config struct {
	Port         string
	ProxyConfigs []ProxyConfig
}

// 从环境变量加载配置
func loadConfig() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // 默认端口
	}

	// 从环境变量获取代理配置
	// 格式: PROXY_CONFIGS=name1:url1,name2:url2,...
	proxyConfigsStr := os.Getenv("PROXY_CONFIGS")
	if proxyConfigsStr == "" {
		// 默认配置，实际应用中应该从环境变量获取
		proxyConfigsStr = "Google:https://www.google.com,GitHub:https://github.com,Baidu:https://www.baidu.com"
	}

	var proxyConfigs []ProxyConfig
	configPairs := strings.Split(proxyConfigsStr, ",")
	for _, pair := range configPairs {
		parts := strings.Split(pair, ":")
		if len(parts) == 2 {
			proxyConfigs = append(proxyConfigs, ProxyConfig{
				Name: parts[0],
				URL:  parts[1],
			})
		}
	}

	return Config{
		Port:         port,
		ProxyConfigs: proxyConfigs,
	}
}

// 创建反向代理处理器
func createReverseProxy(targetURL string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(url)
	
	// 修改请求头
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = url.Host
		req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	}

	return proxy, nil
}

func main() {
	config := loadConfig()

	// 设置静态文件服务
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 首页处理
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		// 创建模板函数
		funcMap := template.FuncMap{
			"ToLower": strings.ToLower,
		}

		// 解析模板并添加函数
		tmpl, err := template.New("index.html").Funcs(funcMap).ParseFiles("templates/index.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmpl.Execute(w, config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// 为每个代理配置创建处理器
	for _, proxyConfig := range config.ProxyConfigs {
		// 使用闭包捕获当前的proxyConfig
		func(config ProxyConfig) {
			proxyHandler, err := createReverseProxy(config.URL)
			if err != nil {
				log.Fatalf("创建代理失败: %v", err)
			}

			// 为每个代理创建路由
			proxyPath := "/proxy/" + strings.ToLower(config.Name) + "/"
			http.HandleFunc(proxyPath, func(w http.ResponseWriter, r *http.Request) {
				// 修改请求路径，移除代理前缀
				r.URL.Path = strings.TrimPrefix(r.URL.Path, proxyPath)
				if r.URL.Path == "" {
					r.URL.Path = "/"
				}
				proxyHandler.ServeHTTP(w, r)
			})
		}(proxyConfig)
	}

	// 启动服务器
	fmt.Printf("服务器启动在 http://localhost:%s\n", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}