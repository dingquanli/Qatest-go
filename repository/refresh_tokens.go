package repository

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"qatest/database"
	"qatest/models"
)

// —— 刷新令牌存储（表 refresh_tokens；SQL 迁自 handlers/auth.go，语句原样保留）——
//
// 轮换式刷新令牌：库中只存 SHA-256 哈希，明文令牌泄露无法反查；
// 旧令牌轮换后立即作废，重放被拒绝。

// RefreshTTL 刷新令牌有效期；每次轮换重新起算
const RefreshTTL = 7 * 24 * time.Hour

// NewSecureToken 生成十六进制安全随机串（n 字节熵）
func NewSecureToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashRefreshToken 刷新令牌的存储哈希（SHA-256；令牌本身为 32 字节 CSPRNG 输出，
// 无需 bcrypt 级别的慢哈希，重点是库泄露后不可反推明文）。
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IssueRefreshToken 签发新刷新令牌并持久化哈希，返回明文令牌（仅此一次可见）。
func IssueRefreshToken(userID string) (string, error) {
	token := NewSecureToken(32)
	_, err := database.DB.Exec(
		"INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at) VALUES (?,?,?,?,0,?)",
		NewID("rt"), userID, HashRefreshToken(token),
		time.Now().Add(RefreshTTL).Format(time.RFC3339), models.NowStr(),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

// RotateRefreshToken 校验并轮换：有效则将旧令牌标记作废，返回其归属用户。
// 重放已作废令牌会在此被拒绝（refresh token rotation 的核心防线）。
func RotateRefreshToken(token string) (string, error) {
	if token == "" {
		return "", sql.ErrNoRows
	}
	var id, userID, expiresAt string
	var revoked int
	err := database.DB.QueryRow(
		"SELECT id, user_id, expires_at, revoked FROM refresh_tokens WHERE token_hash = ?",
		HashRefreshToken(token),
	).Scan(&id, &userID, &expiresAt, &revoked)
	if err != nil {
		return "", err
	}
	if revoked == 1 {
		return "", sql.ErrNoRows
	}
	if exp, parseErr := time.Parse(time.RFC3339, expiresAt); parseErr != nil || time.Now().After(exp) {
		return "", sql.ErrNoRows
	}
	_, err = database.DB.Exec("UPDATE refresh_tokens SET revoked = 1 WHERE id = ?", id)
	if err != nil {
		return "", err
	}
	return userID, nil
}

// RevokeRefreshToken 按明文令牌撤销（登出用）；未命中即视为已撤销，幂等返回。
func RevokeRefreshToken(token string) error {
	if token == "" {
		return nil
	}
	_, err := database.DB.Exec("UPDATE refresh_tokens SET revoked = 1 WHERE token_hash = ?", HashRefreshToken(token))
	return err
}
