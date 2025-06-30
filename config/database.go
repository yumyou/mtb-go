package config

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/go-sql-driver/mysql"
)

// 数据库连接信息
const (
	Username = "root"
	Password = "root"
	Hostname = "127.0.0.1:3306"
	DBName   = "mtb"
)

var (
	DB   *sql.DB
	once sync.Once
)

// ConnectDB 连接数据库
func ConnectDB() (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4", Username, Password, Hostname, DBName)
	return sql.Open("mysql", dsn)
}

// InitDB 初始化数据库连接
func InitDB() {
	once.Do(func() {
		var err error
		DB, err = ConnectDB()
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		if err = DB.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		// 自动迁移数据库
		if err = autoMigrate(DB); err != nil {
			log.Fatalf("Failed to migrate database: %v", err)
		}

		log.Println("Database connected and migrated successfully")
	})
}

// autoMigrate 自动迁移数据库
func autoMigrate(db *sql.DB) error {
	// 创建 migrations 表用于跟踪迁移状态
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %v", err)
	}

	// 运行所有迁移
	migrations := getMigrations()
	for _, migration := range migrations {
		if err := runMigrationIfNotExists(db, migration); err != nil {
			return fmt.Errorf("failed to run migration %s: %v", migration.Name, err)
		}
	}

	return nil
}

// Migration 迁移结构
type Migration struct {
	Name string
	SQL  string
}

// createMigrationsTable 创建迁移表
func createMigrationsTable(db *sql.DB) error {
	createSQL := `
	CREATE TABLE IF NOT EXISTS migrations (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255) NOT NULL UNIQUE,
		executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)
	`
	_, err := db.Exec(createSQL)
	return err
}

// getMigrations 获取所有迁移
func getMigrations() []Migration {
	return []Migration{
		{
			Name: "001_create_users_table",
			SQL: `
			CREATE TABLE IF NOT EXISTS users (
				id INT AUTO_INCREMENT PRIMARY KEY,
				username VARCHAR(255) UNIQUE,
				password VARCHAR(255),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
			`,
		},
		{
			Name: "002_add_phone_to_users",
			SQL: `
			ALTER TABLE users 
				ADD COLUMN phone VARCHAR(20),
				ADD COLUMN wechat_openid VARCHAR(255),
				ADD COLUMN wechat_unionid VARCHAR(255),
				ADD COLUMN nickname VARCHAR(255),
				ADD COLUMN avatar_url TEXT,
				ADD COLUMN avatar_base64 LONGTEXT,
				ADD COLUMN machine_code VARCHAR(255),
				ADD COLUMN last_login_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				ADD COLUMN login_method VARCHAR(50) DEFAULT 'username',
				ADD COLUMN role INT DEFAULT 0,
				ADD COLUMN status VARCHAR(20) DEFAULT 'active',
				ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				ADD INDEX idx_username (username),
				ADD INDEX idx_phone (phone),
				ADD INDEX idx_wechat_openid (wechat_openid),
				ADD INDEX idx_machine_code (machine_code)
			`,
		},
		{
			Name: "003_create_machine_codes_table",
			SQL: `
			CREATE TABLE IF NOT EXISTS machine_codes (
				id INT AUTO_INCREMENT PRIMARY KEY,
				code VARCHAR(255) NOT NULL UNIQUE,
				name VARCHAR(255),
				description TEXT,
				created_by INT,
				user_id INT,
				binded_at TIMESTAMP,
				is_active BOOLEAN DEFAULT TRUE,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
				INDEX idx_code (code),
				INDEX idx_created_by (created_by),
				FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL
			)
			`,
		},
	}
}

// runMigrationIfNotExists 如果迁移不存在则运行
func runMigrationIfNotExists(db *sql.DB, migration Migration) error {
	// 检查迁移是否已执行
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM migrations WHERE name = ?", migration.Name).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Printf("Migration %s already executed, skipping", migration.Name)
		return nil
	}

	// 执行迁移
	log.Printf("Running migration: %s", migration.Name)
	if _, err := db.Exec(migration.SQL); err != nil {
		return err
	}

	// 记录迁移已执行
	_, err = db.Exec("INSERT INTO migrations (name) VALUES (?)", migration.Name)
	return err
}
