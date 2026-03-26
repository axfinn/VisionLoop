# 迭代 4 设计

## 新需求: 所有变更代码都要及时同步 github ！！

---

## 影响分析（哪些已有文件需要修改）

当前项目状态：
- 分支: `iter-1`
- 已修改未提交文件 (15个): README.md, RESULT.md, build.sh, main.go, config.yaml, server.go, encoder.go, mp4.go, webrtc.go, web/index.html, web/src/App.vue, web/src/main.js, .autodev/logs/*
- 未跟踪文件: CONFIG.md, DEPLOY.md, Dockerfile, Dockerfile.detection, deploy.sh, docker-compose.yml, process/iter-1/result.md, process/iter-2/, process/iter-3/

需要修改的文件：
1. `.gitignore` - 添加忽略规则（.autodev/logs/, __pycache__/, *.pyc）
2. `deploy.sh` - 添加 git sync 相关命令
3. 新增 `sync.sh` - Git 同步脚本

---

## 实现方案（最小改动原则）

### 1. 完善 .gitignore
- 忽略 .autodev/ 日志目录
- 忽略 Python __pycache__ 和 *.pyc
- 忽略构建产物 (*.exe, *.dll, dist/, build/)

### 2. 创建 sync.sh 脚本
Git 同步脚本，支持：
- `sync.sh status` - 查看未同步状态
- `sync.sh commit "message"` - 提交所有变更
- `sync.sh push` - 推送到远程
- `sync.sh sync "message"` - 一键提交并推送
- 自动检测未提交变更并列出

### 3. 更新 deploy.sh
在 deploy.sh 中集成 git sync 命令：
- 添加 `git-commit` 命令
- 添加 `git-push` 命令
- 任何 `install` 操作后自动提示 sync

---

## 执行步骤清单

1. [ ] 创建/更新 `.gitignore`
2. [ ] 创建 `sync.sh` 脚本
3. [ ] 更新 `deploy.sh` 添加 git 命令
4. [ ] 提交所有 pending changes
5. [ ] 推送到 GitHub
6. [ ] 验证同步成功
7. [ ] 更新 RESULT.md
