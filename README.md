# OnlyOffice fnOS Connector

在浏览器中直接编辑 NAS 上的 Office 文档。支持 DOCX、XLSX、PPTX 等格式的在线编辑，以及 DOC、XLS、PPT、ODT、ODS、ODP 等格式的转换和查看。

![OnlyOffice Connector](README.assets/image.png)

## 功能特性

- **在线编辑**: 直接在浏览器中编辑 DOCX、XLSX、PPTX 文档
- **格式转换**: 自动将旧格式 (DOC/XLS/PPT) 转换为 OOXML 格式
- **文档查看**: 支持 PDF、EPUB、FB2 等格式的在线预览
- **JWT 安全**: 支持 JWT 签名验证，确保文档传输安全
- **fnOS 集成**: 专为飞牛 NAS (fnOS) 设计的应用连接器

## 支持的文件格式

| 类型 | 可编辑 | 可转换 | 仅查看 |
|------|--------|--------|--------|
| 文档 | docx | doc, odt, rtf, txt | pdf, djvu, epub, fb2 |
| 表格 | xlsx | xls, ods, csv | - |
| 演示 | pptx | ppt, odp | - |

## 安装部署

### 1. 部署 OnlyOffice Document Server

在 fnOS 的 Docker 管理中创建 `docker-compose.yml`:

```yaml
services:
  onlyoffice-documentserver:
    image: onlyoffice/documentserver:latest
    container_name: onlyoffice-documentserver
    environment:
      - JWT_ENABLED=true
      - JWT_SECRET=your-secret-key-change-me  # 请修改为你自己的密钥
      - JWT_HEADER=Authorization
      - JWT_IN_BODY=true
    ports:
      - "10098:80"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/healthcheck"]
      interval: 30s
      retries: 5
      start_period: 60s
      timeout: 10s
    restart: always
    stop_grace_period: 60s
    volumes:
      - ./data:/var/www/onlyoffice/Data
      - ./log:/var/log/onlyoffice
```

启动服务:

```bash
docker compose up -d
```

### 2. 安装 OnlyOffice Connector

1. 前往 [Releases](https://github.com/tf4fun/onlyoffice-fnos/releases) 页面下载最新的 `.fpk` 安装包
2. 在 fnOS 应用中心选择「手动安装」，上传 `.fpk` 文件完成安装
3. 安装完成后，在应用列表中打开「OnlyOffice 连接器」进行配置

### 3. 配置连接器

打开连接器设置页面，填写以下信息:

- **Document Server URL**: `http://your-nas-ip:10098`
- **JWT Secret**: 与 docker-compose 中设置的 `JWT_SECRET` 保持一致
- **Base URL**: `http://your-nas-ip:10099` (连接器的回调地址)

## 使用方法

配置完成后，在 fnOS 文件管理器中右键点击 Office 文档，选择「使用 OnlyOffice 打开」即可在浏览器中编辑。

## 项目结构

```
.
├── cmd/server/          # 主程序入口
├── internal/
│   ├── config/          # 配置管理
│   ├── editor/          # 编辑器配置生成
│   ├── file/            # 文件服务
│   ├── format/          # 格式管理
│   ├── jwt/             # JWT 签名验证
│   └── server/          # HTTP 服务器
├── web/
│   ├── static/          # 静态资源
│   └── templates/       # HTML 模板
└── fnos.onlyoffice-connector/  # fnOS 应用包
```

## 开发

```bash
# 编译
go build -o onlyoffice-connector ./cmd/server

# 运行测试
go test ./...
```

## 许可证

MIT License

## 致谢

- [OnlyOffice Document Server](https://github.com/ONLYOFFICE/DocumentServer)
- [飞牛 NAS (fnOS)](https://www.fnnas.com/)
