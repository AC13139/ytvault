# YtVault 项目使用手册（第一版草稿）

## 1. 项目简介

YtVault 是一个面向内部自建场景的 YouTube 内容同步与归档系统，目标不是做一次性的“下载器”，而是提供一个**可持续运行、状态清晰、便于排障与恢复**的控制面。

当前第一版（Phase 1 skeleton）以 **API First** 为核心设计，提供三类使用入口：

- **HTTP API**：作为统一控制面
- **Web 页面**：用于基础人工操作与状态查看
- **CLI**：用于脚本化调用与运维检查

现阶段项目重点是先把“源管理 + 基础任务模型 + 最小可操作界面”跑通，为后续下载执行、调度器、凭据管理、诊断体系打基础。

---

## 2. 当前已支持内容

基于当前仓库第一版实现，已具备以下能力：

### 2.1 已支持的 Source 类型
支持新增以下三类 YouTube Source：

- **video**：单个视频
- **playlist**：播放列表
- **channel**：频道

### 2.2 已支持的核心能力
- 提供 API 服务启动能力
- 提供基础健康检查接口：`/healthz`
- 提供 Source 列表查询接口
- 提供 Source 新增接口
- 提供 Task 列表查询接口
- 新增 Source 时，自动生成一条初始 `index` 类型任务
- Source 与 Task 状态会持久化到本地 JSON 文件
- 提供一个内嵌式简易 Web 页面作为第一版控制台
- 提供基础 CLI，可调用 health/source/task 三类操作
- 已实现基础的 YouTube URL 校验逻辑

### 2.3 当前状态说明
当前版本属于**第一版 skeleton / 控制面雏形**，主要完成的是：

- 交互入口打通
- 数据模型初步建立
- 状态持久化机制可用
- 基础操作链路可验证

**尚未进入完整下载执行阶段**。也就是说，系统目前可以“登记 Source、生成 Task、展示状态”，但还不能真正完成 YouTube 内容抓取、下载、转码和元数据处理。

---

## 3. 环境要求

### 3.1 运行环境
建议环境如下：

- **操作系统**：Linux / macOS / WSL2
- **Go 版本**：`go 1.26` 或兼容版本
- **网络要求**：可访问 YouTube
- **本地磁盘权限**：可写入项目数据目录

### 3.2 当前第一版依赖特征
当前 skeleton 为纯 Go 最小实现，运行 API/CLI 不依赖 PostgreSQL、Redis、yt-dlp 或 ffmpeg。

当前状态持久化采用本地 JSON 文件，例如：

- 默认数据文件：`./data/state.json`

因此第一版更适合：

- 本地功能验证
- 交互流程演示
- 后续架构扩展前的骨架确认

---

## 4. 如何启动 API

### 4.1 进入项目目录
```bash
cd /home/xiaob/.openclaw/workspace/projects/YtVault
```

### 4.2 直接启动
使用 Makefile：

```bash
make run-api
```

该命令实际执行：

```bash
go run ./apps/api
```

### 4.3 默认监听地址
默认监听：

```text
:8080
```

即本机访问地址通常为：

- `http://127.0.0.1:8080`

### 4.4 可选环境变量
可通过环境变量覆盖默认配置：

#### API 监听地址
```bash
export YTVAULT_ADDR=:8081
```

#### 数据文件路径
```bash
export YTVAULT_DATA_PATH=./data/test-state.json
```

示例：

```bash
YTVAULT_ADDR=:8080 YTVAULT_DATA_PATH=./data/state.json go run ./apps/api
```

### 4.5 健康检查
启动后可执行：

```bash
curl http://127.0.0.1:8080/healthz
```

预期返回 JSON，包含：

- `status`
- `service`
- `timestamp`

---

## 5. 如何使用 Web 页面

### 5.1 打开页面
启动 API 后，在浏览器访问：

```text
http://127.0.0.1:8080/
```

### 5.2 当前 Web 页面可执行的操作
第一版 Web 页面主要提供以下能力：

- 查看当前 Source 数量
- 查看当前 Task 数量
- 查看当前模式为 `Phase 1 skeleton`
- 通过表单新增 Source
- 查看 Recent Tasks
- 查看已有 Sources

### 5.3 新增 Source 的操作步骤
1. 打开首页
2. 在 **Add source** 区域选择类型：
   - `video`
   - `playlist`
   - `channel`
3. 填写 YouTube URL
4. 点击 **Create source**
5. 页面成功提交后会刷新
6. 新 Source 会显示在 Sources 表格中
7. 系统会自动生成一条 `queued` 状态的 `index` Task

### 5.4 URL 格式要求
Web 页面提交时会调用后端校验，当前要求如下：

#### video
支持示例：
- `https://www.youtube.com/watch?v=VIDEO_ID`
- `https://youtu.be/VIDEO_ID`

#### playlist
支持示例：
- `https://www.youtube.com/playlist?list=PLAYLIST_ID`
- 或任意带 `list=` 参数的 YouTube URL

#### channel
支持示例：
- `https://www.youtube.com/channel/CHANNEL_ID`
- `https://www.youtube.com/@handle`
- `https://www.youtube.com/c/name`
- `https://www.youtube.com/user/name`

### 5.5 页面定位说明
当前 Web 页面是一个**轻量控制面原型**，定位是：

- 验证交互闭环
- 支撑内部测试
- 展示状态与最小操作入口

不属于正式完备的运营控制台。

---

## 6. 如何使用 CLI

### 6.1 启动方式
可直接使用 Go 运行：

```bash
go run ./apps/cli
```

也可先构建：

```bash
make build
```

构建后生成：

- `bin/ytvault-api`
- `bin/yd`

CLI 主程序为：

```bash
./bin/yd
```

### 6.2 API 地址配置
CLI 默认访问：

```text
http://127.0.0.1:8080
```

如需修改，可设置：

```bash
export YTVAULT_API=http://127.0.0.1:8081
```

### 6.3 当前支持命令

#### 健康检查
```bash
yd health
```

作用：调用 `/healthz`，检查 API 是否在线。

#### 查看 Source 列表
```bash
yd source list
```

作用：查询当前所有 Source。

#### 新增 video Source
```bash
yd source add video "https://www.youtube.com/watch?v=VIDEO_ID"
```

#### 新增 playlist Source
```bash
yd source add playlist "https://www.youtube.com/playlist?list=PLAYLIST_ID"
```

#### 新增 channel Source
```bash
yd source add channel "https://www.youtube.com/@example"
```

作用：创建 Source，并自动生成一条初始 `index` Task。

#### 查看 Task 列表
```bash
yd task list
```

作用：查看当前任务列表。

### 6.4 推荐使用方式
对内部使用者而言，建议：

- **人工试用 / 验证流程**：优先使用 Web 页面
- **批量调用 / 脚本集成 / 自动化检查**：优先使用 CLI

### 6.5 Channel 测试说明入口（预留）
为配合本轮 channel 支持推进，使用手册建议预留一个独立的测试说明入口，便于后续把“测试对象、操作步骤、结果口径、遗留问题”沉淀到统一位置。

当前建议入口如下：

- **测试输入说明**：`docs/test-inputs.md`
- **阶段记录模板**：`docs/YD-CHANNEL-PHASE-RECORD-TEMPLATE.md`

当前可先将 `docs/test-inputs.md` 视为 channel 测试输入登记页，重点记录：

- 测试频道 URL
- 测试目的（URL 校验 / Source 创建 / Task 入列 / 后续 E2E）
- 当前系统支持边界
- 后续补测建议

建议后续 channel 支持完成后，在本手册补充一节正式内容，至少覆盖：

1. **推荐测试前提**：API 启动方式、数据文件路径、是否需要 cookies/凭据
2. **Web 测试步骤**：通过页面新增 channel Source，并确认 Source 与 Task 状态
3. **CLI 测试步骤**：使用 `yd source add channel <url>` 与 `yd task list` 验证链路
4. **预期结果**：Source 创建成功、自动生成 `index` Task、状态字段符合预期
5. **失败排查入口**：URL 格式、日志位置、数据文件、未实现能力边界

在真实 channel 执行链路尚未落地前，本节只作为“测试说明入口预留”，避免手册过早写成已交付能力说明。

---

## 7. 当前已知限制

第一版 skeleton 的限制需要明确说明，避免被误认为是完整产品版本：

### 7.1 尚未真正执行下载
当前系统只会：

- 校验 URL
- 创建 Source
- 生成 `index` 任务
- 持久化状态

**尚未实现**：

- 实际索引频道/列表/视频内容
- 调用 `yt-dlp` 下载媒体
- 调用 `ffmpeg` 做后处理
- 媒体文件落盘
- 元数据抓取与 sidecar 生成

### 7.2 当前存储仍是本地 JSON
目前不是生产级数据库方案，仍使用单文件状态存储：

- 无 PostgreSQL
- 无 Redis
- 无任务队列
- 无分布式锁
- 无并发调度体系

因此当前版本仅适合：

- 本地开发
- 单机验证
- 原型演示

### 7.3 Web 页面仍为嵌入式简易页面
当前 Web 不是独立前端工程，缺少：

- Dashboard 深度统计
- Source 详情页
- 错误中心
- Credentials 管理页
- Settings 页面
- 复杂筛选与分页能力

### 7.4 CLI 命令覆盖有限
当前 CLI 仅支持：

- health
- source list
- source add
- task list

尚未支持：

- source reindex
- task retry
- media list
- doctor
- credentials import

### 7.5 Source 信息仍为占位状态
Source 创建后：

- `title` 固定为 `pending resolution`
- Task 仅处于 `queued`
- 没有真实执行进度
- 没有失败分类与重试机制

### 7.6 尚未接入凭据 / Cookies
当前尚未支持：

- cookies 导入
- 凭据配置绑定
- 登录态校验
- bot-check 诊断

这意味着当前版本还无法覆盖需要认证或带限制的视频访问场景。

---

## 8. 下一步计划

结合当前架构文档与项目规划，建议下一阶段按以下优先级推进：

### 8.1 API 与持久化升级
- 从本地 JSON 存储迁移到 PostgreSQL
- 建立标准化 Source / Task / Media 数据模型
- 增加迁移脚本与初始化能力

### 8.2 任务队列与调度器落地
- 引入 Redis 作为队列与瞬时状态层
- 实现 Scheduler
- 支持任务去重、重试、退避与心跳

### 8.3 单视频链路优先打通
建议优先实现 **single video one-shot** 路径：

- URL 解析
- 元信息抓取
- 下载执行
- 文件落盘
- Task 状态闭环

这是最小可验证业务路径，适合先形成“真正可用”的第一条生产链路。

### 8.4 下载器与后处理能力
- 接入 `yt-dlp`
- 接入 `ffmpeg`
- 支持格式选择、保存路径、文件命名模板
- 建立下载记录与失败分类

### 8.5 凭据 / Cookies 管理
- 支持 cookies 导入
- 支持凭据配置绑定到 Source
- 增加认证健康检查
- 明确 AUTH/BOT_CHECK 失败原因

### 8.6 运维可观测性
- 增加 doctor / diagnostics 能力
- 增加错误分类与任务详情
- 增加 worker heartbeat 与 stuck job 检测

### 8.7 Web / CLI 补全
- Web：补齐 Dashboard、Tasks、Errors、Credentials、Settings 页面
- CLI：补齐 reindex、retry、doctor、credentials 等核心命令

---

## 9. 建议的内部使用口径

对于当前第一版，建议内部统一表述为：

> YtVault 已完成 Phase 1 的基础骨架与控制面雏形，当前可用于 Source 建模、任务入列演示与接口联调；尚未进入完整下载执行与生产级调度阶段。

这样既能体现项目已具备明确结构与可操作入口，也能避免老板或使用方对“已可归档下载”产生过高预期。

---

## 10. 附：常用启动命令速查

### 启动 API
```bash
cd /home/xiaob/.openclaw/workspace/projects/YtVault
make run-api
```

### 启动 CLI（Go 方式）
```bash
cd /home/xiaob/.openclaw/workspace/projects/YtVault
go run ./apps/cli health
```

### 构建二进制
```bash
cd /home/xiaob/.openclaw/workspace/projects/YtVault
make build
```

### 查看 Source
```bash
./bin/yd source list
```

### 新增单视频 Source
```bash
./bin/yd source add video "https://www.youtube.com/watch?v=VIDEO_ID"
```

### 查看 Task
```bash
./bin/yd task list
```

---

如需，我下一步可以继续补一版：

1. **更适合老板汇报的 1 页版说明**（偏管理视角）
2. **更适合研发交接的 Runbook 版**（偏操作与排障视角）
3. **直接替换 README 的精简正式版**
