package controllers

import (
	"database/sql"
	"net/http"
	"time"

	"go-mengtuobang/models"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid"
)

type MachineController struct {
	DB *sql.DB
}

// 检查机器码请求
type CheckMachineCodeRequest struct {
	MachineCode string `json:"machineCode" binding:"required,len=16"`
}

// 绑定机器码请求
type BindMachineCodeRequest struct {
	MachineCode string `json:"machineCode" binding:"required,len=16"`
	UserID      int    `json:"userId" binding:"required"`
}

// 生成机器码
func (c *MachineController) CreateMachineCode(ctx *gin.Context) {
	userID := ctx.GetInt("userID")

	// 检查用户是否为管理员
	var role int
	err := c.DB.QueryRow("SELECT role FROM users WHERE id = ?", userID).Scan(&role)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "查询用户信息失败"})
		return
	}

	if role != models.RoleAdmin {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "权限不足"})
		return
	}

	var req CreateMachineCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 生成机器码
	customAlphabet := "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	code, err := gonanoid.Generate(customAlphabet, 16)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "生成机器码失败"})
		return
	}
	// 插入机器码记录
	result, err := c.DB.Exec(
		"INSERT INTO machine_codes (code, name, description, created_by, created_at) VALUES (?, ?, ?, ?, ?)",
		code, req.Name, req.Description, userID, time.Now(),
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "创建机器码失败"})
		return
	}

	machineCodeID, _ := result.LastInsertId()

	ctx.JSON(http.StatusCreated, gin.H{
		"code":    200,
		"message": "机器码创建成功",
		"data": gin.H{
			"id":          machineCodeID,
			"code":        code,
			"name":        req.Name,
			"description": req.Description,
		},
	})
}

// 检查机器码是否有效
func (mc *MachineController) CheckMachineCode(c *gin.Context) {
	var req CheckMachineCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的请求参数",
		})
		return
	}

	// 使用原生SQL查询
	query := `SELECT id, code, user_id, is_active FROM machine_codes WHERE code = ?`
	var machineCode models.MachineCode
	err := mc.DB.QueryRow(query, req.MachineCode).Scan(&machineCode.ID, &machineCode.Code, &machineCode.UserID, &machineCode.IsActive)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  "机器码不存在",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	if !machineCode.IsActive {
		c.JSON(http.StatusOK, gin.H{
			"code": 403,
			"msg":  "机器码已被禁用",
		})
		return
	}

	if machineCode.UserID != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 403,
			"msg":  "机器码已被绑定",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "OK",
		"data": gin.H{
			"isValid": true,
		},
	})
}

// 绑定机器码
func (mc *MachineController) BindMachineCode(c *gin.Context) {
	var req BindMachineCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  err.Error(),
		})
		return
	}

	// 查询机器码
	query := `SELECT id, code, user_id, is_active FROM machine_codes WHERE code = ?`
	var machineCode models.MachineCode
	err := mc.DB.QueryRow(query, req.MachineCode).Scan(&machineCode.ID, &machineCode.Code, &machineCode.UserID, &machineCode.IsActive)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  "机器码不存在",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	if !machineCode.IsActive {
		c.JSON(http.StatusOK, gin.H{
			"code": 403,
			"msg":  "机器码已被禁用",
		})
		return
	}

	if machineCode.UserID != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 403,
			"msg":  "机器码已被绑定",
		})
		return
	}

	// 检查用户是否已绑定其他机器码
	var existingCodeID int
	query = `SELECT id FROM machine_codes WHERE user_id = ?`
	err = mc.DB.QueryRow(query, req.UserID).Scan(&existingCodeID)
	if err == nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 403,
			"msg":  "用户已绑定其他机器码",
		})
		return
	} else if err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  err.Error(),
		})
		return
	}

	// 更新机器码绑定信息
	now := time.Now()
	query = `UPDATE machine_codes SET user_id = ?, updated_at = ?, binded_at = ? WHERE id = ?`
	_, err = mc.DB.Exec(query, req.UserID, now, now, machineCode.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "绑定机器码失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "绑定成功",
		"data": gin.H{
			"machineCode": machineCode.Code,
			"userId":      req.UserID,
			"bindedAt":    now,
		},
	})
}

// 获取用户绑定的机器码
func (mc *MachineController) GetUserMachineCode(c *gin.Context) {
	userID := c.Param("userId")

	// 查询用户绑定的机器码
	query := `SELECT code, binded_at FROM machine_codes WHERE user_id = ?`
	var code string
	var bindedAt time.Time
	err := mc.DB.QueryRow(query, userID).Scan(&code, &bindedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  "用户未绑定机器码",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "OK",
		"data": gin.H{
			"machineCode": code,
			"bindedAt":    bindedAt,
		},
	})
}

// 解绑机器码
func (mc *MachineController) UnbindMachineCode(c *gin.Context) {
	userID := c.Param("userId")

	// 查询用户绑定的机器码
	query := `SELECT id FROM machine_codes WHERE user_id = ?`
	var id int
	err := mc.DB.QueryRow(query, userID).Scan(&id)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusOK, gin.H{
				"code": 404,
				"msg":  "用户未绑定机器码",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code": 500,
				"msg":  err.Error(),
			})
		}
		return
	}

	// 更新机器码信息
	now := time.Now()
	query = `UPDATE machine_codes SET user_id = '', updated_at = ? WHERE id = ?`
	_, err = mc.DB.Exec(query, now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "解绑机器码失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "解绑成功",
	})
}
