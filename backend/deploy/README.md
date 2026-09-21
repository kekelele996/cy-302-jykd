# 部署说明

本项目首选使用根目录的 `docker-compose.yml` 一键部署：

```bash
cp .env.example .env
docker compose --env-file .env up -d --build
```

部署文件说明：

- 根目录 `docker-compose.yml`：三服务编排（backend / frontend / db）。
- `backend/Dockerfile`：Go 多阶段构建。
- `frontend/Dockerfile`：Vue 多阶段构建 + Nginx。
- `database/init.sql`：MySQL 首次启动时执行的初始化脚本。
- 本目录仅保留部署文档，实际 Docker/K8s 资源文件与项目根目录保持统一。

健康检查接口：

- 后端：`GET /healthz` 与 `GET /health`
- 前端：Nginx 静态资源根路径 `/`
