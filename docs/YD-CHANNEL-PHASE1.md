# YtVault Channel 支持：Phase 1 设计与验收定义

## 0. 文档目的
本文把 **channel source** 在 Phase 1 的目标范围、用户流、URL 规则、数据表现与验收标准钉清楚，避免把当前 skeleton 误当成完整下载能力。

测试目标频道：`http://www.youtube.com/@SpeechLab-m7o`

设计遵循现有原则：
- API-first
- Web / CLI 共享同一后端能力
- index / download / metadata 分离
- 明确区分业务状态与执行状态
- 强调可恢复、可诊断，而不是先堆复杂 UI

---

## 1. Phase 1 目标范围（channel source）

### 1.1 要解决的问题
让用户能把 **YouTube channel** 作为一等 source 添加进系统，而不是 playlist 变通方案；并且后续可自然接入真实索引与下载链路。

### 1.2 Phase 1 in-scope
Phase 1 的 channel source 目标分两层：

#### A. 当前 skeleton 已满足
- 可以通过 **Web / CLI / API** 新增 `channel` 类型 source
- 会做基础 YouTube URL 校验
- 可以把 source 持久化保存
- 会自动生成一条初始 `index` task（状态为 `queued`）
- 可用于后续真实 worker 接入前的接口联调与状态建模验证

#### B. 下一阶段必须补齐
- channel 真实索引：解析频道、发现视频、落媒体记录
- 周期性同步：支持 recurring sync，而不是只停留在“可创建 source”
- 下载链路接入：由 index 结果驱动 download task
- 失败分类与重试：能区分 source 无效、上游限制、认证问题等
- source 详情可观测：最近索引时间、成功时间、错误状态、发现条目数

### 1.3 Phase 1 out-of-scope
以下不作为本设计的第一优先级验收项：
- 复杂筛选策略（如仅下载 Shorts / 指定时间窗）
- 频道高级元数据完整归档
- UI 上复杂的 source 编辑器
- 多租户 / 权限体系
- 非 YouTube 平台扩展

---

## 2. 用户流：如何添加 channel

## 2.1 Web 用户流
目标：最低成本完成“添加 channel source”。

### 当前 skeleton 已满足
1. 打开首页
2. 在 **Add source** 表单中选择 `channel`
3. 输入 channel URL
4. 点击 `Create source`
5. 后端校验 URL
6. 成功后：
   - 创建 source
   - 创建一条 `index` task
   - 页面刷新后在 Sources / Recent Tasks 中可见

### 下一阶段要满足
- 表单级错误提示更明确（区分 host/path/缺失 ID 等）
- 成功后跳转 source detail 或至少高亮新建 source
- 可手动触发 `reindex`
- 可查看最近索引结果、最近错误与发现数量

## 2.2 CLI 用户流
命令目标：让自动化和脚本集成不依赖 Web。

### 当前 skeleton 已满足
```bash
yd source add channel "http://www.youtube.com/@SpeechLab-m7o"
```

预期：
- 请求 API 成功
- 输出创建后的 source 与 task JSON

### 下一阶段要满足
- 返回码与错误信息可直接用于脚本判断
- 增加：
  - `yd source reindex <id>`
  - `yd source get <id>`
  - `yd task retry <id>`
- 对非法 URL 输出稳定错误码/文案

## 2.3 API 用户流
目标：API 作为唯一真实写入口。

### 当前 skeleton 已满足
`POST /api/v1/sources`

请求：
```json
{
  "kind": "channel",
  "url": "http://www.youtube.com/@SpeechLab-m7o"
}
```

成功响应：`201 Created`
- 返回 `source`
- 返回自动生成的初始 `task`

### 下一阶段要满足
- source 响应增加更明确字段（如 `external_id`、`last_index_at`、`last_error_code` 等）
- API 对重复 source、不可接受 URL、规范化结果给出稳定行为
- 可触发 reindex / task retry / source detail 查询

---

## 3. URL 校验与接受规则

## 3.1 Phase 1 接受规则
`channel` 类型只接受 YouTube 域名下、且 path 明确代表“频道”的 URL。

当前代码已接受：
- `https://www.youtube.com/channel/CHANNEL_ID`
- `https://www.youtube.com/@handle`
- `https://www.youtube.com/c/name`
- `https://www.youtube.com/user/name`
- host 可为：
  - `youtube.com`
  - `www.youtube.com`
  - `m.youtube.com`
- scheme 可为：
  - `http`
  - `https`

因此测试目标 `http://www.youtube.com/@SpeechLab-m7o` **应被接受**。

## 3.2 当前 skeleton 已满足
- 拒绝非 `http/https`
- 拒绝非 YouTube 域名
- 拒绝与 `channel` 类型不匹配的 path（例如 watch URL）
- 对 `@handle` / `channel/` / `c/` / `user/` 进行基础识别

## 3.3 下一阶段建议收紧/补齐
为避免 source 语义漂移，下一阶段建议明确：

### 应继续接受
- `http` 与 `https`（录入时接受，内部可规范化为 `https`）
- `youtube.com` / `www.youtube.com` / `m.youtube.com`
- `/@handle`
- `/channel/<id>`
- `/c/<name>`
- `/user/<name>`

### 应拒绝
- `youtu.be/...`（这是视频短链，不是 channel）
- `/watch?...`
- `/playlist?...`
- 根路径或空 path
- 无法判断为频道资源的 path

### 建议新增的规范化规则
录入成功后建议保存两类值：
- `original_url`：用户原始输入
- `url`：系统规范化后的 canonical URL

最小规范化建议：
- host 统一成 `www.youtube.com`
- 可移除无关 query / fragment
- `http` 统一转为 `https`
- 保留 path 语义，不擅自把 `@handle` 改写成 `channel/ID`（除非后续索引已成功解析出 canonical channel ID）

---

## 4. 期望的数据表现（source / task）

## 4.1 source 层

### 当前 skeleton 已满足
创建 channel source 后，至少应有：
- `id`
- `kind=channel`
- `url`
- `title="pending resolution"`
- `status=active`
- `createdAt`
- `updatedAt`

这说明当前 skeleton 的 source 语义是：
**“用户已声明一个待解析的频道源”**，而不是“系统已完成频道识别”。

### 下一阶段要满足
channel source 至少应进一步具备：
- `external_id`：解析出的频道稳定标识（若可得）
- `title`：频道标题，不再是 placeholder
- `last_index_at`
- `last_success_index_at`
- `next_index_at`
- `last_error_code` / `last_error_message`（至少二选一或通过 task/event 关联可见）
- 可选：`canonical_url`

## 4.2 task 层

### 当前 skeleton 已满足
新增 source 后自动生成一条：
- `type=index`
- `entityType=source`
- `entityId=<src_id>`
- `status=queued`

这符合现有架构里“新增 source -> 初始 index task”的设计方向。

### 当前 skeleton 尚未满足
- task 不会真实执行
- 没有 `failed` 状态
- 没有错误码/错误信息
- 没有 retry / backoff / heartbeat
- 没有 job event 明细

### 下一阶段要满足
至少应支持：
- `queued / running / succeeded / failed`
- `index` 与后续 `download` 分离
- 失败时可见错误分类
- 同一 source 的重复索引具备幂等/去重策略

---

## 5. 真实下载链路接入前后的验收标准

## 5.1 接入前（当前 skeleton 阶段）
这是“控制面闭环验收”，不是“真实下载验收”。

### 必须通过
1. `channel` 可通过 Web / CLI / API 创建
2. 测试 URL `http://www.youtube.com/@SpeechLab-m7o` 可被接受
3. 创建后 source 被持久化保存
4. 自动生成一条 `index` task
5. Source 列表和 Task 列表都能看到新记录
6. 非法 channel URL 会被 API 以 `400 invalid_source` 拒绝

### 不应误判为已完成
以下即使未实现，也不应阻塞 skeleton 验收：
- 真实访问 YouTube 获取频道元数据
- 发现频道视频列表
- 创建 media item
- 触发 download task
- 落盘媒体文件

## 5.2 接入后（真实索引 / 下载链路阶段）
这是“业务闭环验收”。

### channel 索引验收
1. 新增 `channel` source 后，index worker 能实际处理该 source
2. 能成功解析目标频道基本信息（至少标题 / 可识别的频道标识）
3. 能发现该频道下可见视频，并写入 media 记录
4. 重复执行 index 不产生无意义重复媒体记录
5. source 的最近索引时间、最近成功时间、错误状态可见

### channel 下载验收
1. index 结果可按策略生成 `download` task
2. downloader 能把目标媒体正确下载到挂载目录
3. 下载成功后状态从 source/task/media 侧能一致反映
4. 若下载失败，错误可诊断且可重试
5. 重启服务后状态不损坏，未完成任务可恢复或可重新调度

---

## 6. 给乔的集成 / 验收测试点列表

以下列表按“现在就能做 / 下一阶段必须做”拆开，方便直接变成测试用例。

## 6.1 现在就能做（skeleton）

### API
- `POST /api/v1/sources` + `kind=channel` + `http://www.youtube.com/@SpeechLab-m7o` 返回 `201`
- 返回体中：
  - `source.kind == channel`
  - `source.url == 原始输入 URL`
  - `source.status == active`
  - `task.type == index`
  - `task.status == queued`
- `GET /api/v1/sources` 可见新 source
- `GET /api/v1/tasks` 可见新 task
- 非法 URL（如 `https://www.youtube.com/watch?v=abc` 且 kind=channel）返回 `400`

### CLI
- `yd source add channel "http://www.youtube.com/@SpeechLab-m7o"` 成功
- `yd source list` 可看到新 source
- `yd task list` 可看到 `index` task
- 非法 URL 时 CLI 输出后端错误信息

### Web
- 首页表单选择 `channel` 并提交目标 URL 可成功创建
- 刷新后 Sources 表格出现该 source
- Recent Tasks 出现对应 `index` task
- 提交非法 URL 时页面可看到错误返回文本

## 6.2 下一阶段必须做（真实链路）

### URL / source 规范化
- 输入 `http://www.youtube.com/@SpeechLab-m7o` 后，系统可选择性规范化保存为 canonical URL
- 保留原始输入，避免用户排障时丢失上下文
- 重复添加同一 channel 的语义应明确：
  - 要么拒绝重复
  - 要么允许但需有明确去重/说明

### Index
- 能对目标频道完成真实索引
- 能发现至少一条媒体记录
- 二次索引不产生重复媒体
- 对频道不存在 / 迁移 / 私有 / 访问受限给出正确错误分类

### Download
- 对已发现媒体生成 download task
- 任务执行后文件真实落盘
- 失败后可 retry
- 服务重启后不丢状态

### Observability
- source detail 可见：
  - 最近索引时间
  - 最近成功时间
  - 最近错误
  - 发现数量 / 下载数量（至少一部分）
- task 可见：
  - 运行中 / 成功 / 失败
  - 错误码与错误文案

---

## 7. 核心判断
1. **当前 skeleton 已经具备 channel source 的“录入闭环”**，包括 Web / CLI / API 入口、URL 校验、持久化与初始 `index` task 生成。
2. **当前还不具备 channel source 的“业务闭环”**：没有真实索引、没有 media、没有 download、没有失败分类与恢复机制。
3. 对 Phase 1 来说，channel 必须坚持作为 **一等 source** 存在；`http://www.youtube.com/@SpeechLab-m7o` 应作为固定验收样例持续保留。
4. 下一阶段最关键的不是先做复杂 UI，而是补齐：
   - channel 真实索引
   - source/task 状态扩展
   - 幂等/重复控制
   - 下载链路与错误可诊断性
