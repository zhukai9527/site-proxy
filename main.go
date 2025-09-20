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
    "regexp"
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
func createReverseProxy(targetURL string, proxyPath string) (*httputil.ReverseProxy, error) {
    target, err := url.Parse(targetURL)
    if err != nil {
        return nil, err
    }

    proxy := httputil.NewSingleHostReverseProxy(target)
    
    // 修改请求头
    originalDirector := proxy.Director
    proxy.Director = func(req *http.Request) {
        originalDirector(req)
        req.Host = target.Host
        req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
        req.Header.Set("X-Forwarded-Proto", "http")
        
        // 记录原始URL以便在重写重定向时使用
        req.Header.Set("X-Original-Path", proxyPath)
    }

    // 修改响应，处理重定向
    proxy.ModifyResponse = func(resp *http.Response) error {
        // 检查是否是重定向响应
        if resp.StatusCode >= 300 && resp.StatusCode < 400 {
            location, err := resp.Location()
            if err == nil && location != nil {
                // 如果重定向指向目标域名，则重写为重定向到代理路径
                if location.Host == target.Host {
                    // 保存原始路径
                    originalPath := location.Path
                    if location.RawQuery != "" {
                        originalPath += "?" + location.RawQuery
                    }
                    
                    // 构建新的代理URL
                    newLocation := &url.URL{
                        Path:     proxyPath + strings.TrimPrefix(originalPath, "/"),
                        RawQuery: location.RawQuery,
                    }
                    
                    // 设置新的Location头
                    resp.Header.Set("Location", newLocation.String())
                    log.Printf("重写重定向: %s -> %s", location.String(), newLocation.String())
                }
            }
        }
        
        // 修复可能的内容安全策略头，防止浏览器阻塞资源加载
        if csp := resp.Header.Get("Content-Security-Policy"); csp != "" {
            // 放宽CSP策略以允许通过代理加载资源
            resp.Header.Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval' *;")
        }
        
        return nil
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
        var loadedPath string
        
        for _, path := range templatePaths {
            tmpl, err = template.New(filepath.Base(path)).Funcs(funcMap).ParseFiles(path)
            if err == nil {
                loadedPath = path
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
    })

    // 为每个代理配置创建处理器
    for _, proxyConfig := range config.ProxyConfigs {
        // 使用闭包捕获当前的proxyConfig
        func(pc ProxyConfig) {
            proxyPath := "/proxy/" + strings.ToLower(pc.Name) + "/"
            proxyHandler, err := createReverseProxy(pc.URL, proxyPath)
            if err != nil {
                log.Printf("创建代理 %s 失败: %v", pc.Name, err)
                return
            }

            // 为每个代理创建路由
            http.HandleFunc(proxyPath, func(w http.ResponseWriter, r *http.Request) {
                // 修改请求路径，移除代理前缀
                originalPath := r.URL.Path
                r.URL.Path = strings.TrimPrefix(r.URL.Path, proxyPath)
                if r.URL.Path == "" {
                    r.URL.Path = "/"
                }
                
                log.Printf("代理请求: %s -> %s%s", originalPath, pc.URL, r.URL.Path)
                proxyHandler.ServeHTTP(w, r)
            })
            
            // 处理没有尾部斜杠的情况
            http.HandleFunc("/proxy/"+strings.ToLower(pc.Name), func(w http.ResponseWriter, r *http.Request) {
                http.Redirect(w, r, proxyPath, http.StatusMovedPermanently)
            })
        }(proxyConfig)
    }

    // 启动服务器
    fmt.Printf("服务器启动在 http://localhost:%s\n", config.Port)
    log.Printf("加载的代理配置: %+v\n", config.ProxyConfigs)
    log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}