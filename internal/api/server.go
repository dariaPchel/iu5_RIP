package api

import (
	"log"
	"web_backend/internal/app/handler"
	"web_backend/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

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

	api := r.Group("/api")
	{
		api.GET("/risk_factors", h.GetRiskFactors)
		api.GET("/risk_factors/feed", h.GetFeed)
		api.GET("/risk_factors/feed/:id", h.GetFeed)
		api.GET("/risk_factors/draft", h.GetDraft)
		api.POST("/risk_factors", h.CreateRiskFactor)
		api.PUT("/risk_factors/:id/publish", h.PublishRiskFactor)
		api.DELETE("/risk_factors/:id", h.DeleteRiskFactor)
		api.POST("/risk_factors/:id/like", h.LikeRiskFactor)

		api.POST("/users/register", h.RegisterUser)
		api.POST("/users/login", h.LoginUser)
		api.POST("/users/logout", h.LogoutUser)
	}

	r.Run(":3000")
	log.Println("Server down")
}
