package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
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
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register 用户注册
func (c *AuthController) Register(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
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

	// 加密密码
	// hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	// if err != nil {
	// 	ctx.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
	// 	return
	// }

	// 插入用户记录
	result, err := c.DB.Exec(
		"INSERT INTO users (username, password, created_at) VALUES (?, ?, ?)",
		req.Username, req.Username, time.Now().Format("2006-01-02 15:04:05"),
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
			"password":   req.Password,
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
	var user User
	var hashedPassword string
	err := c.DB.QueryRow(
		"SELECT id, username, password FROM users WHERE username = ?",
		req.Username,
	).Scan(&user.ID, &user.Username, &hashedPassword)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "用户名或密码错误"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err})
		}
		return
	}

	// 验证密码
	// err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	// if err != nil {
	// 	ctx.JSON(http.StatusBadRequest, gin.H{"error": "用户名或密码错误"})
	// 	return
	// }
	if req.Password != hashedPassword {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "用户名或密码错误"})
		return
	}
	fmt.Println(user.ID)
	// 生成JWT令牌
	token, err := generateToken(user.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"token":      token,
			"username":   req.Username,
			"password":   req.Password,
			"customerId": user.ID,
		},
	})
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
	tokenString, err := token.SignedString([]byte("mengtuobang_secret_key")) // 在实际应用中，应该使用环境变量存储密钥

	return tokenString, err
}
