# Changelog

本文件记录 HateSentry 已发布版本的用户可见变化。版本号遵循语义化版本。

## [Unreleased]

## [0.2.0] - 2026-07-12

### Added

- 新增同源 Vue 管理控制台，覆盖待处理队列、审核历史、客户端管理、Webhook 投递和运营概览。
- 新增审核历史游标分页、人工状态筛选和详情追溯。
- 新增外部客户端创建、启停、API Key 轮换、策略分配与恢复默认策略操作。
- 新增客户端 Webhook URL 配置、签名 secret 轮换和清除操作。
- 新增 Webhook 最新投递状态查询、筛选和失败手动重试。
- 新增审核、人工复核和 Webhook 最新状态基础运营指标。
- 新增生产环境配置预检和 MySQL 备份恢复验收流程。

### Changed

- 审核统计改为从同一个只读 `REPEATABLE READ` 数据库快照计算。
- Docker Compose 的数据库和 RabbitMQ 账号密码支持从环境变量覆盖。
- Release 配置支持为 Redis 设置认证密码，并在 API 与健康检查中使用同一配置。
- API 健康响应和 Compose 镜像标签统一携带 `0.2.0` 版本。
- 控制台完整 API Key 与 Webhook secret 只进入一次性内存面板，不写入浏览器持久化存储。

### Security

- Webhook URL 会拒绝 localhost、内网、链路本地、组播和元数据服务地址，并在发送前检查 DNS 解析结果。
- Webhook 投递错误在持久化和运营输出边界归一化，避免暴露 URL 查询参数、底层网络错误全文或数据库详情。
- Release 预检会拒绝开发默认密码、占位 provider key、明显测试或低多样性 secret、secret 复用、空 Redis 密码、root 数据库应用账号和 guest RabbitMQ 账号。

## [0.1.0] - 2026-07-12

### Added

- 首个文本审核 API MVP。
- 同步 `POST /api/v1/moderation/check` 工作流及 `allow`、`review`、`block` 服务端策略决策。
- 审核请求、结果、人工复核、客户端 API Key、Webhook 最终决策回调和基础 Prometheus 指标。
- MySQL、Redis、RabbitMQ 与 API 的 Docker Compose 本地运行环境。

[Unreleased]: https://github.com/astenir/HateSentry/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/astenir/HateSentry/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/astenir/HateSentry/releases/tag/v0.1.0
