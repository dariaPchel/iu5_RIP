package repository

import (
	"fmt"
)

type Repository struct {
	MinIOURL string
}

func NewRepository() (*Repository, error) {
	return &Repository{
		MinIOURL: "http://localhost:9000/pospom-media",
	}, nil
}

type RiskFactor struct {
	ID             int
	Name           string
	Description    string
	Age            int
	BMI            float64
	Comorbidity    string
	SurgeryType    string
	RiskPercentage float64
	ImageKey       string
	VideoKey       string
	ImageURL       string
	VideoURL       string
	Status         string
	LikedBy        []string
}

func (r *Repository) GetRiskFactors() ([]RiskFactor, error) {
	factors := []RiskFactor{
		{
			ID:             1,
			Name:           "Возраст старше 70 лет",
			Description:    "Валидация POSPOM в Германии показала: каждый год после 70 лет линейно увеличивает риск внутрибольничной смертности. При кардиохирургии этот эффект усиливается снижением компенсаторных возможностей организма.",
			Age:            74,
			BMI:            24.5,
			SurgeryType:    "Протезирование аортального клапана",
			RiskPercentage: 15.14,
			ImageKey:       "heart-surgery.jpg",
			VideoKey:       "surgery-video.mp4",
			ImageURL:       r.MinIOURL + "/heart-surgery.jpg",
			VideoURL:       r.MinIOURL + "/surgery-video.mp4",
			Status:         "published",
			LikedBy:        []string{"user_101", "user_205", "user_333", "user_412", "user_589"}, // Пример ID
		},
		{
			ID:             2,
			Name:           "ХОБЛ + Сердечная недостаточность",
			Description:    "Валидация POSPOM в Германии показала: сочетание этих 2 заболеваний из 15 критериев критически повышает риск. ХОБЛ и сердечная недостаточность вместе создают дополнительную нагрузку на организм.",
			Age:            34,
			BMI:            29.1,
			Comorbidity:    "J44 (ХОБЛ) + I50 (СН)",
			SurgeryType:    "Аортокоронарное шунтирование",
			RiskPercentage: 12.8,
			ImageKey:       "lungs-copd.jpg",
			VideoKey:       "lungs.mp4",
			ImageURL:       r.MinIOURL + "/lungs-copd.jpg",
			VideoURL:       r.MinIOURL + "/lungs.mp4",
			Status:         "draft",
			LikedBy:        []string{"user_101", "user_205", "user_333"},
		},
		{
			ID:             3,
			Name:           "Цирроз печени",
			Description:    "Хроническое заболевание печени повышает риск кровотечений и инфекций. Цирроз нарушает синтез белков свертывания крови и снижает иммунную защиту организма.",
			Age:            23,
			BMI:            26.3,
			Comorbidity:    "K74 (Цирроз)",
			SurgeryType:    "Трансплантация печени",
			RiskPercentage: 22.4,
			ImageKey:       "liver-cirrhosis.jpg",
			VideoKey:       "liver.mp4",
			ImageURL:       r.MinIOURL + "/liver-cirrhosis.jpg",
			VideoURL:       r.MinIOURL + "/liver.mp4",
			Status:         "deleted",
			LikedBy:        []string{"user_101"},
		},
	}
	return factors, nil
}

func (r *Repository) GetRiskFactor(id int) (RiskFactor, error) {
	factors, err := r.GetRiskFactors()
	if err != nil {
		return RiskFactor{}, err
	}
	for _, factor := range factors {
		if factor.ID == id {
			return factor, nil
		}
	}
	return RiskFactor{}, fmt.Errorf("Фактор не найден")
}

func (r *Repository) GetNextRiskFactor(currentID int) (RiskFactor, error) {
	factors, err := r.GetRiskFactors()
	if err != nil {
		return RiskFactor{}, err
	}

	currentIndex := -1
	for i, factor := range factors {
		if factor.ID == currentID {
			currentIndex = i
			break
		}
	}

	for i := 1; i <= len(factors); i++ {
		index := (currentIndex + i) % len(factors)
		if factors[index].Status != "deleted" {
			return factors[index], nil
		}
	}

	return factors[0], nil
}

func (r *Repository) GetPrevRiskFactor(currentID int) (RiskFactor, error) {
	factors, err := r.GetRiskFactors()
	if err != nil {
		return RiskFactor{}, err
	}

	currentIndex := -1
	for i, factor := range factors {
		if factor.ID == currentID {
			currentIndex = i
			break
		}
	}

	for i := 1; i <= len(factors); i++ {
		index := (currentIndex - i + len(factors)) % len(factors)
		if factors[index].Status != "deleted" {
			return factors[index], nil
		}
	}

	return factors[0], nil
}

func (r *Repository) GetRiskFactorByMaxAge(maxAge int) ([]RiskFactor, error) {
	factors, err := r.GetRiskFactors()
	if err != nil {
		return []RiskFactor{}, err
	}
	var result []RiskFactor
	for _, factor := range factors {
		if factor.Age <= maxAge {
			result = append(result, factor)
		}
	}
	return result, nil
}

func (r *Repository) GetDraft() (RiskFactor, error) {
	factors, err := r.GetRiskFactors()
	if err != nil {
		return RiskFactor{}, err
	}
	for _, factor := range factors {
		if factor.Status == "draft" {
			return factor, nil
		}
	}
	return RiskFactor{}, fmt.Errorf("Черновик не найден")
}
