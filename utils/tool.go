package utils

import (
	"math/rand"
	"strings"
	"time"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func base62Encode(n uint64, length int) string {
	var result strings.Builder
	for i := 0; i < length; i++ {
		result.WriteByte(base62Chars[n%62])
		n /= 62
	}
	return result.String()
}

func generateMachineCode() string {
	// 使用时间戳 + 随机数
	rand.Seed(time.Now().UnixNano())
	randomNum := rand.Uint64() % 916132832 // 防止溢出
	code := base62Encode(randomNum, 16)
	return code
}
