package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// base62Encode 将数字编码为base62字符串
func base62Encode(n uint64, length int) string {
	if n == 0 {
		return strings.Repeat("0", length)
	}

	var result []byte
	for n > 0 && len(result) < length {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}

	// 如果结果长度不足，在前面补0
	for len(result) < length {
		result = append([]byte{'0'}, result...)
	}

	return string(result)
}

// getMachineFingerprint 获取机器指纹
func getMachineFingerprint() uint64 {
	var fingerprint uint64 = 0

	// 1. 获取主机名
	if hostname, err := os.Hostname(); err == nil {
		for _, b := range []byte(hostname) {
			fingerprint = fingerprint*31 + uint64(b)
		}
	}

	// 2. 获取MAC地址
	if interfaces, err := net.Interfaces(); err == nil {
		for _, iface := range interfaces {
			if len(iface.HardwareAddr) > 0 {
				for _, b := range iface.HardwareAddr {
					fingerprint = fingerprint*31 + uint64(b)
				}
				break // 只取第一个有效的MAC地址
			}
		}
	}

	return fingerprint
}

// getSecureRandom 获取加密安全的随机数
func getSecureRandom() uint64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// 如果加密随机数生成失败，回退到时间戳
		return uint64(time.Now().UnixNano())
	}
	return binary.BigEndian.Uint64(buf[:])
}

// GenerateMachineCode 生成16位机器码
func GenerateMachineCode() string {
	// 获取机器指纹
	machineFingerprint := getMachineFingerprint()

	// 获取当前时间戳（毫秒级）
	timestamp := uint64(time.Now().UnixMilli())

	// 获取安全随机数
	randomNum := getSecureRandom()

	// 组合生成唯一标识
	// 使用位运算混合三个组件
	combined := machineFingerprint + timestamp + randomNum

	// 生成16位base62编码
	code := base62Encode(combined, 16)

	return code
}

// GenerateMachineCodeWithPrefix 生成带前缀的机器码
func GenerateMachineCodeWithPrefix(prefix string) string {
	code := GenerateMachineCode()
	return fmt.Sprintf("%s-%s", prefix, code)
}

// ValidateMachineCode 验证机器码格式
func ValidateMachineCode(code string) bool {
	if len(code) != 16 {
		return false
	}

	for _, char := range code {
		if !strings.ContainsRune(base62Chars, char) {
			return false
		}
	}

	return true
}

// GenerateBatchMachineCodes 批量生成机器码（确保唯一性）
func GenerateBatchMachineCodes(count int) []string {
	codes := make([]string, 0, count)
	codeSet := make(map[string]bool)

	for len(codes) < count {
		code := GenerateMachineCode()
		if !codeSet[code] {
			codes = append(codes, code)
			codeSet[code] = true
		}
		// 如果生成重复，会自动重试
		time.Sleep(time.Microsecond) // 确保时间戳有所不同
	}

	return codes
}
