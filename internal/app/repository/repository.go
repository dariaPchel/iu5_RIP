package repository

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db          *gorm.DB
	MinioClient *minio.Client
	MinioBucket string
}

func NewRepository() (*Repository, error) {
	dsn := "host=localhost user=postgres password=postgres1234 dbname=pospom_db port=5432 sslmode=disable TimeZone=Europe/Moscow"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Автоматическая миграция таблиц
	err = db.AutoMigrate(&User{}, &RiskFactor{}, &Like{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 2. Подключение к MinIO (Обязательно для ЛР 3)
	minioClient, err := minio.New("localhost:9000", &minio.Options{
		Creds:  credentials.NewStaticV4("admin", "admin123", ""), // Ваши ключи MinIO
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to minio: %w", err)
	}

	ctx := context.Background()
	bucketName := "pospom-media"
	err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errExist := minioClient.BucketExists(ctx, bucketName)
		if errExist != nil || !exists {
			return nil, fmt.Errorf("failed to create/check minio bucket: %w", err)
		}
	}

	return &Repository{
		db:          db,
		MinioClient: minioClient,
		MinioBucket: bucketName,
	}, nil
}

type User struct {
	ID               uint         `gorm:"primaryKey"`
	Name             string       `gorm:"size:100;not null"`
	Email            string       `gorm:"size:255;unique;not null"`
	Password         string       `gorm:"size:128;not null"`
	LikedRiskFactors []RiskFactor `gorm:"many2many:likes;"` // Исправлено для корректной связи many2many
}

type RiskFactor struct {
	ID             uint   `gorm:"primaryKey"`
	Name           string `gorm:"size:255;not null"`
	Description    string `gorm:"type:text"`
	Status         string `gorm:"size:20;not null;default:'draft'"`
	ImageURL       string `gorm:"size:500"`
	VideoURL       string `gorm:"size:500"`
	Age            int
	BMI            float64
	RiskPercentage float64
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	CreatedBy      uint
	PublishedAt    *time.Time
	LikedBy        []User `gorm:"many2many:likes;"`
}

type Like struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       uint      `gorm:"not null"`
	RiskFactorID uint      `gorm:"not null"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

// GetRiskFactors: только опубликованные и не удаленные, с опциональным фильтром по возрасту
func (r *Repository) GetRiskFactors(maxAge *int) ([]RiskFactor, error) {
	var factors []RiskFactor
	query := r.db.Preload("LikedBy").Where("status = ? AND status != ?", "published", "deleted")
	if maxAge != nil {
		query = query.Where("age <= ?", *maxAge)
	}
	err := query.Find(&factors).Error
	return factors, err
}

// GetRiskFactor: только если не удален
func (r *Repository) GetRiskFactor(id int) (RiskFactor, error) {
	var factor RiskFactor
	err := r.db.Preload("LikedBy").Where("status != ?", "deleted").First(&factor, id).Error
	return factor, err
}

// GetNextRiskFactor: только опубликованные и не удаленные
func (r *Repository) GetNextRiskFactor(currentID int) (RiskFactor, error) {
	var factor RiskFactor
	err := r.db.Preload("LikedBy").
		Where("id > ? AND status = ? AND status != ?", currentID, "published", "deleted").
		Order("id ASC").
		First(&factor).Error

	if err != nil {
		err = r.db.Preload("LikedBy").
			Where("status = ? AND status != ?", "published", "deleted").
			Order("id ASC").
			First(&factor).Error
	}
	return factor, err
}

// GetPrevRiskFactor: только опубликованные и не удаленные
func (r *Repository) GetPrevRiskFactor(currentID int) (RiskFactor, error) {
	var factor RiskFactor
	err := r.db.Preload("LikedBy").
		Where("id < ? AND status = ? AND status != ?", currentID, "published", "deleted").
		Order("id DESC").
		First(&factor).Error

	if err != nil {
		err = r.db.Preload("LikedBy").
			Where("status = ? AND status != ?", "published", "deleted").
			Order("id DESC").
			First(&factor).Error
	}
	return factor, err
}

// GetDraft: только черновик ТЕКУЩЕГО пользователя (Требование ЛР 3)
func (r *Repository) GetDraft(userID uint) (RiskFactor, error) {
	var factor RiskFactor
	err := r.db.Preload("LikedBy").Where("status = ? AND created_by = ?", "draft", userID).First(&factor).Error
	return factor, err
}

func (r *Repository) CreateRiskFactor(factor *RiskFactor) error {
	return r.db.Create(factor).Error
}

func (r *Repository) UpdateRiskFactor(id uint, updates map[string]interface{}) error {
	return r.db.Model(&RiskFactor{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteRiskFactorSQL: мягкое удаление ТОЛЬКО для создателя (Требование ЛР 3)
func (r *Repository) DeleteRiskFactorSQL(id int, userID uint) error {
	return r.db.Model(&RiskFactor{}).Where("id = ? AND created_by = ?", id, userID).Update("status", "deleted").Error
}

// PublishRiskFactor: публикация ТОЛЬКО для создателя (Требование ЛР 3)
// Оставляем ваши поля description, age, bmi, но добавляем проверку userID
func (r *Repository) PublishRiskFactor(id int, userID uint, description string, age int, bmi float64) error {
	now := time.Now()

	result := r.db.Model(&RiskFactor{}).
		Where("id = ? AND created_by = ? AND status = ?", id, userID, "draft").
		Updates(map[string]interface{}{
			"status":       "published",
			"description":  description,
			"age":          age,
			"bmi":          bmi,
			"published_at": &now, // Если ты добавила это поле в структуру
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("услуга не найдена, уже опубликована или нет прав")
	}

	return nil
}

func (r *Repository) AddLike(riskFactorID uint, userID uint) error {
	var count int64
	r.db.Model(&Like{}).Where("risk_factor_id = ? AND user_id = ?", riskFactorID, userID).Count(&count)
	if count > 0 {
		return nil
	}
	return r.db.Create(&Like{UserID: userID, RiskFactorID: riskFactorID}).Error
}

func (r *Repository) RemoveLike(riskFactorID uint, userID uint) error {
	return r.db.Where("risk_factor_id = ? AND user_id = ?", riskFactorID, userID).Delete(&Like{}).Error
}

func (r *Repository) CreateUser(user *User) error {
	return r.db.Create(user).Error
}

// UploadToMinio: загрузка файла и возврат имени (Требование ЛР 3)
func (r *Repository) UploadToMinio(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer file.Close()

	ext := filepath.Ext(fileHeader.Filename)
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = r.MinioClient.PutObject(
		context.Background(),
		r.MinioBucket,
		filename,
		file,
		fileHeader.Size,
		minio.PutObjectOptions{ContentType: contentType},
	)
	if err != nil {
		return "", fmt.Errorf("ошибка загрузки в MinIO: %w", err)
	}
	return filename, nil
}
