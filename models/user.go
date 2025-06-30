package models

import (
	"time"
)

type User struct {
	ID            int       `db:"id"`
	Username      *string   `db:"username"`
	Password      *string   `db:"password" json:"-"`
	Phone         *string   `db:"phone"`
	WechatOpenID  *string   `db:"wechat_openid"`
	WechatUnionID *string   `db:"wechat_unionid"`
	Nickname      *string   `db:"nickname"`
	AvatarURL     *string   `db:"avatar_url"`
	AvatarBase64  *string   `db:"avatar_base64"`
	MachineCode   *string   `db:"machine_code"`
	LastLoginAt   time.Time `db:"last_login_at"`
	LoginMethod   string    `db:"login_method"`
	Role          int       `db:"role"`
	Status        *string   `db:"status"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// 角色常量
const (
	RoleUser  = 0 // 普通用户
	RoleAdmin = 1 // 管理员
)

// IsAdmin 检查用户是否为管理员
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
