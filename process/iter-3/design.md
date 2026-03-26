# 迭代 3 设计

## 新需求: 没有部署脚本 没有 docker 部署支持 没有文档 没有配置介绍！！

---

## 影响分析

### 需要新增的文件
| 文件 | 说明 |
|------|------|
| `deploy.sh` | 部署脚本，一键部署VisionLoop |
| `Dockerfile` | Docker镜像构建文件 |
| `docker-compose.yml` | Docker Compose编排 |
| `Dockerfile.detection` | Python检测进程独立镜像 |
| `CONFIG.md` | 配置详细介绍文档 |
| `DEPLOY.md` | 部署指南文档 |

### 需要修改的文件
| 文件 | 修改内容 |
|------|----------|
| `README.md` | 补充完整使用说明、API文档、FAQ |
| `config.yaml` | 添加注释说明每个配置项 |
| `build.sh` | 改进构建脚本，添加前端构建 |

---

## 实现方案

### 1. 部署脚本 (deploy.sh)

```bash
#!/bin/bash
# deploy.sh - VisionLoop 一键部署脚本

# 支持:
#   - Linux native 部署
#   - Docker 容器部署
#   - Docker Compose 部署
```

功能:
- 检测系统环境 (Linux/Docker)
- 自动下载/构建依赖
- 配置生成
- 服务启动/停止

### 2. Docker 支持

**Dockerfile** - Go服务
- 多阶段构建: Go build + runtime
- 嵌入式FFmpeg
- 健康检查

**Dockerfile.detection** - Python检测
- Python 3.11 + PyTorch
- YOLOv8 + ONNX runtime
- 独立容器运行

**docker-compose.yml**
- visionloop 主服务
- visionloop-detect 检测服务
- 卷挂载: clips, events, screenshots
- 网络: host模式(访问摄像头)

### 3. 配置文件详解 (CONFIG.md)

详细说明每个配置项:
- server.* - HTTP服务配置
- storage.* - 存储配置
- capture.* - 摄像头采集配置
- encoding.* - 编码器配置
- detection.* - 检测引擎配置

### 4. README.md 增强

- 完整的快速开始
- 环境要求详细说明
- Docker部署详细步骤
- 故障排除FAQ
- API完整文档

---

## 执行步骤清单

1. [ ] 创建 `deploy.sh` 部署脚本
2. [ ] 创建 `Dockerfile` 主服务镜像
3. [ ] 创建 `Dockerfile.detection` 检测进程镜像
4. [ ] 创建 `docker-compose.yml` 编排文件
5. [ ] 创建 `CONFIG.md` 配置详解
6. [ ] 创建 `DEPLOY.md` 部署指南
7. [ ] 增强 `README.md`
8. [ ] 更新 `config.yaml` 添加注释
9. [ ] 更新 `build.sh` 完善构建流程
10. [ ] 验证所有脚本可执行

---

## 最小改动原则

- 不修改已有功能代码
- 不重构项目结构
- 仅新增部署/文档文件
- 保持向后兼容