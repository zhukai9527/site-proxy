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
    "path/filepath"
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
    proxyConfigsStr := os.Getenv("PROXY_CONFIGS")
    if proxyConfigsStr == "" {
        // 提供更明显的默认配置
        log.Println("警告: PROXY_CONFIGS 环境变量未设置，使用默认配置")
        proxyConfigsStr = "Google:https://www.google.com,GitHub:https://github.com,Baidu:https://www.baidu.com"
    }

    var proxyConfigs []ProxyConfig
    configPairs := strings.Split(proxyConfigsStr, ",")
    for _, pair := range configPairs {
        parts := strings.SplitN(pair, ":", 2) // 使用SplitN防止URL中有冒号
        if len(parts) == 2 {
            proxyConfigs = append(proxyConfigs, ProxyConfig{
                Name: strings.TrimSpace(parts[0]),
                URL:  strings.TrimSpace(parts[1]),
            })
        } else {
            log.Printf("警告: 忽略无效的代理配置项: %s\n", pair)
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
                break
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
    })

    // 为每个代理配置创建处理器
    for _, proxyConfig := range config.ProxyConfigs {
        // 使用闭包捕获当前的proxyConfig
        func(pc ProxyConfig) {
            proxyHandler, err := createReverseProxy(pc.URL)
            if err != nil {
                log.Printf("创建代理 %s 失败: %v", pc.Name, err)
                return
            }

            // 为每个代理创建路由
            proxyPath := "/proxy/" + strings.ToLower(pc.Name) + "/"
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
    log.Printf("加载的代理配置: %+v\n", config.ProxyConfigs)
    log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}