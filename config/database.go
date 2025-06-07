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
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", Username, Password, Hostname, DBName)
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
	})
}