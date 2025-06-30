package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"go-mengtuobang/models"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

// AuthController 处理用户认证相关的请求
type AuthController struct {
	DB *sql.DB
}

// NewAuthController 创建一个新的AuthController实例
func NewAuthController(db *sql.DB) *AuthController {
	return &AuthController{DB: db}
}

// User 用户模型
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Phone    string `json:"phone"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	MachineCode string `json:"machine_code"`
}

// WechatLoginRequest 微信登录请求
type WechatLoginRequest struct {
	Code         string  `json:"code" binding:"required"` // 微信授权码
	Nickname     string  `json:"nickname"`
	AvatarBase64 string  `json:"avatar_base64"`
	MachineCode  *string `json:"machine_code"`
}

// PhoneBindRequest 绑定手机号请求
type PhoneBindRequest struct {
	Phone      string `json:"phone" binding:"required"`
	VerifyCode string `json:"verify_code" binding:"required"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Phone       string `json:"phone" binding:"required"`
	VerifyCode  string `json:"verify_code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// SendSMSRequest 发送短信验证码请求
type SendSMSRequest struct {
	Phone string `json:"phone" binding:"required"`
	Type  string `json:"type" binding:"required"` // bind, reset
}

// CreateMachineCodeRequest 创建机器码请求
type CreateMachineCodeRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// WechatUserInfo 微信用户信息
type WechatUserInfo struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	Nickname   string `json:"nickname"`
	HeadImgURL string `json:"headimgurl"`
}

// WechatAccessTokenResponse 微信访问令牌响应
type WechatAccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	OpenID       string `json:"openid"`
	Scope        string `json:"scope"`
	UnionID      string `json:"unionid"`
}

const (
	// 微信配置 - 实际使用时应该从环境变量读取
	WECHAT_APP_ID     = "wxa2a9b21a2361d5b8"
	WECHAT_APP_SECRET = "f9c33eb028d0297e29193d7a2bab73c7"
	JWT_SECRET_KEY    = "mengtuobang_secret_key" // 应该使用环境变量
)

// Register 用户注册
func (c *AuthController) Register(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证手机号格式（如果提供）
	if req.Phone != "" {
		if !isValidPhone(req.Phone) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号格式不正确"})
			return
		}

		// 检查手机号是否已被使用
		var count int
		err := c.DB.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ?", req.Phone).Scan(&count)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
			return
		}
		if count > 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号已被使用"})
			return
		}
	}

	// 检查用户名是否已存在
	var count int
	err := c.DB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", req.Username).Scan(&count)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}

	if count > 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在"})
		return
	}

	// 插入用户记录
	result, err := c.DB.Exec(
		"INSERT INTO users (username, password, phone, created_at) VALUES (?, ?, ?, ?)",
		req.Username, req.Password,
		func() interface{} {
			if req.Phone != "" {
				return req.Phone
			}
			return nil
		}(),
		time.Now().Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	userID, err := result.LastInsertId()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户ID失败"})
		return
	}

	// 生成JWT令牌
	token, err := generateToken(int(userID))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"code":    200,
		"message": "注册成功",
		"data": gin.H{
			"token":      token,
			"username":   req.Username,
			"customerId": userID,
		},
	})
}

// Login 用户登录
func (c *AuthController) Login(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询用户
	var user models.User
	var hashedPassword string
	err := c.DB.QueryRow(
		"SELECT id, username, password, nickname, phone, role FROM users WHERE username = ? AND status = 'active'",
		req.Username,
	).Scan(&user.ID, &user.Username, &hashedPassword, &user.Nickname, &user.Phone, &user.Role)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "用户名或密码错误"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err})
		}
		return
	}

	// 验证密码
	if req.Password != hashedPassword {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "用户名或密码错误"})
		return
	}

	// 更新最后登录时间
	_, err = c.DB.Exec("UPDATE users SET last_login_at = ? WHERE id = ?", time.Now(), user.ID)
	if err != nil {
		fmt.Printf("更新最后登录时间失败: %v\n", err)
	}

	// 生成JWT令牌
	token, err := generateToken(user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	nickname := ""
	if user.Nickname != nil {
		nickname = *user.Nickname
	}

	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"token":      token,
			"username":   *user.Username,
			"nickname":   nickname,
			"phone":      phone,
			"role":       user.Role,
			"customerId": user.ID,
		},
	})
}

// WechatLogin 微信登录
func (c *AuthController) WechatLogin(ctx *gin.Context) {
	var req WechatLoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 通过code获取access_token
	accessTokenResp, err := c.getWechatAccessToken(req.Code)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查找或创建用户
	user, isNewUser, err := c.findOrCreateWechatUser(accessTokenResp.OpenID, accessTokenResp.UnionID, req.Nickname, req.AvatarBase64, req.MachineCode)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新最后登录时间
	_, err = c.DB.Exec("UPDATE users SET last_login_at = ? WHERE id = ?", time.Now(), user.ID)
	if err != nil {
		fmt.Printf("更新最后登录时间失败: %v\n", err)
	}

	// 生成JWT令牌
	token, err := generateToken(user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	message := "登录成功"
	if isNewUser {
		message = "注册并登录成功"
	}

	nickname := ""
	if user.Nickname != nil {
		nickname = *user.Nickname
	}

	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": message,
		"data": gin.H{
			"token":      token,
			"nickname":   nickname,
			"phone":      phone,
			"role":       user.Role,
			"customerId": user.ID,
			"isNewUser":  isNewUser,
		},
	})
}

// BindPhone 绑定手机号
func (c *AuthController) BindPhone(ctx *gin.Context) {
	userID := ctx.GetInt("userID")

	var req PhoneBindRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证手机号格式
	if !isValidPhone(req.Phone) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号格式不正确"})
		return
	}

	// 验证短信验证码（这里简化处理，实际应该验证验证码）
	if !c.verifyCode(req.Phone, req.VerifyCode, "bind") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误或已过期"})
		return
	}

	// 检查手机号是否已被其他用户使用
	var count int
	err := c.DB.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ? AND id != ?", req.Phone, userID).Scan(&count)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		return
	}
	if count > 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号已被其他用户使用"})
		return
	}

	// 更新用户手机号
	_, err = c.DB.Exec("UPDATE users SET phone = ?, updated_at = ? WHERE id = ?", req.Phone, time.Now(), userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "绑定手机号失败"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "手机号绑定成功",
	})
}

// SendSMS 发送短信验证码
func (c *AuthController) SendSMS(ctx *gin.Context) {
	var req SendSMSRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证手机号格式
	if !isValidPhone(req.Phone) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号格式不正确"})
		return
	}

	// 生成验证码
	code := generateVerifyCode()

	// 这里应该调用短信服务发送验证码，简化处理直接返回验证码
	// 实际生产环境中应该调用短信API
	fmt.Printf("短信验证码: %s (手机号: %s, 类型: %s)\n", code, req.Phone, req.Type)

	// 存储验证码（实际应该存储到Redis或数据库，这里简化处理）
	c.storeVerifyCode(req.Phone, code, req.Type)

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "验证码发送成功",
		"data": gin.H{
			"verify_code": code, // 仅用于测试，生产环境不应返回
		},
	})
}

// ResetPassword 重置密码
func (c *AuthController) ResetPassword(ctx *gin.Context) {
	var req ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证手机号格式
	if !isValidPhone(req.Phone) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号格式不正确"})
		return
	}

	// 验证短信验证码
	if !c.verifyCode(req.Phone, req.VerifyCode, "reset") {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "验证码错误或已过期"})
		return
	}

	// 查找用户
	var userID int
	err := c.DB.QueryRow("SELECT id FROM users WHERE phone = ? AND status = 'active'", req.Phone).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "手机号未注册"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "数据库查询失败"})
		}
		return
	}

	// 更新密码
	_, err = c.DB.Exec("UPDATE users SET password = ?, updated_at = ? WHERE id = ?", req.NewPassword, time.Now(), userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "密码重置失败"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "密码重置成功",
	})
}

// CreateMachineCode 创建机器码（管理员功能）

// GetUserInfo 获取用户信息
func (c *AuthController) GetUserInfo(ctx *gin.Context) {
	userID := ctx.GetInt("userID")

	var user models.User
	err := c.DB.QueryRow(`
		SELECT id, username, nickname, phone, avatar_url, avatar_base64, 
		       machine_code, role, status, created_at, last_login_at
		FROM users WHERE id = ?
	`, userID).Scan(
		&user.ID, &user.Username, &user.Nickname, &user.Phone,
		&user.AvatarURL, &user.AvatarBase64, &user.MachineCode,
		&user.Role, &user.Status, &user.CreatedAt, &user.LastLoginAt,
	)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败"})
		return
	}

	nickname := ""
	if user.Nickname != nil {
		nickname = *user.Nickname
	}

	username := ""
	if user.Username != nil {
		username = *user.Username
	}

	phone := ""
	if user.Phone != nil {
		phone = *user.Phone
	}

	avatarURL := ""
	if user.AvatarURL != nil {
		avatarURL = *user.AvatarURL
	}

	avatarBase64 := ""
	if user.AvatarBase64 != nil {
		avatarBase64 = *user.AvatarBase64
	}

	machineCode := ""
	if user.MachineCode != nil {
		machineCode = *user.MachineCode
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "获取成功",
		"data": gin.H{
			"id":            user.ID,
			"username":      username,
			"nickname":      nickname,
			"phone":         phone,
			"avatar_url":    avatarURL,
			"avatar_base64": avatarBase64,
			"machine_code":  machineCode,
			"role":          user.Role,
			"is_admin":      user.Role == models.RoleAdmin,
			"created_at":    user.CreatedAt,
			"last_login_at": user.LastLoginAt,
		},
	})
}

// getWechatAccessToken 获取微信访问令牌
func (c *AuthController) getWechatAccessToken(code string) (*WechatAccessTokenResponse, error) {
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		WECHAT_APP_ID, WECHAT_APP_SECRET, code,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var sessionResp WechatAccessTokenResponse
	if err := json.Unmarshal(body, &sessionResp); err != nil {
		return nil, err
	}

	if sessionResp.OpenID == "" {
		return nil, fmt.Errorf("获取 session_key 失败")
	}

	return &sessionResp, nil
}

// getWechatAccessToken 获取微信访问令牌
// findOrCreateWechatUser 查找或创建微信用户
func (c *AuthController) findOrCreateWechatUser(openID, unionID, nickname, avatarBase64 string, machineCode *string) (*models.User, bool, error) {
	var user models.User
	var isNewUser bool

	// 首先尝试通过UnionID查找用户（如果有的话）
	var query string
	var args []interface{}

	if unionID != "" {
		query = "SELECT id, wechat_openid, wechat_unionid, nickname, avatar_url, avatar_base64, machine_code, phone, role, status FROM users WHERE wechat_unionid = ?"
		args = []interface{}{unionID}
	} else {
		query = "SELECT id, wechat_openid, wechat_unionid, nickname, avatar_url, avatar_base64, machine_code, phone, role, status FROM users WHERE wechat_openid = ?"
		args = []interface{}{openID}
	}

	err := c.DB.QueryRow(query, args...).Scan(
		&user.ID, &user.WechatOpenID, &user.WechatUnionID,
		&user.Nickname, &user.AvatarURL, &user.AvatarBase64, &user.MachineCode,
		&user.Phone, &user.Role, &user.Status,
	)

	if err == sql.ErrNoRows {
		// 用户不存在，创建新用户
		isNewUser = true
		now := time.Now()
		status := "active"
		loginMethod := "wechat"

		result, err := c.DB.Exec(`
			INSERT INTO users (wechat_openid, wechat_unionid, nickname, avatar_base64, machine_code, 
			                  login_method, role, status, created_at, updated_at, last_login_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			openID,
			func() *string {
				if unionID != "" {
					return &unionID
				} else {
					return nil
				}
			}(),
			nickname, avatarBase64, machineCode,
			loginMethod, 0, status, now, now, now,
		)
		if err != nil {
			return nil, false, err
		}

		userID, err := result.LastInsertId()
		if err != nil {
			return nil, false, err
		}

		user.ID = int(userID)
		user.WechatOpenID = &openID
		if unionID != "" {
			user.WechatUnionID = &unionID
		}
		user.Nickname = &nickname
		user.AvatarBase64 = &avatarBase64
		user.MachineCode = machineCode
		user.Role = 0
		user.Status = &status

	} else if err != nil {
		return nil, false, err
	} else {
		// 用户存在，检查状态
		if user.Status != nil && *user.Status != "active" {
			return nil, false, fmt.Errorf("账户已被禁用")
		}

		// 更新用户信息（如果有变化）
		_, err = c.DB.Exec(`
			UPDATE users SET nickname = ?, avatar_base64 = ?, updated_at = ? 
			WHERE id = ?`,
			nickname, avatarBase64, time.Now(), user.ID,
		)
		if err != nil {
			fmt.Printf("更新用户信息失败: %v\n", err)
		}
	}

	return &user, isNewUser, nil
}

// 工具函数

// isValidPhone 验证手机号格式
func isValidPhone(phone string) bool {
	pattern := `^1[3-9]\d{9}$`
	matched, _ := regexp.MatchString(pattern, phone)
	return matched
}

// generateVerifyCode 生成6位数字验证码
func generateVerifyCode() string {
	return fmt.Sprintf("%06d", time.Now().Unix()%1000000)
}

// storeVerifyCode 存储验证码（简化实现，实际应使用Redis）
func (c *AuthController) storeVerifyCode(phone, code, codeType string) {
	// 这里应该存储到Redis，设置5分钟过期时间
	// 简化处理，实际项目中需要实现
	fmt.Printf("存储验证码: 手机号=%s, 验证码=%s, 类型=%s\n", phone, code, codeType)
}

// verifyCode 验证验证码（简化实现）
func (c *AuthController) verifyCode(phone, code, codeType string) bool {
	// 这里应该从Redis验证，简化处理总是返回true
	// 实际项目中需要实现真实的验证逻辑
	return true
}

// 生成JWT令牌
func generateToken(userID int) (string, error) {
	// 设置JWT声明
	claims := jwt.MapClaims{
		"userID": userID,
		"exp":    time.Now().Add(time.Hour * 24 * 7).Unix(), // 令牌有效期7天
	}

	// 创建令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 使用密钥签名令牌并获取完整的编码令牌作为字符串
	tokenString, err := token.SignedString([]byte(JWT_SECRET_KEY))

	return tokenString, err
}
