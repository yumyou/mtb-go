package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-mengtuobang/models"
)

// SoilController 处理测土配肥相关的请求
type SoilController struct {
	DB *sql.DB
}

// NewSoilController 创建一个新的SoilController实例
func NewSoilController(db *sql.DB) *SoilController {
	return &SoilController{DB: db}
}

// SaveSoilSoil 保存测土配肥记录
func (c *SoilController) SaveSoilRecord(ctx *gin.Context) {
	userID := ctx.GetInt("userID")
	var record models.Soil
	if err := ctx.ShouldBindJSON(&record); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开始事务
	tx, err := c.DB.Begin()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "事务开始失败"})
		return
	}

	// 准备SQL语句
	stmt, err := tx.Prepare(`
		INSERT INTO records (
			add_number, timestamp, location, crop, plot_size, average_yield,
			fertilizer_demand_n, fertilizer_demand_p2o5, fertilizer_demand_k2o, 
			total_supply_n, total_supply_p2o5, total_supply_k2o,
			supplement_n, supplement_p2o5, supplement_k2o,
			nitrogen_replenish_name, nitrogen_replenish_weight,
			phosphorus_replenish_name, phosphorus_replenish_weight,
			potassium_replenish_name, potassium_replenish_weight, 
			organic_fertilizer_name, organic_fertilizer_amount, 
			nitrogen_Basic_name, nitrogen_Basic_weight,
			phosphorus_Basic_name, phosphorus_Basic_weight,
			potassium_Basic_name, potassium_Basic_weight,
			custom_ratios, user_id
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`)

	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer stmt.Close()

	// 获取当前时间
	now := time.Now()
	timestamp := now.Format("2006-01-02 15:04:05")

	// 执行SQL语句
	result, err := stmt.Exec(
		record.AddNumber, timestamp, record.Location, record.Crop, record.PlotSize, record.AverageYield,
		record.FertilizerDemand.N, record.FertilizerDemand.P2O5, record.FertilizerDemand.K2O,
		record.TotalSupply.N, record.TotalSupply.P2O5, record.TotalSupply.K2O,
		record.Supplement.N, record.Supplement.P2O5, record.Supplement.K2O,
		record.NitrogenReplenish.Name, record.NitrogenReplenish.Weight,
		record.PhosphorusReplenish.Name, record.PhosphorusReplenish.Weight,
		record.PotassiumReplenish.Name, record.PotassiumReplenish.Weight,
		record.OrganicFertilizer.Name, record.OrganicFertilizer.Amount,
		record.NitrogenBasic.Name, record.NitrogenBasic.Weight,
		record.PhosphorusBasic.Name, record.PhosphorusBasic.Weight,
		record.PotassiumBasic.Name, record.PotassiumBasic.Weight,
		record.CustomRatios, userID,
	)

	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 获取插入记录的ID
	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	record.Id = int(id)
	record.UserId = userID
	record.Timestamp = timestamp

	ctx.JSON(http.StatusOK, record)
}

// GetSoilSoils 获取测土配肥记录列表
func (c *SoilController) GetSoilRecords(ctx *gin.Context) {
	userID := ctx.GetInt("userID")

	// 获取查询参数
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))
	startDate := ctx.Query("startDate")
	endDate := ctx.Query("endDate")
	location := ctx.Query("location")
	crop := ctx.Query("crop")

	// 构建基础查询
	query := `
		SELECT * FROM records 
		WHERE user_id = ?
	`

	queryParams := []interface{}{userID}

	// 添加筛选条件
	if startDate != "" && endDate != "" {
		query += " AND timestamp BETWEEN ? AND ?"
		queryParams = append(queryParams, startDate, endDate)
	}

	if location != "" {
		query += " AND location LIKE ?"
		queryParams = append(queryParams, "%"+location+"%")
	}

	if crop != "" {
		query += " AND crop LIKE ?"
		queryParams = append(queryParams, "%"+crop+"%")
	}

	// 添加排序和分页
	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	queryParams = append(queryParams, pageSize, (page-1)*pageSize)

	// 执行查询
	rows, err := c.DB.Query(query, queryParams...)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "查询记录失败"})
		return
	}
	defer rows.Close()

	var records []models.Soil
	for rows.Next() {
		var record models.Soil
		err := rows.Scan(
			&record.Id, &record.UserId, &record.AddNumber, &record.Timestamp, &record.Location, &record.Crop,
			&record.PlotSize, &record.AverageYield,
			&record.FertilizerDemand.N, &record.FertilizerDemand.P2O5, &record.FertilizerDemand.K2O,
			&record.TotalSupply.N, &record.TotalSupply.P2O5, &record.TotalSupply.K2O,
			&record.Supplement.N, &record.Supplement.P2O5, &record.Supplement.K2O,
			&record.NitrogenReplenish.Name, &record.NitrogenReplenish.Weight,
			&record.PhosphorusReplenish.Name, &record.PhosphorusReplenish.Weight,
			&record.PotassiumReplenish.Name, &record.PotassiumReplenish.Weight,
			&record.OrganicFertilizer.Name, &record.OrganicFertilizer.Amount,
			&record.NitrogenBasic.Name, &record.NitrogenBasic.Weight,
			&record.PhosphorusBasic.Name, &record.PhosphorusBasic.Weight,
			&record.PotassiumBasic.Name, &record.PotassiumBasic.Weight,
			&record.CustomRatios,
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		records = append(records, record)
	}

	// 获取总记录数
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM records WHERE user_id = ?"
	countParams := []interface{}{userID}

	if startDate != "" && endDate != "" {
		countQuery += " AND timestamp BETWEEN ? AND ?"
		countParams = append(countParams, startDate, endDate)
	}

	if location != "" {
		countQuery += " AND location LIKE ?"
		countParams = append(countParams, "%"+location+"%")
	}

	if crop != "" {
		countQuery += " AND crop LIKE ?"
		countParams = append(countParams, "%"+crop+"%")
	}

	err = c.DB.QueryRow(countQuery, countParams...).Scan(&totalCount)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取总记录数失败"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",

		"data":        records,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// GetSoilSoil 获取单个测土配肥记录
func (c *SoilController) GetSoilRecord(ctx *gin.Context) {
	userID := ctx.GetInt("userID")
	fmt.Println(userID)
	id := ctx.Query("id")
	fmt.Println(id)
	// id, err := strconv.Atoi(idStr)

	// if err != nil {
	// 	ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
	// 	return
	// }

	// 查询记录
	var record models.Soil
	query := "SELECT * FROM records WHERE id = ? AND user_id = ?"
	err := c.DB.QueryRow(query, id, userID).Scan(
		&record.Id, &record.UserId, &record.AddNumber, &record.Timestamp, &record.Location, &record.Crop,
		&record.PlotSize, &record.AverageYield,
		&record.FertilizerDemand.N, &record.FertilizerDemand.P2O5, &record.FertilizerDemand.K2O,
		&record.TotalSupply.N, &record.TotalSupply.P2O5, &record.TotalSupply.K2O,
		&record.Supplement.N, &record.Supplement.P2O5, &record.Supplement.K2O,
		&record.NitrogenReplenish.Name, &record.NitrogenReplenish.Weight,
		&record.PhosphorusReplenish.Name, &record.PhosphorusReplenish.Weight,
		&record.PotassiumReplenish.Name, &record.PotassiumReplenish.Weight,
		&record.OrganicFertilizer.Name, &record.OrganicFertilizer.Amount,
		&record.NitrogenBasic.Name, &record.NitrogenBasic.Weight,
		&record.PhosphorusBasic.Name, &record.PhosphorusBasic.Weight,
		&record.PotassiumBasic.Name, &record.PotassiumBasic.Weight,
		&record.CustomRatios,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "ok",
		"data": record,
	})
}
