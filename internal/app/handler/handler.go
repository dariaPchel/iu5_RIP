package handler

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"web_backend/internal/app/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const jwtSecret = "super_secret_jwt_key_for_lr4_change_in_prod" // В продакшене выносить в .env

type Handler struct {
	Repository *repository.Repository
}

func NewHandler(r *repository.Repository) *Handler {
	return &Handler{Repository: r}
}

// getCurrentUserID извлекает ID пользователя из контекста Gin
func getCurrentUserID(ctx *gin.Context) uint {
	userID, exists := ctx.Get("user_id")
	if !exists {
		return 0
	}
	return userID.(uint)
}

// AuthMiddleware проверяет наличие и валидность JWT токена
func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Требуется авторизация"})
			ctx.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Неверный формат токена"})
			ctx.Abort()
			return
		}

		tokenString := parts[1]

		// 1. Проверка блеклиста в Redis
		isBlacklisted, err := h.Repository.IsBlacklisted(context.Background(), tokenString)
		if err != nil {
			logrus.Error("Ошибка проверки блеклиста Redis: ", err)
		}
		if isBlacklisted {
			ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Токен аннулирован (logout)"})
			ctx.Abort()
			return
		}

		// 2. Парсинг и валидация JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Недействительный токен"})
			ctx.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Неверные claims токена"})
			ctx.Abort()
			return
		}

		// Сохраняем ID пользователя в контексте для дальнейшего использования
		ctx.Set("user_id", uint(claims["user_id"].(float64)))
		ctx.Next()
	}
}

// OptionalAuthMiddleware проверяет токен, но не блокирует запрос, если его нет или он невалиден
// Нужно для эндпоинтов, где гость может видеть данные, а авторизованный пользователь - еще и is_liked
func (h *Handler) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader == "" {
			ctx.Next()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			tokenString := parts[1]
			isBlacklisted, _ := h.Repository.IsBlacklisted(context.Background(), tokenString)
			if !isBlacklisted {
				token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
					return []byte(jwtSecret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						ctx.Set("user_id", uint(claims["user_id"].(float64)))
					}
				}
			}
		}
		ctx.Next()
	}
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

// RegisterUser godoc
// @Summary Регистрация пользователя
// @Description Регистрирует нового пользователя с хешированием пароля
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Данные для регистрации"
// @Success 201 {object} map[string]interface{}
// @Router /api/users/register [post]
func (h *Handler) RegisterUser(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Неверные данные"})
		return
	}

	// Хеширование пароля перед сохранением
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка хеширования пароля"})
		return
	}

	user := repository.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	if err := h.Repository.CreateUser(&user); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка регистрации (возможно, email занят)"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Пользователь зарегистрирован",
		"user_id": user.ID,
	})
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginUser godoc
// @Summary Аутентификация пользователя
// @Description Проверяет учетные данные и возвращает JWT токен
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Данные для входа"
// @Success 200 {object} map[string]interface{}
// @Router /api/users/login [post]
func (h *Handler) LoginUser(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Неверные данные"})
		return
	}

	// Получаем пользователя из БД (убедись, что в repository.go есть метод GetUserByEmail)
	user, err := h.Repository.GetUserByEmail(req.Email)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Неверный email или пароль"})
		return
	}

	// Сравниваем хеши паролей
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Неверный email или пароль"})
		return
	}

	// Генерация JWT
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"exp":     expirationTime.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Ошибка генерации токена"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":       "success",
		"message":      "Аутентификация успешна",
		"access_token": tokenString,
		"expires_in":   86400,
	})
}

// LogoutUser godoc
// @Summary Выход пользователя
// @Description Добавляет текущий JWT токен в блеклист Redis
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/users/logout [post]
func (h *Handler) LogoutUser(ctx *gin.Context) {
	authHeader := ctx.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && parts[0] == "Bearer" {
		// Добавляем токен в блеклист на 24 часа (время жизни токена)
		h.Repository.AddToBlacklist(context.Background(), parts[1], 24*time.Hour)
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Деавторизация успешна"})
}

// GetRiskFactors godoc
// @Summary Получение списка факторов риска
// @Description Возвращает список опубликованных факторов риска с опциональным фильтром по возрасту
// @Tags RiskFactors
// @Accept json
// @Produce json
// @Param maxAge query int false "Максимальный возраст"
// @Success 200 {object} map[string]interface{}
// @Router /api/risk_factors [get]
func (h *Handler) GetRiskFactors(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx) // Заменяет utils.GetCurrentUserID()
	var maxAge *int
	if maxAgeStr := ctx.Query("maxAge"); maxAgeStr != "" {
		if age, err := strconv.Atoi(maxAgeStr); err == nil {
			maxAge = &age
		}
	}

	factors, err := h.Repository.GetRiskFactors(maxAge)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка получения списка"})
		return
	}

	var result []gin.H
	for _, f := range factors {
		isCreator := 0
		if f.CreatedBy == currentUserID {
			isCreator = 1
		}

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
			"is_liked":    isLiked,
		})
	}
	ctx.JSON(http.StatusOK, gin.H{"data": result})
}

// GetFeed godoc
// @Summary Получение ленты факторов риска
// @Description Возвращает один фактор риска для ленты (первый, следующий или предыдущий)
// @Tags RiskFactors
// @Accept json
// @Produce json
// @Param id path int false "ID фактора риска"
// @Param next query bool false "Получить следующий"
// @Param prev query bool false "Получить предыдущий"
// @Success 200 {object} map[string]interface{}
// @Router /api/risk_factors/feed [get]
// @Router /api/risk_factors/feed/{id} [get]
func (h *Handler) GetFeed(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx)
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

// GetDraft godoc
// @Summary Получение черновика
// @Description Возвращает черновик фактора риска текущего пользователя
// @Tags RiskFactors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/risk_factors/draft [get]
func (h *Handler) GetDraft(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx)
	if currentUserID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Требуется авторизация"})
		return
	}

	factor, err := h.Repository.GetDraft(currentUserID)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": nil})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "data": factor})
}

// CreateRiskFactor godoc
// @Summary Создание или обновление фактора риска
// @Description Создает новый фактор риска или обновляет существующий черновик (с загрузкой файлов)
// @Tags RiskFactors
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /api/risk_factors [post]
func (h *Handler) CreateRiskFactor(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx)
	if currentUserID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Требуется авторизация"})
		return
	}

	err := ctx.Request.ParseMultipartForm(10 << 20)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Ошибка разбора формы"})
		return
	}

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

// PublishRiskFactor godoc
// @Summary Публикация фактора риска
// @Description Публикует черновик фактора риска (доступно только создателю)
// @Tags RiskFactors
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "ID фактора риска"
// @Success 200 {object} map[string]interface{}
// @Router /api/risk_factors/{id}/publish [put]
func (h *Handler) PublishRiskFactor(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx)
	if currentUserID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Требуется авторизация"})
		return
	}

	id, _ := strconv.Atoi(ctx.Param("id"))
	description := ctx.PostForm("description")
	age := 0
	if ageStr := ctx.PostForm("age"); ageStr != "" {
		age, _ = strconv.Atoi(ageStr)
	}
	bmi := 0.0
	if bmiStr := ctx.PostForm("bmi"); bmiStr != "" {
		bmi, _ = strconv.ParseFloat(bmiStr, 64)
	}

	err := h.Repository.PublishRiskFactor(id, currentUserID, description, age, bmi)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"status": "error", "message": "Нет прав на публикацию или услуга не найдена"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "message": "Услуга опубликована"})
}

// DeleteRiskFactor godoc
// @Summary Удаление фактора риска
// @Description Мягкое удаление фактора риска (доступно только создателю)
// @Tags RiskFactors
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID фактора риска"
// @Success 200 {object} map[string]interface{}
// @Router /api/risk_factors/{id} [delete]
func (h *Handler) DeleteRiskFactor(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx)
	if currentUserID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Требуется авторизация"})
		return
	}

	id, _ := strconv.Atoi(ctx.Param("id"))
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

// LikeRiskFactor godoc
// @Summary Лайк фактора риска
// @Description Добавляет или удаляет лайк у фактора риска
// @Tags RiskFactors
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID фактора риска"
// @Param request body LikeRequest true "Действие (1 - лайк, 0 - удалить лайк)"
// @Success 200 {object} map[string]interface{}
// @Router /api/risk_factors/{id}/like [post]
func (h *Handler) LikeRiskFactor(ctx *gin.Context) {
	currentUserID := getCurrentUserID(ctx)
	if currentUserID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Требуется авторизация"})
		return
	}

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
