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
		logrus.Error("Ошибка инициализации репозитория")
	}
	h := handler.NewHandler(repo)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.Static("/static", "./resources")

	r.GET("/feed/:id", h.GetFeed)
	r.GET("/add", h.GetAdd)
	r.GET("/grid", h.GetGrid)

	r.Run(":3000")
	log.Println("Server down")
}
