package models

import (
	"time"
)

// MachineCode 机器码模型
type MachineCode struct {
	ID          int        `db:"id" json:"id"`
	UserID      *int       `db:"user_id" json:"user_id"`
	BindedAt    *time.Time `db:"binded_at" json:"binded_at"`
	Code        string     `db:"code" json:"code"`
	Name        *string    `db:"name" json:"name"`
	Description *string    `db:"description" json:"description"`
	CreatedBy   *int       `db:"created_by" json:"created_by"`
	IsActive    bool       `db:"is_active" json:"is_active"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// CreateMachineCodeRequest 创建机器码请求
type CreateMachineCodeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateMachineCodeRequest 更新机器码请求
type UpdateMachineCodeRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

// TableName 设置表名
func (MachineCode) TableName() string {
	return "machine_codes"
}
