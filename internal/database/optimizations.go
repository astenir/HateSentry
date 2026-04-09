package database

import (
	"fmt"
	"hatesentry/internal/errors"
	"hatesentry/internal/models"
	"time"
)

// OptimizeDatabase creates indexes and optimizes database queries
func OptimizeDatabase() error {
	if DB == nil {
		return errors.ConfigurationError("database not initialized")
	}

	// 创建复合索引以优化常用查询
	indexes := []string{
		// DetectionRequest 索引
		"CREATE INDEX idx_detection_request_user_created ON detection_requests(user_id, created_at DESC)",
		"CREATE INDEX idx_detection_request_status ON detection_requests(status)",
		"CREATE INDEX idx_detection_request_processed ON detection_requests(processed, created_at DESC)",
		
		// DetectionResult 索引
		"CREATE INDEX idx_detection_result_request ON detection_results(request_id)",
		"CREATE INDEX idx_detection_result_hate_speech ON detection_results(is_hate_speech, created_at DESC)",
		
		// DetectionStats 索引
		"CREATE INDEX idx_detection_stats_user_date ON detection_stats(user_id, date DESC)",
		
		// AuditLog 索引
		"CREATE INDEX idx_audit_log_user_created ON audit_logs(user_id, created_at DESC)",
		"CREATE INDEX idx_audit_log_action ON audit_logs(action, created_at DESC)",
	}

	for _, idx := range indexes {
		if err := DB.Exec(idx).Error; err != nil {
			// 索引可能已存在，忽略错误
			continue
		}
	}

	// 优化表结构
	optimizations := []string{
		// 添加检测结果的分区（按月分区以提升大数据量下的查询性能）
		"ALTER TABLE detection_results PARTITION BY RANGE (YEAR(created_at) * 100 + MONTH(created_at))",
		
		// 优化文本字段的存储
		"ALTER TABLE detection_requests MODIFY COLUMN content MEDIUMTEXT",
		"ALTER TABLE detection_results MODIFY COLUMN explanation MEDIUMTEXT",
	}

	// 分区创建仅在第一次执行
	partitionCheck := "SELECT COUNT(*) FROM information_schema.partitions WHERE table_name = 'detection_results' AND partition_name IS NOT NULL"
	var count int64
	if DB.Raw(partitionCheck).Scan(&count).Error == nil && count == 0 {
		for _, opt := range optimizations {
			DB.Exec(opt)
		}
	}

	return nil
}

// CreateMonthlyPartitions creates monthly partitions for current and next year
func CreateMonthlyPartitions() error {
	if DB == nil {
		return errors.ConfigurationError("database not initialized")
	}

	currentYear := time.Now().Year()
	nextYear := currentYear + 1

	years := []int{currentYear, nextYear}

	for _, year := range years {
		for month := 1; month <= 12; month++ {
			partitionName := fmt.Sprintf("p%d%02d", year, month)
			value := year*100 + month
			
			partitionSQL := fmt.Sprintf(
				"ALTER TABLE detection_results ADD PARTITION (PARTITION %s VALUES LESS THAN (%d))",
				partitionName,
				value,
			)
			
			// 如果分区已存在则跳过
			if DB.Exec(partitionSQL).Error != nil {
				continue
			}
		}
	}

	return nil
}

// CleanupOldData removes old data to prevent database bloat
func CleanupOldData(retentionMonths int) error {
	if DB == nil {
		return errors.ConfigurationError("database not initialized")
	}

	cutoffDate := time.Now().AddDate(0, -retentionMonths, 0)

	// 删除旧的检测请求（软删除）
	result := DB.Where("created_at < ?", cutoffDate).
		Delete(&models.DetectionRequest{})
	
	if result.Error != nil {
		return errors.DatabaseError(result.Error, "failed to cleanup old data")
	}

	// 删除旧的审计日志
	DB.Where("created_at < ?", cutoffDate).
		Delete(&models.AuditLog{})

	return nil
}

// AnalyzeSlowQueries analyzes and reports slow queries
func AnalyzeSlowQueries(threshold float64) ([]SlowQuery, error) {
	if DB == nil {
		return nil, errors.ConfigurationError("database not initialized")
	}

	var queries []SlowQuery

	// MySQL 慢查询分析
	sql := `
		SELECT 
			DIGEST_TEXT as query,
			COUNT_STAR as exec_count,
			AVG_TIMER_WAIT/1000000000000 as avg_time_sec,
			SUM_ROWS_EXAMINED as rows_examined,
			SUM_ROWS_SENT as rows_sent
		FROM performance_schema.events_statements_summary_by_digest
		WHERE SCHEMA_NAME = DATABASE()
		AND AVG_TIMER_WAIT/1000000000000 > ?
		ORDER BY AVG_TIMER_WAIT DESC
		LIMIT 20
	`

	err := DB.Raw(sql, threshold).Scan(&queries).Error
	if err != nil {
		return nil, errors.DatabaseError(err, "failed to analyze slow queries")
	}
	return queries, nil
}

// SlowQuery represents a slow query record
type SlowQuery struct {
	Query         string  `json:"query"`
	ExecCount     int64   `json:"exec_count"`
	AvgTimeSec    float64 `json:"avg_time_sec"`
	RowsExamined  int64   `json:"rows_examined"`
	RowsSent      int64   `json:"rows_sent"`
}

// GetQueryStats returns database query statistics
func GetQueryStats() (*QueryStats, error) {
	if DB == nil {
		return nil, errors.ConfigurationError("database not initialized")
	}

	stats := &QueryStats{}

	// 获取连接池状态
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, errors.DatabaseError(err, "failed to get database instance")
	}

	stats.MaxOpenConnections = sqlDB.Stats().MaxOpenConnections
	stats.OpenConnections = sqlDB.Stats().OpenConnections
	stats.InUse = sqlDB.Stats().InUse
	stats.Idle = sqlDB.Stats().Idle

	// 获取表大小和行数
	var tableStats []TableStat
	DB.Raw(`
		SELECT 
			TABLE_NAME,
			TABLE_ROWS,
			ROUND((DATA_LENGTH + INDEX_LENGTH) / 1024 / 1024, 2) as size_mb
		FROM information_schema.TABLES 
		WHERE TABLE_SCHEMA = DATABASE()
		ORDER BY (DATA_LENGTH + INDEX_LENGTH) DESC
	`).Scan(&tableStats)

	stats.TableStats = tableStats

	return stats, nil
}

// QueryStats represents database query statistics
type QueryStats struct {
	MaxOpenConnections int         `json:"max_open_connections"`
	OpenConnections   int         `json:"open_connections"`
	InUse            int         `json:"in_use"`
	Idle             int         `json:"idle"`
	TableStats       []TableStat `json:"table_stats"`
}

// TableStat represents table statistics
type TableStat struct {
	TableName string  `json:"table_name"`
	TableRows int64   `json:"table_rows"`
	SizeMB    float64 `json:"size_mb"`
}
