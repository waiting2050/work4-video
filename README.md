# Video - TikTok 风格短视频平台

一个基于 Hertz 框架构建的类 TikTok 短视频平台后端，具有双 Token 认证、Redis 缓存和完善的社交互动功能。

## 目录

- [项目概述](#项目概述)
- [核心功能](#核心功能)
- [技术栈](#技术栈)
- [环境配置](#环境配置)
- [安装步骤](#安装步骤)
- [使用方法](#使用方法)
- [API 接口文档](#api-接口文档)
- [项目结构](#项目结构)
- [配置说明](#配置说明)
- [Docker 部署](#docker-部署)
- [贡献指南](#贡献指南)
- [许可证](#许可证)

## 项目概述

Video 是一个受 TikTok 启发的短视频社交平台后端，旨在提供完整的视频分享和社交互动体验。本项目实现了一个完整的后端系统，包含用户认证、视频管理、社交互动和内容发现等功能。

### 核心亮点

- **双 Token 认证**：基于 JWT 的安全认证机制，包含访问令牌和刷新令牌
- **Redis 缓存**：使用 Redis 缓存优化热门视频排行榜
- **RESTful API**：17 个完善的 API 接口，遵循 OpenAPI 3.0.1 规范
- **Docker 就绪**：完整的容器化支持，便于部署
- **清晰架构**：模块化设计，职责分离明确

## 核心功能

### 用户模块
- 用户注册（bcrypt 密码加密）
- 登录（双 Token 认证：Access-Token + Refresh-Token）
- 用户信息管理
- 头像上传功能

### 视频模块
- 视频上传与发布
- 用户视频列表（支持分页）
- 视频搜索（支持关键词、用户名、日期范围过滤）
- 热门视频排行榜（Redis 缓存）

### 互动模块
- 视频点赞/取消点赞
- 查看点赞视频列表
- 发布视频评论
- 查看评论列表
- 删除自己的评论

### 社交模块
- 关注/取消关注用户
- 查看关注列表
- 查看粉丝列表
- 查看互关好友列表

## 技术栈

| 类别 | 技术 |
|------|------|
| **框架** | [Hertz](https://github.com/cloudwego/hertz) - CloudWeGo HTTP 框架 |
| **数据库** | MySQL + GORM ORM |
| **缓存** | Redis（热门排行榜缓存） |
| **认证** | JWT (golang-jwt/jwt) |
| **密码加密** | bcrypt |
| **ID 生成** | UUID (google/uuid) |
| **容器化** | Docker & Docker Compose |

## 环境配置

### 前置要求

- Go 1.26+
- MySQL 8.0+
- Redis 7.0+
- Docker（可选，用于容器化部署）

### 环境变量

创建 `.env` 文件或设置以下环境变量：

```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=video_website

# Redis 配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# JWT 配置
JWT_ACCESS_SECRET=your-access-secret-key
JWT_REFRESH_SECRET=your-refresh-secret-key

# 服务器配置
SERVER_PORT=8888
```

## 安装步骤

### 本地开发

1. **克隆仓库**
   ```bash
   git clone https://github.com/waiting2050/work4-video.git
   cd work4-video
   ```

2. **安装依赖**
   ```bash
   go mod download
   ```

3. **创建数据库**
   ```sql
   CREATE DATABASE video_website CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
   ```

4. **运行应用**
   ```bash
   go run .
   ```

服务器将在 `http://localhost:8888` 启动

### Docker 部署

1. **使用 Docker Compose 构建并运行**
   ```bash
   docker-compose up -d
   ```

2. **查看服务状态**
   ```bash
   docker-compose ps
   ```

3. **查看日志**
   ```bash
   docker-compose logs -f app
   ```

## 使用方法

### 健康检查

```bash
curl http://localhost:8888/ping
```

### 用户注册

```bash
curl -X POST http://localhost:8888/user/register \
  -F "username=testuser" \
  -F "password=password123"
```

### 用户登录

```bash
curl -X POST http://localhost:8888/user/login \
  -F "username=testuser" \
  -F "password=password123"
```

响应头将包含：
- `Access-Token`：短期令牌（2 小时）
- `Refresh-Token`：长期令牌（7 天）

### 上传视频（需要认证）

```bash
curl -X POST http://localhost:8888/video/publish \
  -H "Access-Token: your-access-token" \
  -F "data=@video.mp4" \
  -F "title=我的视频" \
  -F "description=视频描述"
```

## API 接口文档

### 基础 URL
```
http://localhost:8888
```

### 响应格式

所有 API 响应遵循以下格式：

```json
{
  "base": {
    "code": 10000,
    "msg": "success"
  },
  "data": {
    // 响应数据
  }
}
```

### 状态码

| 状态码 | 含义 |
|--------|------|
| 10000 | 成功 |
| -1 | 失败 |

### API 端点

#### 用户模块

| 方法 | 端点 | 描述 | 需要认证 |
|------|------|------|----------|
| POST | `/user/register` | 用户注册 | 否 |
| POST | `/user/login` | 用户登录 | 否 |
| GET | `/user/info?user_id={id}` | 获取用户信息 | 否 |
| PUT | `/user/avatar/upload` | 上传头像 | 是 |

#### 视频模块

| 方法 | 端点 | 描述 | 需要认证 |
|------|------|------|----------|
| POST | `/video/publish` | 上传视频 | 是 |
| GET | `/video/list?user_id={id}&page_num={n}&page_size={n}` | 获取用户视频列表 | 否 |
| POST | `/video/search` | 搜索视频 | 否 |
| GET | `/video/popular?page_num={n}&page_size={n}` | 获取热门视频 | 否 |

#### 互动模块

| 方法 | 端点 | 描述 | 需要认证 |
|------|------|------|----------|
| POST | `/like/action` | 点赞/取消点赞 | 是 |
| GET | `/like/list?user_id={id}` | 获取点赞视频列表 | 否 |
| POST | `/comment/publish` | 发布评论 | 是 |
| GET | `/comment/list?video_id={id}` | 获取评论列表 | 否 |
| DELETE | `/comment/delete` | 删除评论 | 是 |

#### 社交模块

| 方法 | 端点 | 描述 | 需要认证 |
|------|------|------|----------|
| POST | `/relation/action` | 关注/取消关注用户 | 是 |
| GET | `/following/list?user_id={id}` | 获取关注列表 | 否 |
| GET | `/follower/list?user_id={id}` | 获取粉丝列表 | 否 |
| GET | `/friends/list` | 获取好友列表 | 是 |

### 请求参数

#### 点赞操作
```
video_id: string（必填）
action_type: int（1=点赞，2=取消点赞）
```

#### 关注操作
```
to_user_id: string（必填）
action_type: int（0=关注，1=取消关注）
```

#### 视频搜索
```
keywords: string（可选）
username: string（可选）
from_date: int64（可选，13 位时间戳）
to_date: int64（可选，13 位时间戳）
page_num: int
page_size: int
```

## 项目结构

```
video/
├── biz/
│   ├── auth/                 # JWT 认证
│   │   └── jwt.go
│   ├── cache/                # Redis 缓存
│   │   └── redis.go
│   ├── handler/              # HTTP 处理器
│   │   ├── user.go           # 用户接口
│   │   ├── video.go          # 视频接口
│   │   ├── interaction.go    # 互动接口
│   │   └── social.go         # 社交接口
│   ├── model/                # 数据模型
│   │   ├── models.go         # GORM 模型
│   │   ├── db.go             # 数据库连接
│   │   └── config.go         # 配置
│   ├── service/              # 业务逻辑
│   │   ├── user.go
│   │   ├── video.go
│   │   ├── interaction.go
│   │   └── social.go
│   └── utils/                # 工具函数
│       └── response.go       # 响应辅助函数
├── uploads/                  # 文件上传目录
│   ├── avatars/              # 用户头像
│   └── videos/               # 视频文件
├── Dockerfile
├── docker-compose.yml
├── .dockerignore
├── go.mod
├── go.sum
├── main.go                   # 程序入口
├── router.go                 # 路由注册
└── README.md
```

## 配置说明

### 数据库配置

应用使用 GORM 连接 MySQL。数据库表在启动时自动迁移：

- `users` - 用户账户
- `videos` - 视频内容
- `comments` - 视频评论
- `follows` - 用户关系
- `likes` - 视频点赞

### Redis 配置

Redis 用于缓存热门视频排行榜，TTL 为 5 分钟。

### JWT 配置

- **Access-Token**：有效期 2 小时
- **Refresh-Token**：有效期 7 天
- **算法**：HMAC-SHA256

## Docker 部署

### Dockerfile 特性

- 多阶段构建，镜像体积更小
- 使用非 root 用户运行，提高安全性
- 支持健康检查
- 时区配置（Asia/Shanghai）

### Docker Compose 服务

| 服务 | 端口 | 描述 |
|------|------|------|
| app | 8888 | 主应用 |
| mysql | 3306 | 数据库 |
| redis | 6379 | 缓存 |

### 部署步骤

1. **构建镜像**
   ```bash
   docker-compose build
   ```

2. **启动服务**
   ```bash
   docker-compose up -d
   ```

3. **检查健康状态**
   ```bash
   curl http://localhost:8888/ping
   ```

## 贡献指南

### 开发规范

1. 遵循 Go 标准格式化（`gofmt`）
2. 提交 PR 前确保所有测试通过
3. API 变更时更新相关文档
4. 遵循现有代码结构

### 代码风格

- 使用有意义的变量名
- 为复杂逻辑添加注释
- 正确处理错误
- 遵循 RESTful API 规范

## 许可证

本项目采用 MIT 许可证。

## 联系方式

如有问题或需要支持，请在仓库中提交 Issue。

---

**说明**：本项目是使用现代 Go 技术构建类 TikTok 视频平台的学习项目。
