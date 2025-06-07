package routes

import (
	"database/sql"

	"github.com/gin-gonic/gin"

	"go-mengtuobang/controllers"
	"go-mengtuobang/middleware"
)

// SetupRouter 配置所有路由
func SetupRouter(db *sql.DB) *gin.Engine {
	r := gin.Default()

	// 创建控制器实例
	compostController := controllers.NewCompostController(db)
	irrigationController := controllers.NewIrrigationController(db)
	soilController := controllers.NewSoilController(db)
	authController := controllers.NewAuthController(db)

	// 公共路由
	public := r.Group("/")
	{
		// 用户认证相关路由
		public.POST("/register", authController.Register)
		public.POST("/login", authController.Login)
	}

	// 需要认证的路由
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		// 堆肥相关路由
		protected.POST("/compost/save", compostController.SaveCompostRecord)
		protected.GET("/compost/records", compostController.GetCompostRecords)
		protected.GET("/compost/record", compostController.GetCompostRecord)

		// 灌溉相关路由
		protected.POST("/irrigation/save", irrigationController.SaveIrrigationRecord)
		protected.GET("/irrigation/records", irrigationController.GetIrrigationRecords)
		protected.GET("/irrigation/record", irrigationController.GetIrrigationRecord)

		// 测土配肥相关路由
		protected.POST("/soil/save", soilController.SaveSoilRecord)
		protected.GET("/soil/records", soilController.GetSoilRecords)
		protected.GET("/soil/record", soilController.GetSoilRecord)

	}

	return r
}
