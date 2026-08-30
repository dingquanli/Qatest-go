package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewID 生成唯一 ID。
// 采用纳秒时间戳 + 16 字节密码学随机数，实际碰撞概率可忽略。
// （P3 修复记录：旧实现为秒级时间戳 + 4 hex，高并发同前缀同秒存在理论碰撞。）
func NewID(prefix string) string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), hex.EncodeToString(b[:]))
}
