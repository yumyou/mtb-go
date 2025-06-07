package middleware

import (
	"fmt"
	"strings"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"

	"go-mengtuobang/utils"
)

// JWTSecret 用于签名JWT的密钥
const JWTSecret = "mengtuobang_secret_key"

// Claims 定义JWT的声明结构
type Claims struct {
	UserID int `json:"userID"`
	jwt.StandardClaims
}

// AuthMiddleware 验证JWT Token的中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authorization := c.GetHeader("Authorization")
		if authorization == "" {
			utils.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authorization, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			utils.Unauthorized(c, "Authorization header format must be Bearer {token}")
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(JWTSecret), nil
		})

		if err != nil || !token.Valid {
			utils.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}
		fmt.Println(claims.UserID)
		c.Set("userID", claims.UserID)
		c.Next()
	}
}
