package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

var (
	db   *sql.DB
	once sync.Once
)

// Source 定义源结构体，数值字段为 string 类型
type Source struct {
	Name            string `json:"name"`
	CarbonContent   string `json:"carbon_content"`
	NitrogenContent string `json:"nitrogen_content"`
	MoistureContent string `json:"moisture_content"`
	Density         string `json:"density"`
}

// Result 定义计算结果结构体
type Result struct {
	CNRatio         string `json:"cn_ratio"`
	Density         string `json:"density"`
	MoistureContent string `json:"moisture_content"`
}

// 数据库连接信息
const (
	username = "root"
	password = "root"
	hostname = "127.0.0.1:3306"
	dbname   = "mtb"
)

// 连接数据库
func connectDB() (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", username, password, hostname, dbname)
	return sql.Open("mysql", dsn)
}

// 初始化数据库连接
func initDB() {
	once.Do(func() {
		var err error
		db, err = connectDB()
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		if err = db.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}
	})
}

type Fertilizer struct {
	Name     string  `json:"name"`
	Weight   float64 `json:"weight"`
	C        float64 `json:"c"`
	N        float64 `json:"n"`
	Moisture float64 `json:"moisture"`
	C_N      float64 `json:"c_n"`
}

type CompostHistory struct {
	ID                  int          `json:"id"`
	NitrogenSourcesList []Fertilizer `json:"nitrogenSourcesList"`
	CarbonSourcesList   []Fertilizer `json:"carbonSourcesList"`
	AllVolume           float64      `json:"allVolume"`
	CNRatio             string       `json:"cNRatio"`
	Density             string       `json:"density"`
	WaterAdd            string       `json:"waterAdd"`
	CreatedAt           string       `json:"created_at"`
	UserID              int          `json:"user_id"`
}

// 创建堆肥历史记录
func createCompostHistory(c *gin.Context) {
	userID := c.GetInt("userID")
	var history CompostHistory
	if err := c.ShouldBindJSON(&history); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开始事务
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	insertHistorySQL := `
        INSERT INTO compost_history (all_volume,cn_ratio, density, water_add, user_id)
        VALUES (?,?,?,?,?)
    `
	result, err := tx.Exec(insertHistorySQL, history.AllVolume, history.CNRatio, history.Density, history.WaterAdd, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	history.ID = int(id)

	insertSourceSQL := `
        INSERT INTO compost_history_sources (compost_history_id, source_type, source_name, c_content, n_content, moisture_content, c_n_ratio,weight)
        VALUES (?,?,?,?,?,?,?,?)
    `

	// 插入氮源信息
	for _, source := range history.NitrogenSourcesList {
		fmt.Print(source)
		_, err := tx.Exec(insertSourceSQL, history.ID, "nitrogen", source.Name, source.C, source.N, source.Moisture, source.C_N, source.Weight)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 插入碳源信息
	for _, source := range history.CarbonSourcesList {
		_, err := tx.Exec(insertSourceSQL, history.ID, "carbon", source.Name, source.C, source.N, source.Moisture, source.C_N, source.Weight)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, history)
}

// 获取所有堆肥历史记录
func getAllCompostHistories(c *gin.Context) {
	userID := c.GetInt("userID")

	// 获取查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	sourceQuery := c.Query("sourceQuery")

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
	historyRows, err := db.Query(query, queryParams...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database for compost histories"})
		return
	}
	defer historyRows.Close()

	var histories []CompostHistory
	for historyRows.Next() {
		var history CompostHistory
		err := historyRows.Scan(&history.ID, &history.AllVolume, &history.CNRatio, &history.Density, &history.WaterAdd, &history.CreatedAt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning compost history row"})
			return
		}

		// 查询该堆肥历史记录对应的氮源和碳源信息
		sourceRows, err := db.Query("SELECT source_type, source_name, c_content, n_content, moisture_content, c_n_ratio,weight FROM compost_history_sources WHERE compost_history_id = ?", history.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database for compost history sources"})
			return
		}
		defer sourceRows.Close()

		for sourceRows.Next() {
			var sourceType string
			var source Fertilizer
			err := sourceRows.Scan(&sourceType, &source.Name, &source.C, &source.N, &source.Moisture, &source.C_N, &source.Weight)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning compost history source row"})
				return
			}
			if sourceType == "nitrogen" {
				history.NitrogenSourcesList = append(history.NitrogenSourcesList, source)
			} else if sourceType == "carbon" {
				history.CarbonSourcesList = append(history.CarbonSourcesList, source)
			}
		}

		if err = sourceRows.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating compost history source rows"})
			return
		}

		histories = append(histories, history)
	}

	if err = historyRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating compost history rows"})
		return
	}

	// 获取总记录数
	var totalCount int
	err = db.QueryRow("SELECT COUNT(DISTINCT ch.id) FROM compost_history ch LEFT JOIN compost_history_sources chs ON ch.id = chs.compost_history_id WHERE ch.user_id = ?", userID).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error getting total count"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"histories":   histories,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// 获取单个堆肥历史记录
func getCompostHistory(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var history CompostHistory
	query := "SELECT id, cn_ratio, density, water_add, created_at FROM compost_history WHERE id = ? AND user_id = ?"
	err = db.QueryRow(query, id, userID).Scan(&history.ID, &history.CNRatio, &history.Density, &history.WaterAdd, &history.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Compost history not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 查询该堆肥历史记录对应的氮源和碳源信息
	sourceRows, err := db.Query("SELECT source_type, source_name, c_content, n_content, moisture_content, c_n_ratio FROM compost_history_sources WHERE compost_history_id =?", history.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database for compost history sources"})
		return
	}
	defer sourceRows.Close()

	for sourceRows.Next() {
		var sourceType string
		var source Fertilizer
		err := sourceRows.Scan(&sourceType, &source.Name, &source.C, &source.N, &source.Moisture, &source.C_N)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error scanning compost history source row"})
			return
		}
		if sourceType == "nitrogen" {
			history.NitrogenSourcesList = append(history.NitrogenSourcesList, source)
		} else if sourceType == "carbon" {
			history.CarbonSourcesList = append(history.CarbonSourcesList, source)
		}
	}

	if err = sourceRows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating compost history source rows"})
		return
	}

	c.JSON(http.StatusOK, history)
}

type Record struct {
	Id                int     `json:"id"`
	UserId            int     `json:"userId"`
	AddNumber         int     `json:"addNumber"`
	Timestamp         string  `json:"timestamp"`
	Location          string  `json:"location"`
	Crop              string  `json:"crop"`
	PlotSize          float64 `json:"plotSize"`
	AverageYield      float64 `json:"averageYield"`
	OrganicFertilizer struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	} `json:"organicFertilizer"`
	FertilizerDemand struct {
		N    float64 `json:"n"`
		P2O5 float64 `json:"p2o5"`
		K2O  float64 `json:"k2o"`
	} `json:"fertilizerDemand"`
	TotalSupply struct {
		N    float64 `json:"n"`
		P2O5 float64 `json:"p2o5"`
		K2O  float64 `json:"k2o"`
	} `json:"totalSupply"`
	Supplement struct {
		N    float64 `json:"n"`
		P2O5 float64 `json:"p2o5"`
		K2O  float64 `json:"k2o"`
	} `json:"supplement"`
	NitrogenReplenish struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"nitrogenReplenish"`
	PhosphorusReplenish struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"phosphorusReplenish"`
	PotassiumReplenish struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"potassiumReplenish"`

	NitrogenBasic struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"nitrogenBasic"`
	PhosphorusBasic struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"phosphorusBasic"`
	PotassiumBasic struct {
		Name   string  `json:"name"`
		Weight float64 `json:"weight"`
	} `json:"potassiumBasic"`
	CustomRatios string `json:"customRatios"`
}

// 创建测土配肥历史记录
func saveTesterCompostRecords(c *gin.Context) {
	userID := c.GetInt("userID")
	var record Record
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取数据库连接

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "事务开始失败"})
		return
	}

	// 准备SQL语句
	stmt, err := tx.Prepare("INSERT INTO records (add_number,timestamp, location, crop, plot_size,average_yield,fertilizer_demand_n, fertilizer_demand_p2o5, fertilizer_demand_k2o, total_supply_n, total_supply_p2o5, total_supply_k2o,supplement_n, supplement_p2o5, supplement_k2o,nitrogen_replenish_name,nitrogen_replenish_weight,phosphorus_replenish_name,phosphorus_replenish_weight,potassium_replenish_name,potassium_replenish_weight, organic_fertilizer_name,organic_fertilizer_amount, nitrogen_Basic_name,nitrogen_Basic_weight,phosphorus_Basic_name,phosphorus_Basic_weight,potassium_Basic_name,potassium_Basic_weight,custom_ratios,user_id) VALUES (?,?,?,?,?,?,?, ?, ?,?,?,?, ?, ?,?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? ,?,?,?,?,?)")
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}
	defer stmt.Close()

	// 将解析后的时间格式化为 MySQL 支持的格式
	now := time.Now()

	// 格式化为 "2006-01-02 15:04:05"
	timeStr := now.Format("2006-01-02 15:04:05")

	// 执行插入
	_, err = stmt.Exec(
		record.AddNumber,
		timeStr,
		record.Location,
		record.Crop,
		record.PlotSize,
		record.AverageYield,

		record.FertilizerDemand.N,
		record.FertilizerDemand.P2O5,
		record.FertilizerDemand.K2O,
		record.TotalSupply.N,
		record.TotalSupply.P2O5,
		record.TotalSupply.K2O,
		record.Supplement.N,
		record.Supplement.P2O5,
		record.Supplement.K2O,

		record.NitrogenReplenish.Name,
		record.NitrogenReplenish.Weight,
		record.PhosphorusReplenish.Name,
		record.PhosphorusReplenish.Weight,
		record.PotassiumReplenish.Name,
		record.PotassiumReplenish.Weight,

		record.OrganicFertilizer.Name,
		record.OrganicFertilizer.Amount,
		record.NitrogenBasic.Name,
		record.NitrogenBasic.Weight,
		record.PhosphorusBasic.Name,
		record.PhosphorusBasic.Weight,
		record.PotassiumBasic.Name,
		record.PotassiumBasic.Weight,
		record.CustomRatios,
		userID,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	// 提交事务
	tx.Commit()

	c.JSON(http.StatusCreated, gin.H{"message": "记录保存成功"})
}

// 获取测土配肥记录处理函数
func getAllTesterCompostHistories(c *gin.Context) {
	userID := c.GetInt("userID")
	recordID := c.Query("id") // 获取ID参数

	// 如果有ID参数，返回单条记录
	if recordID != "" {
		getSingleRecord(c, userID, recordID)
		return
	}

	// 否则返回分页列表
	getPaginatedRecords(c, userID)
}

// 获取单条记录
func getSingleRecord(c *gin.Context, userID int, recordID string) {
	var record Record
	query := `SELECT * FROM records WHERE user_id = ? AND id = ?`

	err := db.QueryRow(query, userID, recordID).Scan(
		&record.Id,
		&record.UserId,
		&record.AddNumber,
		&record.Timestamp,
		&record.Location,
		&record.Crop,
		&record.PlotSize,
		&record.AverageYield,

		&record.FertilizerDemand.N,
		&record.FertilizerDemand.P2O5,
		&record.FertilizerDemand.K2O,
		&record.TotalSupply.N,
		&record.TotalSupply.P2O5,
		&record.TotalSupply.K2O,
		&record.Supplement.N,
		&record.Supplement.P2O5,
		&record.Supplement.K2O,
		&record.NitrogenReplenish.Name,
		&record.NitrogenReplenish.Weight,
		&record.PhosphorusReplenish.Name,
		&record.PhosphorusReplenish.Weight,
		&record.PotassiumReplenish.Name,
		&record.PotassiumReplenish.Weight,
		&record.OrganicFertilizer.Name,
		&record.OrganicFertilizer.Amount,
		&record.NitrogenBasic.Name,
		&record.NitrogenBasic.Weight,
		&record.PhosphorusBasic.Name,
		&record.PhosphorusBasic.Weight,
		&record.PotassiumBasic.Name,
		&record.PotassiumBasic.Weight,
		&record.CustomRatios,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"record": record})
}

// 获取分页记录列表
func getPaginatedRecords(c *gin.Context, userID int) {
	// 获取查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	searchQuery := c.Query("searchQuery")

	// 构建基础查询
	query := "SELECT * FROM records WHERE user_id = ?"
	queryParams := []interface{}{userID}

	// 添加时间区间筛选
	if startDate != "" && endDate != "" {
		query += " AND timestamp BETWEEN ? AND ?"
		queryParams = append(queryParams, startDate, endDate)
	}

	// 添加地点和作物模糊查询
	if searchQuery != "" {
		query += " AND (location LIKE ? OR crop LIKE ?)"
		searchQueryLike := "%" + searchQuery + "%"
		queryParams = append(queryParams, searchQueryLike, searchQueryLike)
	}

	// 添加排序和分页
	query += " ORDER BY timestamp DESC LIMIT ? OFFSET ?"
	queryParams = append(queryParams, pageSize, (page-1)*pageSize)

	// 执行查询
	rows, err := db.Query(query, queryParams...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
		return
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.Id,
			&record.UserId,
			&record.AddNumber,
			&record.Timestamp,
			&record.Location,
			&record.Crop,
			&record.PlotSize,
			&record.AverageYield,

			&record.FertilizerDemand.N,
			&record.FertilizerDemand.P2O5,
			&record.FertilizerDemand.K2O,
			&record.TotalSupply.N,
			&record.TotalSupply.P2O5,
			&record.TotalSupply.K2O,
			&record.Supplement.N,
			&record.Supplement.P2O5,
			&record.Supplement.K2O,
			&record.NitrogenReplenish.Name,
			&record.NitrogenReplenish.Weight,
			&record.PhosphorusReplenish.Name,
			&record.PhosphorusReplenish.Weight,
			&record.PotassiumReplenish.Name,
			&record.PotassiumReplenish.Weight,

			&record.OrganicFertilizer.Name,
			&record.OrganicFertilizer.Amount,
			&record.NitrogenBasic.Name,
			&record.NitrogenBasic.Weight,
			&record.PhosphorusBasic.Name,
			&record.PhosphorusBasic.Weight,
			&record.PotassiumBasic.Name,
			&record.PotassiumBasic.Weight,
			&record.CustomRatios,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "扫描行数据失败: " + err.Error()})
			return
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "迭代行数据时出错: " + err.Error()})
		return
	}

	// 获取总记录数
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM records WHERE user_id = ?"
	countQueryParams := []interface{}{userID}

	if startDate != "" && endDate != "" {
		countQuery += " AND timestamp BETWEEN ? AND ?"
		countQueryParams = append(countQueryParams, startDate, endDate)
	}

	if searchQuery != "" {
		countQuery += " AND (location LIKE ? OR crop LIKE ?)"
		searchQueryLike := "%" + searchQuery + "%"
		countQueryParams = append(countQueryParams, searchQueryLike, searchQueryLike)
	}

	err = db.QueryRow(countQuery, countQueryParams...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取总记录数失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records":     records,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// 更新堆肥历史记录
func updateCompostHistory(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var history CompostHistory
	if err := c.ShouldBindJSON(&history); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 开启事务
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新主表信息
	updateSQL := `
        UPDATE compost_history
        SET cn_ratio = ?, density = ?, water_add = ?
        WHERE id = ? AND user_id = ?
    `
	result, err := tx.Exec(updateSQL, history.CNRatio, history.Density, history.WaterAdd, id, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Compost history not found"})
		return
	}

	// 先删除原有的关联源信息
	_, err = tx.Exec("DELETE FROM compost_history_sources WHERE compost_history_id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	insertSourceSQL := `
        INSERT INTO compost_history_sources (compost_history_id, source_type, source_name, c_content, n_content, moisture_content, c_n_ratio)
        VALUES (?,?,?,?,?,?,?)
    `

	// 插入新的氮源信息
	for _, source := range history.NitrogenSourcesList {
		_, err := tx.Exec(insertSourceSQL, id, "nitrogen", source.Name, source.C, source.N, source.Moisture, source.C_N)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 插入新的碳源信息
	for _, source := range history.CarbonSourcesList {
		_, err := tx.Exec(insertSourceSQL, id, "carbon", source.Name, source.C, source.N, source.Moisture, source.C_N)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	history.ID = id
	c.JSON(http.StatusOK, history)
}

// 删除堆肥历史记录
func deleteCompostHistory(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// 开启事务
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 先删除关联的源信息
	_, err = tx.Exec("DELETE FROM compost_history_sources WHERE compost_history_id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 删除主表记录
	deleteSQL := "DELETE FROM compost_history WHERE id = ? AND user_id = ?"
	result, err := tx.Exec(deleteSQL, id, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Compost history not found"})
		return
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// User 定义用户结构体
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// 注册
func registerUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 对密码进行加密
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not hash password"})
		return
	}

	// 插入用户数据
	insertSQL := `
		INSERT INTO users (username, password)
		VALUES (?, ?);
		`
	_, err = db.Exec(insertSQL, user.Username, string(hashedPassword))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "ok"})
}

// 用户登录
func loginUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查询用户
	var storedUser User
	querySQL := `
	SELECT id, username, password FROM users WHERE username = ?;
	`
	err := db.QueryRow(querySQL, user.Username).Scan(&storedUser.ID, &storedUser.Username, &storedUser.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(storedUser.Password), []byte(user.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// 生成 JWT Token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID":   storedUser.ID,
		"username": storedUser.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte("your-secret-key"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}
	response := gin.H{
		"msg":  "OK",
		"data": gin.H{"token": tokenString},
		"id":   storedUser.ID,
	}
	c.JSON(http.StatusOK, response)
}

// AuthMiddleware 验证 JWT Token 的中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// 提取 Token（去掉 "Bearer " 前缀）
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader { // 如果没有 "Bearer " 前缀
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Authorization header format"})
			c.Abort()
			return
		}

		// 解析 Token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte("your-secret-key"), nil
		})

		if err != nil {
			fmt.Println("Token parsing error:", err) // 打印错误信息
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			fmt.Println("Token claims:", claims) // 打印 claims
			userID := int(claims["userID"].(float64))
			c.Set("userID", userID)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
		}
	}
}

// IrrigationData defines the structure for irrigation data records
type IrrigationData struct {
	ID                          int64   `json:"id"`
	UserID                      int     `json:"user_id"`
	Timestamp                   string  `json:"timestamp"`
	PlotSize                    float64 `json:"plotSize"`
	MoistureContent             []int   `json:"moistureContent"`
	IrrigationMode              string  `json:"irrigationMode"`
	CustomEfficiency            float64 `json:"customEfficiency"`
	FlowRate                    float64 `json:"flowRate"`
	CropType                    string  `json:"cropType"`
	CustomDepth                 float64 `json:"customDepth"`
	OptimalMoisture             float64 `json:"optimalMoisture"`
	SoilType                    string  `json:"soilType"`
	CustomFieldCapacity         float64 `json:"customFieldCapacity"`
	SoilDensity                 float64 `json:"soilDensity"`
	Calculated                  bool    `json:"calculated"`
	WaterAmount                 float64 `json:"waterAmount"`
	IrrigationTime              float64 `json:"irrigationTime"`
	FertilizerTankSize          string  `json:"fertilizerTankSize"`
	FertilizerStartTime         float64 `json:"fertilizerStartTime"`
	FertilizerFlowRate          float64 `json:"fertilizerFlowRate"`
	FertilizerTotalTime         float64 `json:"fertilizerTotalTime"`
	FertilizerStartTimeNegative float64 `json:"fertilizerStartTimeNegative"`
}

// CreateIrrigationDataRecord handles creating a new irrigation data record
func CreateIrrigationDataRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	var record IrrigationData
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	record.UserID = userID
	record.Timestamp = time.Now().Format("2006-01-02 15:04:05")

	// 将 MoistureContent 转换为 JSON 字符串
	moistureContentJSON, err := json.Marshal(record.MoistureContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal moisture content: " + err.Error()})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
		return
	}

	// Insert the record
	insertSQL := `
        INSERT INTO irrigation_records (
            user_id, timestamp, plotSize, moistureContent, irrigationMode, customEfficiency, 
            flowRate, cropType, customDepth, optimalMoisture, soilType, 
            customFieldCapacity, soilDensity, calculated, waterAmount, 
            irrigationTime, fertilizerTankSize, fertilizerStartTime, 
            fertilizerFlowRate, fertilizerTotalTime, fertilizerStartTimeNegative
        ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
    `
	result, err := tx.Exec(
		insertSQL,
		record.UserID,
		record.Timestamp,
		record.PlotSize,
		moistureContentJSON,
		record.IrrigationMode,
		record.CustomEfficiency,
		record.FlowRate,
		record.CropType,
		record.CustomDepth,
		record.OptimalMoisture,
		record.SoilType,
		record.CustomFieldCapacity,
		record.SoilDensity,
		record.Calculated,
		record.WaterAmount,
		record.IrrigationTime,
		record.FertilizerTankSize,
		record.FertilizerStartTime,
		record.FertilizerFlowRate,
		record.FertilizerTotalTime,
		record.FertilizerStartTimeNegative,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert record: " + err.Error()})
		return
	}

	id, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get last insert ID: " + err.Error()})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	record.ID = id
	c.JSON(http.StatusCreated, record)
}

// GetIrrigationDataRecords retrieves all irrigation data records with pagination and filtering for a specific user
func GetIrrigationDataRecords(c *gin.Context) {
	userID := c.GetInt("userID")

	// Get query parameters
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page parameter. Page should be a positive integer"})
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pageSize parameter. Page size should be a positive integer"})
		return
	}
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	searchQuery := c.Query("searchQuery")

	// Build base query
	query := "SELECT * FROM irrigation_records WHERE user_id =?"
	queryParams := []interface{}{userID}

	// Add date range filter
	if startDate != "" && endDate != "" {
		query += " AND timestamp BETWEEN? AND?"
		queryParams = append(queryParams, startDate, endDate)
	}

	// Add search query
	if searchQuery != "" {
		query += " AND (cropType LIKE? OR soilType LIKE?)"
		searchQueryLike := "%" + searchQuery + "%"
		queryParams = append(queryParams, searchQueryLike, searchQueryLike)
	}

	// Add pagination
	query += " ORDER BY timestamp DESC LIMIT? OFFSET?"
	queryParams = append(queryParams, pageSize, (page-1)*pageSize)

	// Execute query
	rows, err := db.Query(query, queryParams...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute query: " + err.Error()})
		return
	}
	defer rows.Close()

	var records []IrrigationData
	for rows.Next() {
		var record IrrigationData
		var moistureContentJSON []byte
		err := rows.Scan(
			&record.ID,
			&record.UserID,
			&record.Timestamp,
			&record.PlotSize,
			&moistureContentJSON,
			&record.IrrigationMode,
			&record.CustomEfficiency,
			&record.FlowRate,
			&record.CropType,
			&record.CustomDepth,
			&record.OptimalMoisture,
			&record.SoilType,
			&record.CustomFieldCapacity,
			&record.SoilDensity,
			&record.Calculated,
			&record.WaterAmount,
			&record.IrrigationTime,
			&record.FertilizerTankSize,
			&record.FertilizerStartTime,
			&record.FertilizerFlowRate,
			&record.FertilizerTotalTime,
			&record.FertilizerStartTimeNegative,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan row: " + err.Error()})
			return
		}

		// 将 JSON 字符串转换回 MoistureContent 数组
		err = json.Unmarshal(moistureContentJSON, &record.MoistureContent)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal moisture content: " + err.Error()})
			return
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating rows: " + err.Error()})
		return
	}

	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM irrigation_records WHERE user_id =?"
	countParams := []interface{}{userID}

	if startDate != "" && endDate != "" {
		countQuery += " AND timestamp BETWEEN? AND?"
		countParams = append(countParams, startDate, endDate)
	}

	if searchQuery != "" {
		countQuery += " AND (cropType LIKE? OR soilType LIKE?)"
		searchQueryLike := "%" + searchQuery + "%"
		countParams = append(countParams, searchQueryLike, searchQueryLike)
	}

	err = db.QueryRow(countQuery, countParams...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total count: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records":     records,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// GetIrrigationDataRecord retrieves a single irrigation data record by ID for a specific user
func GetIrrigationDataRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var record IrrigationData
	var moistureContentJSON []byte
	query := `
        SELECT * FROM irrigation_records 
        WHERE id =? AND user_id =?
    `
	err = db.QueryRow(query, id, userID).Scan(
		&record.ID,
		&record.UserID,
		&record.Timestamp,
		&record.PlotSize,
		&moistureContentJSON,
		&record.IrrigationMode,
		&record.CustomEfficiency,
		&record.FlowRate,
		&record.CropType,
		&record.CustomDepth,
		&record.OptimalMoisture,
		&record.SoilType,
		&record.CustomFieldCapacity,
		&record.SoilDensity,
		&record.Calculated,
		&record.WaterAmount,
		&record.IrrigationTime,
		&record.FertilizerTankSize,
		&record.FertilizerStartTime,
		&record.FertilizerFlowRate,
		&record.FertilizerTotalTime,
		&record.FertilizerStartTimeNegative,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query record: " + err.Error()})
		}
		return
	}

	// 将 JSON 字符串转换回 MoistureContent 数组
	err = json.Unmarshal(moistureContentJSON, &record.MoistureContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal moisture content: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, record)
}

// UpdateIrrigationDataRecord updates an existing irrigation data record for a specific user
func UpdateIrrigationDataRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var record IrrigationData
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	record.UserID = userID
	// 将 MoistureContent 转换为 JSON 字符串
	moistureContentJSON, err := json.Marshal(record.MoistureContent)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal moisture content: " + err.Error()})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
		return
	}

	// Update the record
	updateSQL := `
        UPDATE irrigation_records SET
            user_id =?,
            timestamp =?,
            plotSize =?,
            moistureContent =?,
            irrigationMode =?,
            customEfficiency =?,
            flowRate =?,
            cropType =?,
            customDepth =?,
            optimalMoisture =?,
            soilType =?,
            customFieldCapacity =?,
            soilDensity =?,
            calculated =?,
            waterAmount =?,
            irrigationTime =?,
            fertilizerTankSize =?,
            fertilizerStartTime =?,
            fertilizerFlowRate =?,
            fertilizerTotalTime =?,
            fertilizerStartTimeNegative =?
        WHERE id =? AND user_id =?
    `
	result, err := tx.Exec(
		updateSQL,
		record.UserID,
		record.Timestamp,
		record.PlotSize,
		moistureContentJSON,
		record.IrrigationMode,
		record.CustomEfficiency,
		record.FlowRate,
		record.CropType,
		record.CustomDepth,
		record.OptimalMoisture,
		record.SoilType,
		record.CustomFieldCapacity,
		record.SoilDensity,
		record.Calculated,
		record.WaterAmount,
		record.IrrigationTime,
		record.FertilizerTankSize,
		record.FertilizerStartTime,
		record.FertilizerFlowRate,
		record.FertilizerTotalTime,
		record.FertilizerStartTimeNegative,
		id,
		userID,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record: " + err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get rows affected: " + err.Error()})
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or not owned by user"})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	record.ID = id
	c.JSON(http.StatusOK, record)
}

// WaterRecord defines the structure for water records
type WaterRecord struct {
	ID              int         `json:"id"`
	UserID          int         `json:"user_id"`
	IrrigationMode  string      `json:"irrigationMode"`
	Efficiency      float64     `json:"efficiency"`
	FlowRate        float64     `json:"flowRate"`
	CropType        string      `json:"cropType"`
	Depth           float64     `json:"depth"`
	OptimalMoisture float64     `json:"optimalMoisture"`
	SoilType        string      `json:"soilType"`
	FieldCapacity   float64     `json:"fieldCapacity"`
	SoilDensity     float64     `json:"soilDensity"`
	CreatedAt       string      `json:"created_at"`
	Areas           []WaterArea `json:"areas"`
}

// WaterArea defines the structure for water areas
type WaterArea struct {
	ID             int     `json:"id"`
	RecordID       int     `json:"record_id"`
	PlotSize       float64 `json:"plotSize"`
	TankSize       float64 `json:"tankSize"`
	WaterAmount    float64 `json:"waterAmount"`
	IrrigationTime float64 `json:"irrigationTime"`

	FertilizerStartTime         float64 `json:"fertilizerStartTime"`
	FertilizerStartTimeNegative float64 `json:"fertilizerStartTimeNegative"`
	FertilizerTotalTime         float64 `json:"fertilizerTotalTime"`

	FertilizerFlowRate float64 `json:"fertilizerFlowRate"`

	MoisturePoints []MoisturePoint `json:"moisturePoints"`
}

// MoisturePoint defines the structure for moisture points
type MoisturePoint struct {
	Value float64 `json:"value"`
}

// CreateWaterRecord handles creating a new water record with associated areas
func CreateWaterRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	var record WaterRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	record.UserID = userID
	record.CreatedAt = time.Now().Format("2006-01-02 15:04:05")

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
		return
	}

	// Insert the main water record
	insertRecordSQL := `
        INSERT INTO water_records (
            user_id, irrigation_mode, efficiency, flow_rate, crop_type, 
            depth, optimal_moisture, soil_type, field_capacity, soil_density, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	result, err := tx.Exec(
		insertRecordSQL,
		record.UserID,
		record.IrrigationMode,
		record.Efficiency,
		record.FlowRate,
		record.CropType,
		record.Depth,
		record.OptimalMoisture,
		record.SoilType,
		record.FieldCapacity,
		record.SoilDensity,
		record.CreatedAt,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert water record: " + err.Error()})
		return
	}

	recordID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get last insert ID: " + err.Error()})
		return
	}
	record.ID = int(recordID)

	// Insert water areas with moisture points as JSON
	for i, area := range record.Areas {
		// Convert moisture points to JSON
		moisturePointsJSON, err := json.Marshal(area.MoisturePoints)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal moisture points for area %d: %v", i+1, err)})
			return
		}

		insertAreaSQL := `
            INSERT INTO water_areas (
                record_id, plot_size, tank_size, water_amount, irrigation_time, 
                fertilizer_flow_rate, fertilizer_start_time, fertilizer_start_time_negative, 
                fertilizer_total_time_negative, moisture_points
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `
		_, err = tx.Exec(
			insertAreaSQL,
			record.ID,
			area.PlotSize,
			area.TankSize,
			area.WaterAmount,
			area.IrrigationTime,
			area.FertilizerFlowRate,
			area.FertilizerStartTime,
			area.FertilizerStartTimeNegative,
			area.FertilizerTotalTime,
			moisturePointsJSON,
		)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to insert water area %d: %v", i+1, err)})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, record)
}

// GetWaterRecords retrieves all water records with pagination and filtering for a specific user
func GetWaterRecords(c *gin.Context) {
	userID := c.GetInt("userID")

	// Get query parameters
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page parameter. Page should be a positive integer"})
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid pageSize parameter. Page size should be a positive integer"})
		return
	}
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	searchQuery := c.Query("searchQuery")

	// Build base query
	query := "SELECT id, irrigation_mode, efficiency, flow_rate, crop_type, depth, optimal_moisture, soil_type, field_capacity, soil_density, created_at FROM water_records WHERE user_id = ?"
	queryParams := []interface{}{userID}

	// Add date range filter
	if startDate != "" && endDate != "" {
		query += " AND created_at BETWEEN ? AND ?"
		queryParams = append(queryParams, startDate, endDate)
	}

	// Add search query
	if searchQuery != "" {
		query += " AND (crop_type LIKE ? OR soil_type LIKE ?)"
		searchQueryLike := "%" + searchQuery + "%"
		queryParams = append(queryParams, searchQueryLike, searchQueryLike)
	}

	// Add pagination
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	queryParams = append(queryParams, pageSize, (page-1)*pageSize)

	// Execute query
	rows, err := db.Query(query, queryParams...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute query: " + err.Error()})
		return
	}
	defer rows.Close()

	var records []WaterRecord
	for rows.Next() {
		var record WaterRecord
		err := rows.Scan(
			&record.ID,
			&record.IrrigationMode,
			&record.Efficiency,
			&record.FlowRate,
			&record.CropType,
			&record.Depth,
			&record.OptimalMoisture,
			&record.SoilType,
			&record.FieldCapacity,
			&record.SoilDensity,
			&record.CreatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan row: " + err.Error()})
			return
		}
		record.UserID = userID

		// Get areas for this record
		areaRows, err := db.Query(`
            SELECT id, plot_size, tank_size, water_amount, irrigation_time, 
                   fertilizer_flow_rate, fertilizer_start_time, fertilizer_start_time_negative, 
                   fertilizer_total_time_negative, moisture_points 
            FROM water_areas 
            WHERE record_id = ?`, record.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query water areas: " + err.Error()})
			return
		}
		defer areaRows.Close()

		for areaRows.Next() {
			var area WaterArea
			var moisturePointsJSON string

			err := areaRows.Scan(
				&area.ID,
				&area.PlotSize,
				&area.TankSize,
				&area.WaterAmount,
				&area.IrrigationTime,
				&area.FertilizerFlowRate,
				&area.FertilizerStartTime,
				&area.FertilizerStartTimeNegative,
				&area.FertilizerTotalTime,
				&moisturePointsJSON,
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan area row: " + err.Error()})
				return
			}

			// Parse moisture points JSON
			if moisturePointsJSON != "" {
				var points []MoisturePoint
				if err := json.Unmarshal([]byte(moisturePointsJSON), &points); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse moisture points: " + err.Error()})
					return
				}
				area.MoisturePoints = points
			}

			area.RecordID = record.ID
			record.Areas = append(record.Areas, area)
		}

		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating rows: " + err.Error()})
		return
	}

	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM water_records WHERE user_id = ?"
	countParams := []interface{}{userID}

	if startDate != "" && endDate != "" {
		countQuery += " AND created_at BETWEEN ? AND ?"
		countParams = append(countParams, startDate, endDate)
	}

	if searchQuery != "" {
		countQuery += " AND (crop_type LIKE ? OR soil_type LIKE ?)"
		searchQueryLike := "%" + searchQuery + "%"
		countParams = append(countParams, searchQueryLike, searchQueryLike)
	}

	err = db.QueryRow(countQuery, countParams...).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total count: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records":     records,
		"totalCount":  totalCount,
		"currentPage": page,
		"pageSize":    pageSize,
	})
}

// GetWaterRecord retrieves a single water record by ID with all associated areas
func GetWaterRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var record WaterRecord
	query := `
        SELECT id, irrigation_mode, efficiency, flow_rate, crop_type, depth, 
               optimal_moisture, soil_type, field_capacity, soil_density, created_at 
        FROM water_records 
        WHERE id = ? AND user_id = ?
    `
	err = db.QueryRow(query, id, userID).Scan(
		&record.ID,
		&record.IrrigationMode,
		&record.Efficiency,
		&record.FlowRate,
		&record.CropType,
		&record.Depth,
		&record.OptimalMoisture,
		&record.SoilType,
		&record.FieldCapacity,
		&record.SoilDensity,
		&record.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query record: " + err.Error()})
		}
		return
	}
	record.UserID = userID

	// Get all areas for this record
	areaRows, err := db.Query(`
        SELECT id, plot_size, tank_size, water_amount, irrigation_time, 
               fertilizer_flow_rate, fertilizer_start_time, fertilizer_start_time_negative, 
               fertilizer_total_time_negative, moisture_points 
        FROM water_areas 
        WHERE record_id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query water areas: " + err.Error()})
		return
	}
	defer areaRows.Close()

	for areaRows.Next() {
		var area WaterArea
		var moisturePointsJSON string

		err := areaRows.Scan(
			&area.ID,
			&area.PlotSize,
			&area.TankSize,
			&area.WaterAmount,
			&area.IrrigationTime,
			&area.FertilizerFlowRate,
			&area.FertilizerStartTime,
			&area.FertilizerStartTimeNegative,
			&area.FertilizerTotalTime,
			&moisturePointsJSON,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan area row: " + err.Error()})
			return
		}

		// Parse moisture points JSON
		if moisturePointsJSON != "" {
			var points []MoisturePoint
			if err := json.Unmarshal([]byte(moisturePointsJSON), &points); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse moisture points: " + err.Error()})
				return
			}
			area.MoisturePoints = points
		}

		area.RecordID = id
		record.Areas = append(record.Areas, area)
	}

	c.JSON(http.StatusOK, record)
}

// UpdateWaterRecord updates an existing water record with all associated data
func UpdateWaterRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var record WaterRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format: " + err.Error()})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
		return
	}

	// Update the main water record
	updateRecordSQL := `
        UPDATE water_records SET
            irrigation_mode = ?,
            efficiency = ?,
            flow_rate = ?,
            crop_type = ?,
            depth = ?,
            optimal_moisture = ?,
            soil_type = ?,
            field_capacity = ?,
            soil_density = ?
        WHERE id = ? AND user_id = ?
    `
	result, err := tx.Exec(
		updateRecordSQL,
		record.IrrigationMode,
		record.Efficiency,
		record.FlowRate,
		record.CropType,
		record.Depth,
		record.OptimalMoisture,
		record.SoilType,
		record.FieldCapacity,
		record.SoilDensity,
		id,
		userID,
	)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update water record: " + err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get rows affected: " + err.Error()})
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or not owned by user"})
		return
	}

	// Delete existing areas
	_, err = tx.Exec("DELETE FROM water_areas WHERE record_id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete existing water areas: " + err.Error()})
		return
	}

	// Insert new areas with moisture points as JSON
	for i, area := range record.Areas {
		// Convert moisture points to JSON
		moisturePointsJSON, err := json.Marshal(area.MoisturePoints)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to marshal moisture points for area %d: %v", i+1, err)})
			return
		}

		insertAreaSQL := `
            INSERT INTO water_areas (
                record_id, plot_size, tank_size, water_amount, irrigation_time, 
                fertilizer_flow_rate, fertilizer_start_time, fertilizer_start_time_negative, 
                fertilizer_total_time_negative, moisture_points
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        `
		_, err = tx.Exec(
			insertAreaSQL,
			id,
			area.PlotSize,
			area.TankSize,
			area.WaterAmount,
			area.IrrigationTime,
			area.FertilizerFlowRate,
			area.FertilizerStartTime,
			area.FertilizerStartTimeNegative,
			area.FertilizerTotalTime,
			moisturePointsJSON,
		)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to insert water area %d: %v", i+1, err)})
			return
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	record.ID = id
	record.UserID = userID
	c.JSON(http.StatusOK, record)
}

// DeleteWaterRecord deletes a water record and all associated data
func DeleteWaterRecord(c *gin.Context) {
	userID := c.GetInt("userID")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction: " + err.Error()})
		return
	}

	// Delete water areas
	_, err = tx.Exec("DELETE FROM water_areas WHERE record_id = ?", id)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete water areas: " + err.Error()})
		return
	}

	// Delete the main record
	result, err := tx.Exec("DELETE FROM water_records WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete water record: " + err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get rows affected: " + err.Error()})
		return
	}
	if rowsAffected == 0 {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Record not found or not owned by user"})
		return
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction: " + err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}
func main() {
	initDB()
	r := gin.Default()

	// 注册用户
	r.POST("/registerUser", registerUser)
	// 用户登录
	r.POST("/loginUser", loginUser)

	// 受保护的路由
	auth := r.Group("/")
	auth.Use(AuthMiddleware())
	{
		// 水肥管理
		auth.POST("/saveIrrigationRecords", CreateIrrigationDataRecord)
		auth.GET("/getIrrigationDataRecords", GetIrrigationDataRecords)
		auth.GET("/irrigation-records/:id", GetIrrigationDataRecord)
		auth.PUT("/irrigation-records/:id", UpdateIrrigationDataRecord)

		//创建灌溉
		auth.POST("/water-records", CreateWaterRecord)
		auth.GET("/water-records", GetWaterRecords)
		auth.GET("/water-records/:id", GetWaterRecord)
		auth.PUT("/water-records/:id", UpdateWaterRecord)
		auth.DELETE("/water-records/:id", DeleteWaterRecord)

		// 创建堆肥历史记录
		auth.POST("/saveCompostRecords", createCompostHistory)
		// 获取所有堆肥历史记录
		auth.GET("/getCompostHistory", getAllCompostHistories)
		// 获取单个堆肥历史记录
		auth.GET("/compost-history/:id", getCompostHistory)
		// 更新堆肥历史记录
		auth.PUT("/compost-history/:id", updateCompostHistory)
		// 删除堆肥历史记录
		auth.DELETE("/compost-history/:id", deleteCompostHistory)
		//创建测土培肥历史记录
		auth.POST("/saveTesterCompostRecords", saveTesterCompostRecords)
		// 获取所有测土培肥历史记录
		auth.GET("/getTesterCompostHistory", getAllTesterCompostHistories)
	}

	// 启动服务
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server failed to start: ", err)
	}
}
