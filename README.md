# 在线考试平台（gbexam）

一个前后端分离的全栈在线考试平台，支持题库管理、自动组卷、限时考试、自动阅卷、成绩分析和错题本，适用于自测练习与在线评测场景。

## 主要功能

- **用户与角色管理**：管理员 / 教师 / 学生三种角色，管理员账号由系统预设（默认 `admin` / `admin123`）。
- **题库管理**：单选、多选、判断、填空、简答五种题型，支持难度、知识点、分值与解析，支持 JSON / Excel 批量导入。
- **智能组卷**：教师按题型数量、分值、难度自动抽题组卷；不同学生的题目顺序与选项顺序随机打乱。
- **试卷版本冻结**：发布时将题干、选项、分值、标准答案整体冻结为不可变版本；题库题目后续被修改或撤回，已发布考试、进行中考试与历史成绩仍按组卷当时的内容展示与判分。重新组卷只生成新版本，旧版本保留可查；进行中的考试始终绑定开考时的版本，答题页、历史复核、批改与成绩详情读取同一版本，刷新后内容一致。
- **在线考试**：倒计时自动交卷、标记疑问、答题卡导航、刷新页面视为交卷。
- **自动阅卷与成绩分析**：客观题自动判分，填空/简答由教师在线批改，成绩报告含题型得分分布、正确率与排名。
- **考试记录与错题本**：查看历史记录与答题详情，错题自动收集并可按知识点复习、重复练习。
- **防作弊**：同一账号单设备登录、前端全屏锁定、禁用右键与文本选择。

## 快速启动（Docker Compose 一键部署）

首次启动前执行：

```bash
cp .env.example .env
```

然后启动：

```bash
docker compose --env-file .env up -d --build
```

等待服务健康后访问：

- 前端：http://127.0.0.1:18502
- 后端 API：http://127.0.0.1:19502
- 健康检查：http://127.0.0.1:19502/healthz

停止并清理：

```bash
docker compose --env-file .env down -v --remove-orphans
```

## 本地开发

### 后端

```bash
cd backend
go mod tidy
go run ./cmd/server
```

后端默认监听 `8080`，并通过环境变量读取配置（可参考根目录 `.env.example`）。

### 前端

```bash
cd frontend
npm install
npm run dev
```

前端开发服务器默认监听 `5173`，并将 `/api` 代理到 `http://localhost:19502`。

## 技术栈

| 分层 | 技术 |
| --- | --- |
| 后端 | Go 1.22 + Gin + GORM |
| 数据库 | MySQL 8.0 |
| 认证 | JWT（github.com/golang-jwt/jwt/v5），管理员/教师/学生 RBAC |
| 前端 | Vue 3 + TypeScript |
| UI 库 | Element Plus |
| 构建工具 | Vite |
| 状态管理 | Pinia |
| 图表 | ECharts |
| 其他 | Axios、Day.js |

## 项目目录结构

```
.
├── docker-compose.yml          # 一键部署编排
├── .env.example                # 环境变量示例
├── database/
│   └── init.sql                # MySQL 首次启动初始化脚本
├── backend/
│   ├── Dockerfile              # Go 多阶段构建
│   ├── api/openapi.yaml        # OpenAPI 文档
│   ├── migrations/             # 数据库迁移脚本
│   ├── deploy/                 # 部署说明
│   ├── cmd/server/main.go      # 装配依赖并启动服务
│   └── internal/
│       ├── config/             # 环境变量配置
│       ├── model/              # GORM 数据模型
│       ├── repository/         # 数据访问层
│       ├── service/            # 业务逻辑层
│       ├── handler/            # HTTP 处理层
│       ├── router/             # 路由
│       ├── middleware/         # JWT/RBAC/日志等横切逻辑
│       ├── dto/                # 请求/响应结构体
│       └── constants/          # 错误码与常量
└── frontend/
    ├── Dockerfile              # Vue 多阶段构建 + Nginx
    ├── nginx.conf              # /api 反向代理
    └── src/
        ├── api/                # Axios 请求封装
        ├── components/         # 布局组件
        ├── router/             # 路由与权限守卫
        ├── stores/             # Pinia 状态
        ├── types/              # TypeScript 类型
        └── views/              # 页面
```

## 环境变量说明

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | `gbexam` | Docker Compose 项目名 |
| `DB_NAME` | `gbexam` | 数据库名 |
| `DB_USER` | `gbexam` | 数据库用户 |
| `DB_PASSWORD` | `gbexam123` | 数据库密码 |
| `DB_ROOT_PASSWORD` | `root123` | MySQL root 密码 |
| `JWT_SECRET` | `change-me` | JWT 签名密钥 |
| `JWT_EXPIRE_HOURS` | `24` | JWT 有效期（小时） |
| `ADMIN_USERNAME` | `admin` | 预设管理员用户名 |
| `ADMIN_PASSWORD` | `admin123` | 预设管理员密码 |
| `FRONTEND_PORT` | `18502` | 前端宿主机端口 |
| `BACKEND_PORT` | `19502` | 后端宿主机端口 |
| `DB_PORT` | `57502` | MySQL 宿主机端口 |

## Docker 部署说明

- 编排文件不声明 `version:` 字段，顶层使用 `name: gbexam`。
- 三个服务均带 `container_name`，使用 `${COMPOSE_PROJECT_NAME:-gbexam}` 前缀。
- MySQL 数据通过命名卷 `db_data` 持久化，并配置了 `healthcheck`。
- 后端 `depends_on: db: condition: service_healthy`，前端 `depends_on: backend: condition: service_healthy`。
- 后端内部端口固定为 `8080`，前端 Nginx 将 `/api/` 反向代理到 `http://backend:8080/`。
- 支持任意目录名（含中文目录）下启动。

## 试卷版本冻结机制

- **冻结时机**：草稿考试的题目是可变工作副本；点击发布后，系统在一个数据库事务内锁定考试与题目行，把当时的题干、选项、每题分值、标准答案、解析复制到 `paper_versions` / `paper_version_questions`，考试指针指向该冻结版本。
- **题目变更/撤回无影响**：冻结内容是独立副本，题库题目的后续修改或删除都不会改变已发布试卷；进行中考试刷新页面、交卷自动判分、教师批改、历史试卷复核、成绩详情与报告全部读取答题记录绑定的冻结版本（`exam_attempts.paper_version_id`），因此刷新前后、跨入口看到的内容完全一致。
- **重新组卷只产生新版本**：对已发布/已关闭考试执行重新组卷会生成 `version_no` 递增的新版本并切换“当前版本”，旧版本永久保留，可通过版本列表和 `GET /exams/:id/questions?version_no=N` 查看。已在进行中的考试继续使用其开考时的旧版本，之后新开考的学生使用新版本。
- **版本一致性与并发控制**：发布在事务内对考试行加排他锁并以 `status = 'draft'` 为条件更新，重复/并发发布只有一次真正生效，其余调用幂等返回同一版本；重新组卷携带 `expected_revision` 乐观锁令牌，与他人并发的发布或组卷冲突时返回 `409`，保证同时变更只生效一次。
- **存量数据**：服务启动时会自动为版本功能上线前已存在的考试和答题记录回填一个 v1 快照，历史成绩同样可按冻结内容复核。

## API 清单

统一前缀 `/api/v1`，健康检查 `/healthz` 与 `/health`，响应统一为 `{code, message, data}`。

### 认证

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| POST | `/api/v1/auth/register` | 注册学生/教师 | 公开 |
| POST | `/api/v1/auth/login` | 登录 | 公开 |
| GET | `/api/v1/auth/profile` | 当前用户信息 | 登录 |
| POST | `/api/v1/auth/logout` | 退出登录 | 登录 |

### 用户管理

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | `/api/v1/users` | 用户分页列表 | 管理员 |
| PUT | `/api/v1/users/:id/status` | 启用/禁用用户 | 管理员 |

### 题库管理

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | `/api/v1/questions` | 题目分页列表 | 管理员/教师 |
| POST | `/api/v1/questions` | 新建题目 | 管理员/教师 |
| GET | `/api/v1/questions/:id` | 题目详情 | 管理员/教师 |
| PUT | `/api/v1/questions/:id` | 更新题目 | 管理员/教师 |
| DELETE | `/api/v1/questions/:id` | 删除题目 | 管理员/教师 |
| POST | `/api/v1/questions/batch-import` | 批量导入（JSON/Excel） | 管理员/教师 |

### 考试管理

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | `/api/v1/exams` | 考试列表 | 登录 |
| POST | `/api/v1/exams` | 创建考试并自动组卷 | 管理员/教师 |
| GET | `/api/v1/exams/:id` | 考试详情 | 登录 |
| POST | `/api/v1/exams/:id/publish` | 发布考试并冻结试卷为 v1（重复发布幂等） | 管理员/教师 |
| POST | `/api/v1/exams/:id/regroup` | 重新组卷并生成新冻结版本（旧版本保留） | 管理员/教师 |
| GET | `/api/v1/exams/:id/versions` | 查看全部冻结试卷版本 | 管理员/教师 |
| POST | `/api/v1/exams/:id/close` | 关闭考试 | 管理员/教师 |
| DELETE | `/api/v1/exams/:id` | 删除考试（含全部冻结版本） | 管理员/教师 |
| GET | `/api/v1/exams/:id/questions?version_no=N` | 查看试卷题目（`version_no` 指定历史冻结版本） | 管理员/教师 |
| GET | `/api/v1/exams/:id/stats` | 考试成绩统计 | 管理员/教师 |
| GET | `/api/v1/exams/:id/attempts` | 查看答题记录（批改列表） | 管理员/教师 |

### 考试作答

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| POST | `/api/v1/exams/:id/attempts` | 开始考试（生成随机试卷） | 学生 |
| GET | `/api/v1/exams/:id/attempts/current` | 获取进行中的试卷 | 学生 |
| POST | `/api/v1/attempts/:id/answers` | 保存答案 | 学生 |
| POST | `/api/v1/attempts/:id/submit` | 交卷并自动阅客观题 | 学生 |
| GET | `/api/v1/attempts` | 我的考试记录 | 学生 |
| GET | `/api/v1/attempts/:id` | 答题详情 | 学生/教师/管理员 |
| GET | `/api/v1/attempts/:id/report` | 成绩报告 | 学生/教师/管理员 |
| PUT | `/api/v1/attempts/:id/grade` | 批改主观题 | 管理员/教师 |

### 统计与错题本

| 方法 | 路径 | 说明 | 权限 |
| --- | --- | --- | --- |
| GET | `/api/v1/stats/overview` | 首页概览统计 | 登录 |
| GET | `/api/v1/wrong-questions` | 错题本列表 | 学生 |
| DELETE | `/api/v1/wrong-questions/:id` | 移除错题 | 学生 |
| GET | `/api/v1/wrong-questions/practice` | 获取错题练习 | 学生 |
| POST | `/api/v1/wrong-questions/practice` | 提交错题练习 | 学生 |

## License

MIT
