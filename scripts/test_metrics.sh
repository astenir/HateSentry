#!/bin/bash

# 测试 metrics 端点的脚本

echo "=== 测试 /metrics 端点 ==="
echo ""

# 检查应用是否在运行
if ! curl -s http://localhost:8080/api/v1/health > /dev/null 2>&1; then
    echo "❌ 应用未运行，请先启动应用：make run 或 make dev"
    exit 1
fi

echo "✅ 应用正在运行"
echo ""

# 测试 metrics 端点
echo "测试 /metrics 端点..."
response=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/metrics)

if [ "$response" = "200" ]; then
    echo "✅ /metrics 端点响应正常 (HTTP $response)"
    echo ""
    echo "=== 指标内容示例 ==="
    curl -s http://localhost:8080/metrics | head -20
    echo "..."
    echo ""
    echo "✅ 指标端点已正常工作！"
else
    echo "❌ /metrics 端点返回错误 (HTTP $response)"
    echo ""
    echo "请检查以下问题："
    echo "1. 路由配置中是否包含 RegisterMetricsEndpoint() 调用"
    echo "2. observability 模块是否正确导入"
    echo "3. MetricsMiddleware 是否正确添加到中间件"
    exit 1
fi

echo ""
echo "=== 测试健康检查端点 ==="
curl -s http://localhost:8080/api/v1/health | python3 -m json.tool 2>/dev/null || curl -s http://localhost:8080/api/v1/health

echo ""
echo "=== 所有测试完成 ==="
