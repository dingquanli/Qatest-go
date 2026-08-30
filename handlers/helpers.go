package handlers

import "qatest/repository"

// generateID 生成唯一 ID（实现已迁至 repository.NewID，保留旧名供包内调用）。
func generateID(prefix string) string {
	return repository.NewID(prefix)
}
