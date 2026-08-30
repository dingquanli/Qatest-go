package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 缺陷管理（SQL 迁自 handlers/bugs.go，语句原样保留） ——

// ListBugs 缺陷列表
func ListBugs() ([]models.Bug, error) {
	rows, err := database.DB.Query(
		`SELECT id, title, severity, priority, status, assignee, reporter, module, env,
		 description, steps, expected, actual, tags, related_case_id, external_id, external_url, created_at, updated_at
		 FROM bugs ORDER BY updated_at DESC LIMIT 200`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bugs := make([]models.Bug, 0)
	for rows.Next() {
		var b models.Bug
		if err := rows.Scan(&b.ID, &b.Title, &b.Severity, &b.Priority, &b.Status, &b.Assignee, &b.Reporter,
			&b.Module, &b.Env, &b.Description, &b.Steps, &b.Expected, &b.Actual, &b.Tags,
			&b.RelatedCaseID, &b.ExternalID, &b.ExternalURL, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		bugs = append(bugs, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bugs, nil
}

// GetBugStats 缺陷统计（按 status 分组计数）
func GetBugStats() (map[string]int, error) {
	rows, err := database.DB.Query(
		`SELECT status, COUNT(*) as cnt FROM bugs GROUP BY status`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var cnt int
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, err
		}
		stats[status] = cnt
	}

	return stats, nil
}

// GetBug 缺陷详情（不存在时返回 sql.ErrNoRows）
func GetBug(id string) (models.Bug, error) {
	var b models.Bug
	err := database.DB.QueryRow(
		`SELECT id, title, severity, priority, status, assignee, reporter, module, env,
		 description, steps, expected, actual, tags, related_case_id, external_id, external_url, created_at, updated_at
		 FROM bugs WHERE id = ?`, id,
	).Scan(&b.ID, &b.Title, &b.Severity, &b.Priority, &b.Status, &b.Assignee, &b.Reporter,
		&b.Module, &b.Env, &b.Description, &b.Steps, &b.Expected, &b.Actual, &b.Tags,
		&b.RelatedCaseID, &b.ExternalID, &b.ExternalURL, &b.CreatedAt, &b.UpdatedAt)
	return b, err
}

// CreateBug 插入缺陷（ID/时间戳由调用方填充）
func CreateBug(b models.Bug) error {
	_, err := database.DB.Exec(
		`INSERT INTO bugs (id, title, severity, priority, status, assignee, reporter, module, env,
		 description, steps, expected, actual, tags, related_case_id, external_id, external_url, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Title, b.Severity, b.Priority, b.Status, b.Assignee, b.Reporter,
		b.Module, b.Env, b.Description, b.Steps, b.Expected, b.Actual, b.Tags,
		b.RelatedCaseID, b.ExternalID, b.ExternalURL, b.CreatedAt, b.UpdatedAt,
	)
	return err
}

// UpdateBug 更新缺陷（不含 created_at，Jira 同步字段 external_id/external_url 一并更新）
func UpdateBug(id string, b models.Bug) error {
	_, err := database.DB.Exec(
		`UPDATE bugs SET title=?, severity=?, priority=?, status=?, assignee=?, reporter=?, module=?, env=?,
		 description=?, steps=?, expected=?, actual=?, tags=?, related_case_id=?, external_id=?, external_url=?, updated_at=?
		 WHERE id=?`,
		b.Title, b.Severity, b.Priority, b.Status, b.Assignee, b.Reporter,
		b.Module, b.Env, b.Description, b.Steps, b.Expected, b.Actual, b.Tags,
		b.RelatedCaseID, b.ExternalID, b.ExternalURL, b.UpdatedAt, id,
	)
	return err
}

// DeleteBug 删除缺陷
func DeleteBug(id string) error {
	_, err := database.DB.Exec("DELETE FROM bugs WHERE id = ?", id)
	return err
}

// GetBugForSync 读取缺陷用于 Jira 同步报文（列集与 GetBug 不同，SQL 迁自 handlers/bugs.go SyncBugToJira，原样保留）
func GetBugForSync(id string) (models.Bug, error) {
	var b models.Bug
	err := database.DB.QueryRow(
		`SELECT id,title,severity,priority,status,assignee,reporter,module,env,description,steps,expected,actual,tags
		 FROM bugs WHERE id=?`, id,
	).Scan(&b.ID, &b.Title, &b.Severity, &b.Priority, &b.Status, &b.Assignee, &b.Reporter,
		&b.Module, &b.Env, &b.Description, &b.Steps, &b.Expected, &b.Actual, &b.Tags)
	return b, err
}

// UpdateBugExternal 回写 Jira 同步结果（迁自 handlers/bugs.go SyncBugToJira）
func UpdateBugExternal(id, externalID, externalURL, updatedAt string) error {
	_, err := database.DB.Exec(
		"UPDATE bugs SET external_id=?, external_url=?, updated_at=? WHERE id=?",
		externalID, externalURL, updatedAt, id,
	)
	return err
}
