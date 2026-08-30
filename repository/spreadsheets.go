package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 自由电子表格（SQL 迁自 handlers/spreadsheet.go，语句原样保留） ——

// ListSpreadsheets 电子表格列表（按 created_at 排序）
func ListSpreadsheets() ([]models.Spreadsheet, error) {
	rows, err := database.DB.Query("SELECT id, name, cells, formats, col_widths, row_heights, merges, created_at, updated_at FROM spreadsheets ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]models.Spreadsheet, 0)
	for rows.Next() {
		var s models.Spreadsheet
		if err := rows.Scan(&s.ID, &s.Name, &s.Cells, &s.Formats, &s.ColWidths, &s.RowHeights, &s.Merges, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, nil
}

// GetSpreadsheet 电子表格详情（不存在时返回 sql.ErrNoRows）
func GetSpreadsheet(id string) (models.Spreadsheet, error) {
	var s models.Spreadsheet
	err := database.DB.QueryRow("SELECT id, name, cells, formats, col_widths, row_heights, merges, created_at, updated_at FROM spreadsheets WHERE id = ?", id).
		Scan(&s.ID, &s.Name, &s.Cells, &s.Formats, &s.ColWidths, &s.RowHeights, &s.Merges, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// CreateSpreadsheet 插入电子表格（ID/时间戳由调用方填充）
func CreateSpreadsheet(s models.Spreadsheet) error {
	_, err := database.DB.Exec("INSERT INTO spreadsheets (id, name, cells, formats, col_widths, row_heights, merges, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)",
		s.ID, s.Name, s.Cells, s.Formats, s.ColWidths, s.RowHeights, s.Merges, s.CreatedAt, s.UpdatedAt)
	return err
}

// UpdateSpreadsheet 更新电子表格
func UpdateSpreadsheet(id string, s models.Spreadsheet) error {
	_, err := database.DB.Exec("UPDATE spreadsheets SET name=?, cells=?, formats=?, col_widths=?, row_heights=?, merges=?, updated_at=? WHERE id=?",
		s.Name, s.Cells, s.Formats, s.ColWidths, s.RowHeights, s.Merges, s.UpdatedAt, id)
	return err
}

// DeleteSpreadsheet 删除电子表格
func DeleteSpreadsheet(id string) error {
	_, err := database.DB.Exec("DELETE FROM spreadsheets WHERE id = ?", id)
	return err
}
