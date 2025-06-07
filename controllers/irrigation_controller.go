package controllers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-mengtuobang/models"
)

// IrrigationController 处理灌溉相关的请求
type IrrigationController struct {
	DB *sql.DB
}

// NewIrrigationController 创建一个新的IrrigationController实例
func NewIrrigationController(db *sql.DB) *IrrigationController {
	return &IrrigationController{DB: db}
}

// SaveIrrigationData 保存灌溉数据
func (c *IrrigationController) SaveIrrigationRecord(ctx *gin.Context) {
	userID := ctx.GetInt("userID")
	var requestData struct {
		AreaNum         int               `json:"areaNum"`
		IrrigationMode  string            `json:"irrigationMode"`
		Efficiency      float64           `json:"efficiency"`
		CropType        string            `json:"cropType"`
		Depth           float64           `json:"depth"`
		OptimalMoisture float64           `json:"optimalMoisture"`
		SoilType        string            `json:"soilType"`
		FieldCapacity   float64           `json:"fieldCapacity"`
		SoilDensity     float64           `json:"soilDensity"`
		Areas           []models.AreaData `json:"areas"`
	}

	if err := ctx.ShouldBindJSON(&requestData); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开始事务
	tx, err := c.DB.Begin()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 插入灌溉记录
	insertRecordSQL := `
		INSERT INTO water_records (
			user_id, irrigation_mode, efficiency, crop_type, 
			depth, optimal_moisture, soil_type, field_capacity, soil_density
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := tx.Exec(
		insertRecordSQL,
		userID,
		requestData.IrrigationMode,
		requestData.Efficiency,
		requestData.CropType,
		requestData.Depth,
		requestData.OptimalMoisture,
		requestData.SoilType,
		requestData.FieldCapacity,
		requestData.SoilDensity,
	)

	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	recordID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 插入区域数据
	insertAreaSQL := `
		INSERT INTO water_areas (
			record_id, moisture_points,plot_size,water_flow_rate,tank_size, water_amount, irrigation_time,
			fertilizer_start_time, fertilizer_total_time, fertilizer_flow_rate, negative
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?,?,?)
	`

	// 循环处理每个区域
	for _, area := range requestData.Areas {
		moisturePointsJSON, err := json.Marshal(area.MoisturePoints)
		// 将布尔值转换为整数
		negative := 0
		if area.Negative {
			negative = 1
		}

		result, err = tx.Exec(
			insertAreaSQL,
			recordID,
			moisturePointsJSON,
			area.PlotSize,
			area.WaterFlowRate,
			area.TankSize,
			area.WaterAmount,
			area.IrrigationTime, // 转换为字符串
			area.FertilizerStartTime,
			area.FertilizerTotalTime,
			area.FertilizerFlowRate,
			negative,
		)

		if err != nil {
			tx.Rollback()
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 提交事务
	if err = tx.Commit(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK,
		gin.H{
			"code":      200,
			"msg":       "ok",
			"record_id": recordID,
		},
	)
}

// GetIrrigationRecords 获取灌溉记录
func (c *IrrigationController) GetIrrigationRecords(ctx *gin.Context) {
	userID := ctx.GetInt("userID")

	// 获取查询参数
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))
	startDate := ctx.Query("startDate")
	endDate := ctx.Query("endDate")
	irrigationMode := ctx.Query("query") // 新增：灌溉方式模糊查询参数

	// 构建基础查询
	query := `
		SELECT 
			id, user_id, irrigation_mode, efficiency, crop_type, 
			depth, optimal_moisture, soil_type, field_capacity, soil_density, created_at 
		FROM water_records 
		WHERE user_id = ?
	`

	queryParams := []interface{}{userID}

	// 添加时间区间筛选
	if startDate != "" && endDate != "" {
		query += " AND created_at BETWEEN ? AND ?"
		queryParams = append(queryParams, startDate, endDate)
	}

	// 添加灌溉方式模糊查询（如果参数不为空）
	if irrigationMode != "" {
		query += " AND irrigation_mode LIKE ?"
		queryParams = append(queryParams, "%"+irrigationMode+"%") // 使用 % 实现模糊匹配
	}

	// 添加排序和分页
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	queryParams = append(queryParams, pageSize, (page-1)*pageSize)

	// 执行查询
	rows, err := c.DB.Query(query, queryParams...)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "查询灌溉记录失败"})
		return
	}
	defer rows.Close()

	var records []models.WaterRecord
	for rows.Next() {
		var record models.WaterRecord
		err := rows.Scan(
			&record.ID, &record.UserID, &record.IrrigationMode, &record.Efficiency,
			&record.CropType, &record.Depth, &record.OptimalMoisture, &record.SoilType,
			&record.FieldCapacity, &record.SoilDensity, &record.CreatedAt,
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "解析灌溉记录失败"})
			return
		}

		// 查询区域数据（保持不变）
		areaRows, err := c.DB.Query(`
			SELECT 
				id, record_id, plot_size, tank_size, water_amount, irrigation_time,fertilizer_start_time, fertilizer_total_time, fertilizer_flow_rate,moisture_points, negative
			FROM water_areas 
			WHERE record_id = ?
		`, record.ID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "查询灌溉区域失败"})
			return
		}
		defer areaRows.Close()

		for areaRows.Next() {
			var area models.WaterArea
			err := areaRows.Scan(
				&area.ID, &area.RecordID, &area.PlotSize, &area.TankSize, &area.WaterAmount, &area.IrrigationTime,
				&area.FertilizerStartTime, &area.FertilizerTotalTime, &area.FertilizerFlowRate, &area.MoisturePoints, &area.Negative,
			)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "解析灌溉区域失败"})
				return
			}

			record.Areas = append(record.Areas, area)
		}

		records = append(records, record)
	}

	// 获取总记录数（同时考虑灌溉方式筛选）
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM water_records WHERE user_id = ?"
	countParams := []interface{}{userID}

	if startDate != "" && endDate != "" {
		countQuery += " AND created_at BETWEEN ? AND ?"
		countParams = append(countParams, startDate, endDate)
	}

	// 在总数查询中也加入灌溉方式筛选
	if irrigationMode != "" {
		countQuery += " AND irrigation_mode LIKE ?"
		countParams = append(countParams, "%"+irrigationMode+"%")
	}

	err = c.DB.QueryRow(countQuery, countParams...).Scan(&totalCount)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "获取总记录数失败"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":        200,
		"msg":         "ok",
		"data":        records,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// GetIrrigationRecord 获取单个灌溉记录
func (c *IrrigationController) GetIrrigationRecord(ctx *gin.Context) {
	userID := ctx.GetInt("userID")
	id := ctx.Query("id")

	// 查询记录
	var record models.WaterRecord
	query := `
		SELECT 
			id, user_id, irrigation_mode, efficiency, crop_type, 
			depth, optimal_moisture, soil_type, field_capacity, soil_density, created_at 
		FROM water_records 
		WHERE id = ? AND user_id = ?
	`
	fmt.Println(query)
	fmt.Println(id)
	err := c.DB.QueryRow(query, id, userID).Scan(
		&record.ID, &record.UserID, &record.IrrigationMode, &record.Efficiency,
		&record.CropType, &record.Depth, &record.OptimalMoisture, &record.SoilType,
		&record.FieldCapacity, &record.SoilDensity, &record.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 查询区域数据
	areaRows, err := c.DB.Query(`
		SELECT 
			id, record_id, plot_size, water_flow_rate,tank_size, water_amount, irrigation_time,
			fertilizer_start_time, fertilizer_total_time, fertilizer_flow_rate,moisture_points, negative
		FROM water_areas 
		WHERE record_id = ?
	`, record.ID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer areaRows.Close()

	for areaRows.Next() {
		var area models.WaterArea
		var negative int
		err := areaRows.Scan(
			&area.ID, &area.RecordID, &area.PlotSize, &area.WaterFlowRate, &area.TankSize, &area.WaterAmount, &area.IrrigationTime,
			&area.FertilizerStartTime, &area.FertilizerTotalTime, &area.FertilizerFlowRate, &area.MoisturePoints, &negative,
		)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "解析灌溉区域失败"})
			return
		}

		record.Areas = append(record.Areas, area)
	}

	ctx.JSON(http.StatusOK,
		gin.H{
			"code": 200,
			"msg":  "ok",
			"data": record,
		},
	)
}
