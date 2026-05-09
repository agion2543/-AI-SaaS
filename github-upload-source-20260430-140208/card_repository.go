package repository

import (
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type CardRepository struct {
	db *gorm.DB
}

func NewCardRepository(db *gorm.DB) *CardRepository {
	return &CardRepository{db: db}
}

func (r *CardRepository) CreateBatch(cards []model.CardCode) error {
	return r.db.Create(&cards).Error
}

func (r *CardRepository) FindByCode(code string) (*model.CardCode, error) {
	var card model.CardCode
	err := r.db.Preload("Package").Where("code = ?", code).First(&card).Error
	return &card, err
}

func (r *CardRepository) Save(card *model.CardCode) error {
	return r.db.Save(card).Error
}

func (r *CardRepository) ListAll() ([]model.CardCode, error) {
	var cards []model.CardCode
	err := r.db.Preload("Package").Order("id desc").Find(&cards).Error
	return cards, err
}
