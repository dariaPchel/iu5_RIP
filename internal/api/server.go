package api

import (
	"log"

	"web_backend/internal/app/handler"
	"web_backend/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	// Импорт сгенерированной документации Swagger
	// Имя "web_backend" должно точно совпадать с первой строкой твоего go.mod
	_ "web_backend/docs"

	swaggerFiles "github.com/swaggo/files" // <-- ИСПРАВЛЕНО: правильный путь к файлам
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title POSPOM API
// @version 1.0
// @description Backend для SPA POSPOM (Лабораторная работа 4)
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func StartServer() {
	log.Println("Starting server")

	repo, err := repository.NewRepository()
	if err != nil {
		logrus.Error("Ошибка инициализации репозитория:", err)
		return
	}

	h := handler.NewHandler(repo)
	r := gin.Default()

	r.Static("/static", "./resources")
	r.LoadHTMLGlob("templates/*")

	// Маршрут для Swagger UI (будет доступен по http://localhost:3000/swagger/index.html)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		// Публичные маршруты (доступны без токена)
		api.GET("/risk_factors", h.GetRiskFactors)
		api.GET("/risk_factors/feed", h.GetFeed)
		api.GET("/risk_factors/feed/:id", h.GetFeed)

		api.POST("/users/register", h.RegisterUser)
		api.POST("/users/login", h.LoginUser)

		// Защищенные маршруты (требуется JWT Bearer токен)
		api.GET("/risk_factors/draft", h.AuthMiddleware(), h.GetDraft)
		api.POST("/risk_factors", h.AuthMiddleware(), h.CreateRiskFactor)
		api.PUT("/risk_factors/:id/publish", h.AuthMiddleware(), h.PublishRiskFactor)
		api.DELETE("/risk_factors/:id", h.AuthMiddleware(), h.DeleteRiskFactor)
		api.POST("/risk_factors/:id/like", h.AuthMiddleware(), h.LikeRiskFactor)

		api.POST("/users/logout", h.AuthMiddleware(), h.LogoutUser)
	}

	log.Println("Server is running on port 3000")
	r.Run(":3000")
	log.Println("Server down")
}
