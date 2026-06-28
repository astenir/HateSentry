# Prometheus + Grafana 监控系统使用指南

## 📊 监控架构

HateSentry 项目集成了完整的 Prometheus + Grafana 监控栈，提供：

- **指标采集**: Prometheus 收集应用和系统指标
- **可视化**: Grafana 提供实时监控仪表盘
- **告警**: Alertmanager 处理告警路由和通知
- **日志聚合**: Loki + Promtail 处理日志收集
- **日志分析**: Kibana 提供高级日志分析（可选）

## 🚀 快速启动

### 1. 启动监控服务

```bash
# 启动所有监控组件
docker-compose -f docker-compose.monitoring.yml up -d

# 检查服务状态
docker-compose -f docker-compose.monitoring.yml ps
```

### 2. 访问监控界面

| 服务 | 地址 | 默认账号 |
|------|------|----------|
| Prometheus | http://localhost:9090 | - |
| Grafana | http://localhost:3000 | admin/admin123 |
| Alertmanager | http://localhost:9093 | - |
| Loki | http://localhost:3100 | - |
| Kibana | http://localhost:5601 | - |
| Node Exporter | http://localhost:9100/metrics | - |
| cAdvisor | http://localhost:8081 | - |

### 3. 配置 Grafana 数据源

1. 登录 Grafana (admin/admin123)
2. 进入 Configuration → Data sources
3. 添加 Prometheus 数据源:
   - URL: http://prometheus:9090
4. 添加 Loki 数据源（用于日志）:
   - URL: http://loki:3100

## 📈 指标说明

### HTTP 指标

- `http_requests_total`: HTTP 请求总数（按方法、端点、状态码分组）
- `http_request_duration_seconds`: 请求延迟（直方图）
- `http_request_size_bytes`: 请求大小
- `http_response_size_bytes`: 响应大小

### AI 检测指标

- `detection_requests_total`: 检测请求数（按类型、提供者分组）
- `detection_duration_seconds`: 检测耗时
- `detection_results_total`: 检测结果统计
- `detection_confidence`: 检测置信度分布

### 文本审核指标

- `moderation_checks_total`: 成功完成的文本审核请求数，标签为 `decision`、`provider`、`client_type`。
- `moderation_check_duration_seconds`: 成功文本审核耗时，标签为 `decision`、`provider`、`client_type`。
- `review_cases_finalized_total`: 人工复核完成数，标签为 `status`、`final_decision`。
- `review_case_latency_seconds`: 从复核记录创建到人工完成的耗时，标签为 `status`、`final_decision`。
- `webhook_deliveries_total`: 最终决策 Webhook 投递尝试数，标签为 `status`、`trigger`。
- `webhook_delivery_duration_seconds`: 最终决策 Webhook 投递耗时，标签为 `status`、`trigger`。
- `webhook_retry_batches_total`: 后台 Webhook 自动重试批次数，标签为 `result`。
- `webhook_retry_batch_deliveries_total`: 后台 Webhook 自动重试批次处理的 delivery 记录数，标签为 `result`。

Webhook 指标只使用固定枚举标签，例如 `succeeded`、`failed`、`initial`、`manual_retry`、`automatic_retry`、`completed` 和 `skipped`；不要把请求 ID、客户端 ID、URL、域名或原始错误文本放入 Prometheus 标签。

### 数据库指标

- `db_queries_total`: 数据库查询数
- `db_query_duration_seconds`: 查询耗时
- `db_connections_active`: 活跃连接数
- `db_connections_idle`: 空闲连接数

### 缓存指标

- `cache_hits_total`: 缓存命中数
- `cache_misses_total`: 缓存未命中数
- `cache_size`: 缓存大小

### 消息队列指标

- `mq_messages_published_total`: 发布消息数
- `mq_messages_consumed_total`: 消费消息数
- `mq_messages_failed_total`: 失败消息数
- `mq_queue_size`: 队列大小
- `mq_consumer_lag`: 消费者延迟

### 系统指标

- `active_connections`: 活跃连接数
- `active_goroutines`: Goroutine 数量
- `memory_usage_bytes`: 内存使用量

## 🎯 告警规则

已配置的告警规则包括：

### 应用健康告警

- `ApplicationDown`: 应用停止运行
- `HighErrorRate`: 错误率超过 10%
- `HighLatency`: P95 延迟超过 1 秒
- `VeryHighLatency`: P95 延迟超过 5 秒

### 检测服务告警

- `DetectionServiceDown`: 检测服务无响应
- `HighDetectionFailureRate`: 检测成功率低于 95%

### 数据库告警

- `DatabaseDown`: 数据库连接失败
- `SlowDatabaseQueries`: 慢查询（>500ms）
- `DatabaseConnectionPoolExhausted`: 连接池耗尽

### 缓存告警

- `CacheDown`: Redis 连接失败
- `LowCacheHitRate`: 缓存命中率低于 60%

### 系统资源告警

- `HighMemoryUsage`: 内存使用率超过 85%
- `HighCPUUsage`: CPU 使用率超过 80%
- `DiskSpaceRunningLow`: 磁盘空间剩余低于 15%

## 📊 Grafana 仪表盘

### 推荐仪表盘

#### 1. HateSentry 概览仪表盘

包含以下面板：
- 请求速率和错误率
- 响应时间分布
- 检测性能指标
- 缓存命中率
- 系统资源使用率

#### 2. 应用性能仪表盘

- 请求瀑布图
- 端点性能对比
- 数据库查询性能
- 缓存效率分析

#### 3. 系统健康仪表盘

- CPU、内存、磁盘使用率
- 网络流量
- 容器状态
- 服务健康检查

导入仪表盘：
1. Grafana → Dashboards → Import
2. 导入 JSON 配置文件（参考 `docs/monitoring/grafana-dashboards/`）

## 📝 日志管理

### Loki 日志查询

使用 LogQL (Log Query Language) 查询日志：

```logql
# 查看所有 HateSentry 应用日志
{job="hatesentry"}

# 查看错误日志
{job="hatesentry"} |= "error"

# 查看特定请求的日志
{job="hatesentry"} |= "request_id=xxx"

# 查看慢查询日志
{job="hatesentry"} |= "slow_query"

# 统计错误率
count_over_time({job="hatesentry"} |= "error" [5m])
```

### 日志转发配置

配置日志转发到 Elasticsearch：

```go
import "hatesentry/internal/logging"

esConfig := &logging.ElasticsearchConfig{
    Endpoint:      "http://elasticsearch:9200",
    Index:         "hatesentry-logs",
    Username:      "elastic",
    Password:      "your-password",
    BatchSize:     100,
    FlushInterval: 5 * time.Second,
}

esForwarder := logging.NewElasticsearchForwarder(esConfig, logger)
```

## 🔧 配置调整

### Prometheus 配置

编辑 `config/monitoring/prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'hatesentry-app'
    scrape_interval: 10s  # 调整采集间隔
    static_configs:
      - targets: ['your-host:8080']
```

重载配置：
```bash
docker-compose -f docker-compose.monitoring.yml exec prometheus kill -HUP 1
```

### 告警规则

编辑 `config/monitoring/rules.yml`:

```yaml
- alert: HighLatency
  expr: histogram_quantile(0.95, ...) > 2  # 调整阈值
  for: 10m  # 调整持续时间
```

重载告警规则：
```bash
docker-compose -f docker-compose.monitoring.yml exec prometheus kill -HUP 1
```

### Alertmanager 配置

编辑 `config/monitoring/alertmanager.yml` 配置通知渠道（Slack、Email、Webhook）。

重载配置：
```bash
docker-compose -f docker-compose.monitoring.yml restart alertmanager
```

## 🎓 最佳实践

### 1. 指标设计

- 使用有意义的命名（如 `http_requests_total`）
- 添加适当的标签（如 `method`, `endpoint`, `status_code`）
- 使用直方图记录延迟分布
- 记录 Counter 类型的事件总数

### 2. 告警策略

- 设置合理的阈值（避免告警疲劳）
- 配置告警抑制（防止告警风暴）
- 使用分级告警（warning/critical）
- 配置告警恢复通知

### 3. 仪表盘设计

- 关注关键指标（SLA、SLO）
- 使用聚合视图（按时间、服务、环境）
- 添加阈值线和目标线
- 提供下钻能力

### 4. 日志记录

- 使用结构化日志（JSON 格式）
- 添加关键上下文（request_id, user_id）
- 记录适当的日志级别（DEBUG/INFO/WARN/ERROR）
- 避免记录敏感信息

## 📚 相关资源

- [Prometheus 文档](https://prometheus.io/docs/)
- [Grafana 文档](https://grafana.com/docs/)
- [Loki 文档](https://grafana.com/docs/loki/latest/)
- [Alertmanager 文档](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [Kibana 文档](https://www.elastic.co/guide/en/kibana/current/index.html)

## 🐛 故障排查

### Prometheus 无法采集指标

检查目标是否暴露指标端点：
```bash
curl http://localhost:8080/metrics
```

检查 Prometheus 配置：
```bash
docker-compose -f docker-compose.monitoring.yml exec prometheus promtool check config /etc/prometheus/prometheus.yml
```

### Grafana 无法连接 Prometheus

检查数据源配置：
1. URL 应为 `http://prometheus:9090`
2. 检查网络连接
3. 查看 Grafana 日志

### 告警未触发

检查告警规则：
```bash
docker-compose -f docker-compose.monitoring.yml exec prometheus promtool check rules /etc/prometheus/rules/*.yml
```

查看 Alertmanager 日志：
```bash
docker-compose -f docker-compose.monitoring.yml logs alertmanager
```

## 📞 支持

如有问题，请提交 Issue 或联系技术支持团队。
