package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-mengtuobang/models"
)

// CompostController 处理堆肥相关的请求
type CompostController struct {
	DB *sql.DB
}

// NewCompostController 创建一个新的CompostController实例
func NewCompostController(db *sql.DB) *CompostController {
	return &CompostController{DB: db}
}

// CreateCompostHistory 创建堆肥历史记录
func (c *CompostController) SaveCompostRecord(ctx *gin.Context) {
	userID := ctx.GetInt("userID")
	var history models.CompostHistory
	if err := ctx.ShouldBindJSON(&history); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开始事务
	tx, err := c.DB.Begin()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	insertHistorySQL := `
        INSERT INTO compost_history (all_volume, cn_ratio, density, water_add, user_id)
        VALUES (?,?,?,?,?)
    `
	result, err := tx.Exec(insertHistorySQL, history.AllVolume, history.CNRatio, history.Density, history.WaterAdd, userID)
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	history.ID = int(id)

	insertSourceSQL := `
        INSERT INTO compost_history_sources (compost_history_id, source_type, source_name, c_content, n_content, moisture_content, c_n_ratio, weight)
        VALUES (?,?,?,?,?,?,?,?)
    `

	// 插入氮源信息
	for _, source := range history.NitrogenSourcesList {
		fmt.Print(source)
		_, err := tx.Exec(insertSourceSQL, history.ID, "nitrogen", source.Name, source.C, source.N, source.Moisture, source.C_N, source.Weight)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 插入碳源信息
	for _, source := range history.CarbonSourcesList {
		_, err := tx.Exec(insertSourceSQL, history.ID, "carbon", source.Name, source.C, source.N, source.Moisture, source.C_N, source.Weight)
		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated,
		gin.H{
			"code": 200,
			"msg":  "ok",
			"data": history,
		})
}

// GetAllCompostHistories 获取所有堆肥历史记录
func (c *CompostController) GetCompostRecords(ctx *gin.Context) {
	userID := ctx.GetInt("userID")

	// 获取查询参数
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))
	startDate := ctx.Query("startDate")
	endDate := ctx.Query("endDate")
	sourceQuery := ctx.Query("sourceQuery")

	// 构建基础查询
	query := "SELECT DISTINCT ch.id, ch.all_volume, ch.cn_ratio, ch.density, ch.water_add, ch.created_at FROM compost_history ch " +
		"LEFT JOIN compost_history_sources chs ON ch.id = chs.compost_history_id " +
		"WHERE ch.user_id = ?"

	queryParams := []interface{}{userID}

	// 添加时间区间筛选
	if startDate != "" && endDate != "" {
		query += " AND ch.created_at BETWEEN ? AND ?"
		queryParams = append(queryParams, startDate, endDate)
	}

	// 添加氮源和碳源养料模糊查询
	if sourceQuery != "" {
		query += " AND (chs.source_name LIKE ? OR chs.source_type LIKE ?)"
		sourceQueryLike := "%" + sourceQuery + "%"
		queryParams = append(queryParams, sourceQueryLike, sourceQueryLike)
	}

	// 添加分页
	query += " ORDER BY ch.created_at DESC LIMIT ? OFFSET ?"
	queryParams = append(queryParams, pageSize, (page-1)*pageSize)

	// 执行查询
	historyRows, err := c.DB.Query(query, queryParams...)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database for compost histories"})
		return
	}
	defer historyRows.Close()

	var histories []models.CompostHistory
	for historyRows.Next() {
		var history models.CompostHistory
		err := historyRows.Scan(&history.ID, &history.AllVolume, &history.CNRatio, &history.Density, &history.WaterAdd, &history.CreatedAt)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning compost history row"})
			return
		}

		// 查询该堆肥历史记录对应的氮源和碳源信息
		sourceRows, err := c.DB.Query("SELECT source_type, source_name, c_content, n_content, moisture_content, c_n_ratio, weight FROM compost_history_sources WHERE compost_history_id = ?", history.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database for compost history sources"})
			return
		}
		defer sourceRows.Close()

		for sourceRows.Next() {
			var sourceType string
			var source models.Fertilizer
			err := sourceRows.Scan(&sourceType, &source.Name, &source.C, &source.N, &source.Moisture, &source.C_N, &source.Weight)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning compost history source row"})
				return
			}
			if sourceType == "nitrogen" {
				history.NitrogenSourcesList = append(history.NitrogenSourcesList, source)
			} else if sourceType == "carbon" {
				history.CarbonSourcesList = append(history.CarbonSourcesList, source)
			}
		}

		if err = sourceRows.Err(); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating compost history source rows"})
			return
		}

		histories = append(histories, history)
	}

	if err = historyRows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating compost history rows"})
		return
	}

	// 获取总记录数
	var totalCount int
	err = c.DB.QueryRow("SELECT COUNT(DISTINCT ch.id) FROM compost_history ch LEFT JOIN compost_history_sources chs ON ch.id = chs.compost_history_id WHERE ch.user_id = ?", userID).Scan(&totalCount)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total count"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":        200,
		"msg":         "ok",
		"data":        histories,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// GetCompostHistory 获取单个堆肥历史记录
func (c *CompostController) GetCompostRecord(ctx *gin.Context) {
	userID := ctx.GetInt("userID")
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var history models.CompostHistory
	query := "SELECT id, cn_ratio, density, water_add, created_at FROM compost_history WHERE id = ? AND user_id = ?"
	err = c.DB.QueryRow(query, id, userID).Scan(&history.ID, &history.CNRatio, &history.Density, &history.WaterAdd, &history.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Compost history not found"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 查询该堆肥历史记录对应的氮源和碳源信息
	sourceRows, err := c.DB.Query("SELECT source_type, source_name, c_content, n_content, moisture_content, c_n_ratio FROM compost_history_sources WHERE compost_history_id = ?", history.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database for compost history sources"})
		return
	}
	defer sourceRows.Close()

	for sourceRows.Next() {
		var sourceType string
		var source models.Fertilizer
		err := sourceRows.Scan(&sourceType, &source.Name, &source.C, &source.N, &source.Moisture, &source.C_N)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning compost history source row"})
			return
		}
		if sourceType == "nitrogen" {
			history.NitrogenSourcesList = append(history.NitrogenSourcesList, source)
		} else if sourceType == "carbon" {
			history.CarbonSourcesList = append(history.CarbonSourcesList, source)
		}
	}

	if err = sourceRows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating compost history source rows"})
		return
	}

	ctx.JSON(http.StatusOK,

		gin.H{
			"code": 200,
			"msg":  "ok",
			"data": history,
		})
}
