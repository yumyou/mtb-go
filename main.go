package main

import (
	"log"

	"go-mengtuobang/config"
	"go-mengtuobang/routes"
)

func main() {
	// 初始化数据库连接
	config.InitDB()

	// 设置路由
	r := routes.SetupRouter(config.DB)

	// 启动服务器
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
