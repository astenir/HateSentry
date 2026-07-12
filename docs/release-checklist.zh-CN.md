# HateSentry MVP 发布验收清单

本文用于验收 `v0.2.0` 及后续兼容版本。它不替代生产环境的数据库、网络和密钥管理制度。

## 1. 发布边界

当前稳定主线包括：

- 同步文本审核及 `allow`、`review`、`block` 决策；
- 审核记录与结果查询；
- 人工复核队列和审核历史；
- 外部客户端 API Key、策略分配和幂等请求；
- Webhook 最终决策回调、后台重试和运营查询；
- 同源管理控制台与基础累计指标。

异步审核队列、批量审核、真实图片审核、Webhook 逐次 attempt 历史和时间趋势报表不属于当前发布边界。

## 2. 代码与 CI

发布提交必须满足：

```bash
git status --short --branch
git diff --check
make web-test web-build
make test-release-tools
go test ./...
go build ./...
go test -p 1 -tags=integration ./...
make verify-compose
make smoke-console-local
```

`make smoke-console-local` 会使用临时空数据库完成管理员初始化、审核、人工复核、客户端、策略、API Key、Webhook、运营概览全流程；随后通过 `mysqldump --single-transaction` 备份数据库，恢复到第二个临时数据库并对关键业务表逐表核对记录数。流程会清理两个临时数据库、本地 dump 和 API 日志；主流程成功但清库失败时，验收会返回非零并报告数据库名。

远端 CI 必须同时通过 Unit tests and build、MySQL integration tests、Local MVP and console smoke workflow。

## 3. 生产配置预检

复制发布模板到仓库外的私有路径：

```bash
cp config/release.env.example /secure/path/hatesentry-release.env
chmod 600 /secure/path/hatesentry-release.env
```

使用 `openssl rand -hex 32` 分别生成独立随机值，不复用 API Key、JWT secret、数据库密码或 Webhook secret。

首次部署空数据库时：

```bash
RELEASE_ENV_FILE=/secure/path/hatesentry-release.env \
RELEASE_BOOTSTRAP=1 \
make release-preflight
```

预检会拒绝错误的 `APP_VERSION`、非 `release` 运行模式、已知占位值、明显测试值、低多样性或过短 secret、secret 复用、空 Redis 密码、root 数据库应用账号、guest RabbitMQ 账号、OpenAI 模式缺失结构上有效的 provider key 候选、首次初始化缺失 bootstrap token，以及可被 group/other 访问的私有 env 文件。推荐权限为 `0600`，更严格的 owner-only 权限也可通过。错误只打印变量名和约束，不打印变量值。

为避免和 Compose dotenv 展开产生歧义，预检只接受简单的 `KEY=VALUE`、引号值和行尾注释，不接受变量插值或反斜杠转义。发布 secret 应使用文档建议的随机十六进制值。

预检是离线结构校验，不能证明 provider key 尚未过期、具有额度或能访问目标模型。正式切流前还必须通过受控的真实审核请求验证 provider；请求和日志不得回显 key。

Compose 展开也必须成功：

```bash
docker compose --env-file /secure/path/hatesentry-release.env config --quiet
```

`MYSQL_ROOT_PASSWORD`、`DB_USERNAME`、`DB_PASSWORD` 及 RabbitMQ 初始化账号只会在新数据卷首次创建时初始化服务端账号。修改已有部署的 env 文件不会自动轮换数据库或 RabbitMQ 中已经存在的凭据；应先按对应产品的安全流程完成服务端凭据轮换，再更新 env 并重建容器。

从 `v0.1.0` 复制的旧 `.env` 可能仍包含 `DB_USERNAME=root`。在删除旧卷、迁移新主机或从空卷重建前，必须改成非 root 应用账号；MySQL 官方镜像不允许通过 `MYSQL_USER=root` 初始化新空卷。恢复已有数据卷时，还必须确认该应用账号已经存在并拥有目标数据库权限。

默认 Compose 只把 MySQL、Redis、RabbitMQ 和 RabbitMQ 管理端口绑定到宿主机 `127.0.0.1`，不会发布到外部网卡。跨主机使用托管依赖时，应移除这些本地端口映射并在外部网络层配置 TLS、访问控制和防火墙，不应把明文 Redis 或 AMQP 端口直接暴露到不可信网络。

## 4. 空库部署与管理员初始化

```bash
docker compose --env-file /secure/path/hatesentry-release.env up -d --build
docker compose ps
curl -fsS http://127.0.0.1:8080/api/v1/health
curl -fsS http://127.0.0.1:8080/console/ >/dev/null
```

健康响应必须包含 `"version":"0.2.0"`。Compose 构建的本地镜像标签必须是 `hatesentry:0.2.0`，发布记录中保存其不可变镜像 ID；如果镜像推送到 registry，还应保存 registry 返回的 digest：

```bash
docker image inspect hatesentry:0.2.0 --format '{{.Id}}'
```

使用与 `ADMIN_BOOTSTRAP_TOKEN` 一致的请求初始化第一个管理员。成功后立即清空私有 env 文件中的该变量，重建 API 容器，并按已有部署模式再次预检：

```bash
RELEASE_ENV_FILE=/secure/path/hatesentry-release.env make release-preflight
docker compose --env-file /secure/path/hatesentry-release.env up -d --force-recreate hatesentry
```

已有部署模式下，只要 `ADMIN_BOOTSTRAP_TOKEN` 仍非空，预检就会失败。

## 5. API 契约抽查

验收至少覆盖：

- 管理员 JWT 登录成功，错误密码被拒绝；
- API Key 客户端可以调用 `POST /api/v1/moderation/check`；
- 相同客户端和 `external_id` 不产生重复活动记录；
- 审核结果包含请求 ID、决策、风险、标签、原因和策略版本；
- `review` 决策会创建待处理复核案件；
- 人工通过、拒绝和标记误判保存独立最终状态；
- 完整 API Key 和 Webhook secret 只在创建或轮换响应中出现一次；
- 客户端列表、浏览器存储和运营列表不泄露 secret；
- Webhook 重试会增加累计尝试次数；
- 运营概览数字与 `GET /api/v1/reviews/stats` 一致。

自动化对应命令是 `make smoke-console-local`。

## 6. 备份与恢复

自动验收只证明当前 schema 和关键表可通过标准 MySQL dump/restore 恢复。生产发布前仍应在隔离环境中使用真实备份副本演练：

```bash
mysqldump --single-transaction --skip-lock-tables --no-tablespaces \
  -h <host> -u <backup-user> -p <database> > hatesentry.sql

mysql -h <restore-host> -u <restore-user> -p <restore-database> < hatesentry.sql
```

恢复后核对 `users`、`client_applications`、`moderation_requests`、`moderation_results`、`review_cases` 和 `webhook_deliveries`。

备份文件可能包含用户内容、Webhook payload 和哈希凭据，必须加密保存、限制权限并按数据保留策略删除。

## 7. 发布与回滚

发布前确认目标提交已经在 `origin/main`，且该提交远端 CI 全绿。标签必须是新标签，不得移动已有版本：

```bash
set -e
git fetch origin main --tags
test "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)"
test -z "$(git tag --list v0.2.0)"
remote_tag="$(git ls-remote --tags origin refs/tags/v0.2.0)"
test -z "$remote_tag"
git tag -a v0.2.0 -m "HateSentry 内容审核运营 MVP"
test "$(git rev-parse 'v0.2.0^{}')" = "$(git rev-parse HEAD)"
git push origin v0.2.0
remote_commit="$(git ls-remote --tags origin 'refs/tags/v0.2.0^{}' | cut -f1)"
test "$remote_commit" = "$(git rev-parse HEAD)"
```

发布说明以 `CHANGELOG.md` 对应版本为准。应用回滚使用上一稳定标签重新构建；数据库回滚必须使用发布前备份，不要假设应用镜像回退会自动撤销 schema 或业务数据变化。
