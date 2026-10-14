package handler

import (
	"net/http"
	"strconv"
	"web_backend/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

type FeedView struct {
	ID             uint
	Name           string
	Description    string
	Age            int
	BMI            float64
	Comorbidity    string
	SurgeryType    string
	RiskPercentage float64
	ImageURL       string
	VideoURL       string
	Status         string
	LikesCount     int
}

type GridView struct {
	ID         uint
	Name       string
	ImageURL   string
	Status     string
	LikesCount int
}

func (h *Handler) GetFeed(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.Redirect(http.StatusFound, "/risk_factors_feed/1")
		return
	}

	var factor repository.RiskFactor
	if ctx.Query("next") == "true" {
		factor, err = h.Repository.GetNextRiskFactor(id)
	} else if ctx.Query("prev") == "true" {
		factor, err = h.Repository.GetPrevRiskFactor(id)
	} else {
		factor, err = h.Repository.GetRiskFactor(id)
	}

	if err != nil {
		ctx.Redirect(http.StatusFound, "/risk_factors_feed/1")
		return
	}

	if factor.Status != "published" {
		ctx.Redirect(http.StatusFound, "/risk_factors_feed/1")
		return
	}

	likesCount := len(factor.LikedBy)
	feedView := FeedView{
		ID:             factor.ID,
		Name:           factor.Name,
		Description:    factor.Description,
		Age:            factor.Age,
		BMI:            factor.BMI,
		RiskPercentage: factor.RiskPercentage,
		ImageURL:       factor.ImageURL,
		VideoURL:       factor.VideoURL,
		Status:         factor.Status,
		LikesCount:     likesCount,
	}

	ctx.HTML(http.StatusOK, "risk_factors_feed.html", gin.H{"factor": feedView})
}

func (h *Handler) GetAdd(ctx *gin.Context) {
	factor, err := h.Repository.GetDraft()
	if err != nil {
		factor = repository.RiskFactor{}
	}
	ctx.HTML(http.StatusOK, "risk_factors_add.html", gin.H{"factor": factor})
}

func (h *Handler) GetGrid(ctx *gin.Context) {
	var factors []repository.RiskFactor
	var err error
	maxAgeStr := ctx.Query("maxAge")

	if maxAgeStr != "" {
		maxAge, _ := strconv.Atoi(maxAgeStr)
		factors, err = h.Repository.GetRiskFactorByMaxAge(maxAge)
	} else {
		factors, err = h.Repository.GetRiskFactors()
	}

	if err != nil {
		logrus.Error(err)
	}

	var gridViews []GridView
	for _, f := range factors {
		if f.Status == "published" {
			gridViews = append(gridViews, GridView{
				ID:         f.ID,
				Name:       f.Name,
				ImageURL:   f.ImageURL,
				Status:     f.Status,
				LikesCount: len(f.LikedBy),
			})
		}
	}

	ctx.HTML(http.StatusOK, "risk_factors_grid.html", gin.H{
		"factors": gridViews,
		"maxAge":  maxAgeStr,
	})
}

func (h *Handler) CreateRiskFactor(ctx *gin.Context) {
	var factor repository.RiskFactor

	existingDraft, err := h.Repository.GetDraft()
	if err == nil {
		factor = existingDraft
	}

	factor.Name = ctx.PostForm("name")
	factor.Description = ctx.PostForm("description")

	ageStr := ctx.PostForm("age")
	if ageStr != "" {
		factor.Age, _ = strconv.Atoi(ageStr)
	}

	bmiStr := ctx.PostForm("bmi")
	if bmiStr != "" {
		factor.BMI, _ = strconv.ParseFloat(bmiStr, 64)
	}

	factor.ImageURL = "/static/image/default-photo.png"
	factor.VideoURL = "/static/image/default_video.mp4"
	factor.Status = "draft"
	factor.CreatedBy = 1

	if factor.ID == 0 {
		err = h.Repository.CreateRiskFactor(&factor)
	} else {
		err = h.Repository.UpdateRiskFactor(factor.ID, map[string]interface{}{
			"name":        factor.Name,
			"description": factor.Description,
			"age":         factor.Age,
			"bmi":         factor.BMI,
		})
	}

	if err != nil {
		logrus.Error("Ошибка создания/обновления:", err)
		ctx.String(http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	ctx.Redirect(http.StatusFound, "/risk_factors_add")
}

func (h *Handler) PublishRiskFactor(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.String(http.StatusBadRequest, "Неверный ID")
		return
	}

	// Получаем данные из формы
	description := ctx.PostForm("description")

	ageStr := ctx.PostForm("age")
	age := 0
	if ageStr != "" {
		age, _ = strconv.Atoi(ageStr)
	}

	bmiStr := ctx.PostForm("bmi")
	bmi := 0.0
	if bmiStr != "" {
		bmi, _ = strconv.ParseFloat(bmiStr, 64)
	}

	// Обновляем описание, возраст, ИМТ и меняем статус
	err = h.Repository.PublishRiskFactor(id, description, age, bmi)
	if err != nil {
		logrus.Error("Ошибка публикации:", err)
		ctx.String(http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	ctx.Redirect(http.StatusFound, "/risk_factors_feed/"+idStr)
}

func (h *Handler) DeleteRiskFactor(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.String(http.StatusBadRequest, "Неверный ID")
		return
	}

	err = h.Repository.DeleteRiskFactorSQL(id)
	if err != nil {
		logrus.Error("Ошибка удаления:", err)
		ctx.String(http.StatusInternalServerError, "Ошибка сервера")
		return
	}

	ctx.Redirect(http.StatusFound, "/risk_factors_grid")
}
