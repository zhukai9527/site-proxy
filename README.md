# 代理网站项目

这是一个使用Go语言开发的代理网站项目，可以代理访问第三方网站。项目支持Docker部署，并可通过环境变量配置多个代理地址。

## 功能特点

- 支持多个代理地址配置
- 简洁美观的用户界面
- 毛玻璃效果和动画设计
- Docker容器化部署
- 响应式设计，适配不同设备

## 快速开始

### 本地运行

1. 确保已安装Go环境（推荐Go 1.19或更高版本）
2. 克隆项目到本地
3. 进入项目目录
4. 运行项目

```bash
go run main.go
```

默认情况下，服务将在 http://localhost:8080 上运行。

### 使用Docker运行

1. 确保已安装Docker和Docker Compose
2. 进入项目目录
3. 构建并启动容器

```bash
docker-compose up -d
```

## 配置方式

### 配置文件配置（推荐用于本地开发）

项目支持通过`config.yaml`文件进行配置，这是本地开发时的推荐方式：

```yaml
# 服务器监听端口
port: 8080

# 代理配置列表
proxy_configs:
  - name: Google
    url: https://www.google.com
  - name: GitHub
    url: https://github.com
  - name: Baidu
    url: https://www.baidu.com
```

### 环境变量配置（推荐用于生产环境）

项目也支持以下环境变量配置（当配置文件不存在时使用）：

- `PORT`: 服务器监听端口，默认为8080
- `PROXY_CONFIGS`: 代理配置，格式为`name1:url1,name2:url2,...`

示例：

```
PORT=8080
PROXY_CONFIGS=Google:https://www.google.com,GitHub:https://github.com,Baidu:https://www.baidu.com
```

## 项目结构

```
/
├── main.go           # 主程序入口
├── templates/        # HTML模板
│   └── index.html    # 首页模板
├── static/           # 静态资源
│   ├── css/          # CSS样式
│   │   └── style.css
│   └── js/           # JavaScript脚本
│       └── script.js
├── Dockerfile        # Docker构建文件
├── docker-compose.yml # Docker Compose配置
└── README.md         # 项目说明
```

## 许可证

MIT