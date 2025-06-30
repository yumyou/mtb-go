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
	// 机器码相关路由
	machineController := &controllers.MachineController{db}

	// 公共路由
	public := r.Group("/")
	{
		// 用户认证相关路由
		public.POST("/register", authController.Register)
		public.POST("/login", authController.Login)
		public.POST("/wxLogin", authController.WechatLogin)

		// 短信验证码相关（公共接口）
		public.POST("/sms/send", authController.SendSMS)
		public.POST("/password/reset", authController.ResetPassword)
	}

	// 需要认证的路由
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		// 用户信息相关
		protected.GET("/user/info", authController.GetUserInfo)
		protected.POST("/user/bind-phone", authController.BindPhone)

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

		//机器码
		protected.POST("/machine/create", machineController.CreateMachineCode)
		protected.POST("/machine/check", machineController.CheckMachineCode)
		protected.POST("/machine/bind", machineController.BindMachineCode)
		protected.GET("/machine/user/:userId", machineController.GetUserMachineCode)
		protected.DELETE("/machine/user/:userId", machineController.UnbindMachineCode)
	}

	return r
}
