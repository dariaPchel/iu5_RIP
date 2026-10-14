package handler

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"web_backend/internal/app/repository"
	"web_backend/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

func (h *Handler) uploadToMinio(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	ext := filepath.Ext(fileHeader.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = h.Repository.MinioClient.PutObject(
		context.Background(),
		h.Repository.MinioBucket,
		filename,
		file,
		fileHeader.Size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", err
	}
	return filename, nil
}

func (h *Handler) GetRiskFactors(ctx *gin.Context) {
	currentUserID := utils.GetCurrentUserID()
	var maxAge *int
	if maxAgeStr := ctx.Query("maxAge"); maxAgeStr != "" {
		if age, err := strconv.Atoi(maxAgeStr); err == nil {
			maxAge = &age
		}
	}

	factors, err := h.Repository.GetRiskFactors(maxAge)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка получения списка"})
		return
	}

	// Формируем ответ, добавляя признаки is_creator и is_liked
	var result []gin.H
	for _, f := range factors {
		isCreator := 0
		if f.CreatedBy == currentUserID {
			isCreator = 1
		}

		// Проверяем, лайкнул ли текущий пользователь
		isLiked := 0
		for _, user := range f.LikedBy {
			if user.ID == currentUserID {
				isLiked = 1
				break
			}
		}

		result = append(result, gin.H{
			"id":          f.ID,
			"name":        f.Name,
			"image_url":   f.ImageURL,
			"likes_count": len(f.LikedBy),
			"is_creator":  isCreator,
			"is_liked":    isLiked, // ← ДОБАВЛЕНО
		})
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": result})
}

// GET /api/risk_factors/feed и /api/risk_factors/feed/:id
func (h *Handler) GetFeed(ctx *gin.Context) {
	currentUserID := utils.GetCurrentUserID()
	idStr := ctx.Param("id")
	var factor repository.RiskFactor
	var err error

	if idStr == "" {
		factors, _ := h.Repository.GetRiskFactors(nil)
		if len(factors) > 0 {
			factor = factors[0]
		} else {
			ctx.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Лента пуста"})
			return
		}
	} else {
		id, convErr := strconv.Atoi(idStr)
		if convErr != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Неверный ID"})
			return
		}
		if ctx.Query("next") == "true" {
			factor, err = h.Repository.GetNextRiskFactor(id)
		} else if ctx.Query("prev") == "true" {
			factor, err = h.Repository.GetPrevRiskFactor(id)
		} else {
			factor, err = h.Repository.GetRiskFactor(id)
		}
	}

	if err != nil || factor.Status != "published" {
		ctx.JSON(http.StatusNotFound, gin.H{"status": "error", "message": "Фактор риска не найден"})
		return
	}

	isCreator := 0
	if factor.CreatedBy == currentUserID {
		isCreator = 1
	}

	// Проверяем, лайкнул ли текущий пользователь эту карточку
	isLiked := 0
	for _, user := range factor.LikedBy {
		if user.ID == currentUserID {
			isLiked = 1
			break
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data": gin.H{
			"id":              factor.ID,
			"name":            factor.Name,
			"description":     factor.Description,
			"age":             factor.Age,
			"bmi":             factor.BMI,
			"risk_percentage": factor.RiskPercentage,
			"image_url":       factor.ImageURL,
			"video_url":       factor.VideoURL,
			"likes_count":     len(factor.LikedBy),
			"is_creator":      isCreator,
			"is_liked":        isLiked,
			"published_at":    factor.PublishedAt,
		},
	})
}

func (h *Handler) GetDraft(ctx *gin.Context) {
	currentUserID := utils.GetCurrentUserID()
	factor, err := h.Repository.GetDraft(currentUserID) // Передаем userID
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": nil})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": factor})
}

func (h *Handler) CreateRiskFactor(ctx *gin.Context) {
	err := ctx.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Ошибка разбора формы"})
		return
	}

	currentUserID := utils.GetCurrentUserID()
	existingDraft, err := h.Repository.GetDraft(currentUserID)
	var factor repository.RiskFactor

	if err == nil {
		factor = existingDraft
	} else {
		factor = repository.RiskFactor{
			Status:    "draft",
			CreatedBy: currentUserID,
		}
	}

	factor.Name = ctx.PostForm("name")
	factor.Description = ctx.PostForm("description")
	if ageStr := ctx.PostForm("age"); ageStr != "" {
		factor.Age, _ = strconv.Atoi(ageStr)
	}
	if bmiStr := ctx.PostForm("bmi"); bmiStr != "" {
		factor.BMI, _ = strconv.ParseFloat(bmiStr, 64)
	}
	if riskStr := ctx.PostForm("risk_percentage"); riskStr != "" {
		factor.RiskPercentage, _ = strconv.ParseFloat(riskStr, 64)
	}

	// Загрузка файлов в MinIO
	if imgFile, err := ctx.FormFile("image"); err == nil {
		if filename, err := h.uploadToMinio(imgFile); err == nil {
			factor.ImageURL = filename
		}
	}
	if videoFile, err := ctx.FormFile("video"); err == nil {
		if filename, err := h.uploadToMinio(videoFile); err == nil {
			factor.VideoURL = filename
		}
	}

	if factor.ID == 0 {
		err = h.Repository.CreateRiskFactor(&factor)
	} else {
		updates := map[string]interface{}{
			"name": factor.Name, "description": factor.Description, "age": factor.Age,
			"bmi": factor.BMI, "risk_percentage": factor.RiskPercentage,
		}
		if factor.ImageURL != "" {
			updates["image_url"] = factor.ImageURL
		}
		if factor.VideoURL != "" {
			updates["video_url"] = factor.VideoURL
		}
		err = h.Repository.UpdateRiskFactor(factor.ID, updates)
	}

	if err != nil {
		logrus.Error("Ошибка создания/обновления:", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка сервера"})
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Услуга сохранена", "data": factor})
}

func (h *Handler) PublishRiskFactor(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	currentUserID := utils.GetCurrentUserID()

	// Получаем данные из формы, которые пользователь ввел на 2-м шаге
	description := ctx.PostForm("description")
	age := 0
	if ageStr := ctx.PostForm("age"); ageStr != "" {
		age, _ = strconv.Atoi(ageStr)
	}
	bmi := 0.0
	if bmiStr := ctx.PostForm("bmi"); bmiStr != "" {
		bmi, _ = strconv.ParseFloat(bmiStr, 64)
	}

	// Передаем всё в репозиторий
	err := h.Repository.PublishRiskFactor(id, currentUserID, description, age, bmi)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Нет прав на публикацию или услуга не найдена"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Услуга опубликована"})
}

func (h *Handler) DeleteRiskFactor(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	currentUserID := utils.GetCurrentUserID()

	// Передаем userID для проверки прав!
	err := h.Repository.DeleteRiskFactorSQL(id, currentUserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Нет прав на удаление или услуга не найдена"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Услуга удалена"})
}

type LikeRequest struct {
	Action int `json:"action" binding:"oneof=0 1"`
}

func (h *Handler) LikeRiskFactor(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Неверный ID"})
		return
	}

	var req LikeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка JSON: " + err.Error(),
		})
		return
	}

	currentUserID := utils.GetCurrentUserID()
	if req.Action == 1 {
		err = h.Repository.AddLike(uint(id), currentUserID)
	} else {
		err = h.Repository.RemoveLike(uint(id), currentUserID)
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка БД: " + err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Лайк обновлен"})
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

func (h *Handler) RegisterUser(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Неверные данные"})
		return
	}
	user := repository.User{Name: req.Name, Email: req.Email, Password: req.Password}
	if err := h.Repository.CreateUser(&user); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка регистрации"})
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"status": "success", "message": "Пользователь зарегистрирован", "user_id": user.ID})
}

func (h *Handler) LoginUser(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Аутентификация успешна (заглушка для ЛР 4)"})
}

func (h *Handler) LogoutUser(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Деавторизация успешна (заглушка для ЛР 4)"})
}
