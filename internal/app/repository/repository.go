package repository

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Repository struct {
	db       *gorm.DB
	MinIOURL string
}

func NewRepository() (*Repository, error) {
	dsn := "host=localhost user=postgres password=postgres1234 dbname=pospom_db port=5432 sslmode=disable TimeZone=Europe/Moscow"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	db.AutoMigrate(&User{}, &RiskFactor{}, &Like{})
	return &Repository{
		db:       db,
		MinIOURL: "http://localhost:9000/pospom-media",
	}, nil
}

type User struct {
	ID       uint         `gorm:"primaryKey"`
	Name     string       `gorm:"size:100;not null"`
	Email    string       `gorm:"size:255;unique;not null"`
	Password string       `gorm:"size:128;not null"`
	Services []RiskFactor `gorm:"many2many:likes;"`
}

type RiskFactor struct {
	ID             uint   `gorm:"primaryKey"`
	Name           string `gorm:"size:255;not null"`
	Description    string
	Status         string `gorm:"size:20;not null;default:'draft'"`
	ImageURL       string `gorm:"size:500"`
	VideoURL       string `gorm:"size:500"`
	Age            int
	BMI            float64
	RiskPercentage float64
	CreatedAt      time.Time
	PublishedAt    *time.Time
	CreatedBy      uint
	LikedBy        []User `gorm:"many2many:likes;"`
}

type Like struct {
	ID           uint `gorm:"primaryKey"`
	UserID       uint `gorm:"not null"`
	RiskFactorID uint `gorm:"not null"`
	CreatedAt    time.Time
}

func (r *Repository) GetRiskFactors() ([]RiskFactor, error) {
	var factors []RiskFactor
	err := r.db.Preload("LikedBy").Find(&factors).Error
	return factors, err
}

func (r *Repository) GetRiskFactor(id int) (RiskFactor, error) {
	var factor RiskFactor
	err := r.db.Preload("LikedBy").First(&factor, id).Error
	return factor, err
}

func (r *Repository) GetNextRiskFactor(currentID int) (RiskFactor, error) {
	var factor RiskFactor

	err := r.db.Preload("LikedBy").
		Where("id > ? AND status = ?", currentID, "published").
		Order("id ASC").
		First(&factor).Error

	if err != nil {
		err = r.db.Preload("LikedBy").
			Where("status = ?", "published").
			Order("id ASC").
			First(&factor).Error
	}
	return factor, err
}

func (r *Repository) GetPrevRiskFactor(currentID int) (RiskFactor, error) {
	var factor RiskFactor

	err := r.db.Preload("LikedBy").
		Where("id < ? AND status = ?", currentID, "published").
		Order("id DESC").
		First(&factor).Error

	if err != nil {
		err = r.db.Preload("LikedBy").
			Where("status = ?", "published").
			Order("id DESC").
			First(&factor).Error
	}
	return factor, err
}

func (r *Repository) GetRiskFactorByMaxAge(maxAge int) ([]RiskFactor, error) {
	var factors []RiskFactor
	err := r.db.Preload("LikedBy").Where("age <= ?", maxAge).Find(&factors).Error
	return factors, err
}

func (r *Repository) GetDraft() (RiskFactor, error) {
	var factor RiskFactor
	err := r.db.Preload("LikedBy").Where("status = ?", "draft").First(&factor).Error
	return factor, err
}

func (r *Repository) CreateRiskFactor(factor *RiskFactor) error {
	return r.db.Create(factor).Error
}

func (r *Repository) UpdateRiskFactor(id uint, updates map[string]interface{}) error {
	return r.db.Model(&RiskFactor{}).Where("id = ?", id).Updates(updates).Error
}

func (r *Repository) DeleteRiskFactorSQL(id int) error {
	return r.db.Exec("UPDATE risk_factors SET status = 'deleted' WHERE id = ?", id).Error
}

func (r *Repository) PublishRiskFactor(id int, description string, age int, bmi float64) error {
	now := time.Now()
	return r.db.Model(&RiskFactor{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       "published",
		"description":  description,
		"age":          age,
		"bmi":          bmi,
		"published_at": now, // ← Записываем дату публикации
	}).Error
}
