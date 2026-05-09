package repository

import (
	"strings"

	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByEmailOrPhone(account string) (*model.User, error) {
	return r.FindByIdentifier(account)
}

func (r *UserRepository) FindByIdentifier(account string) (*model.User, error) {
	var user model.User
	err := r.db.Where(
		"display_name = ? OR (email = ? AND email NOT LIKE ?) OR phone = ?",
		account,
		account,
		"%@pending.local",
		account,
	).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByDisplayName(displayName string) (*model.User, error) {
	var user model.User
	err := r.db.Where("display_name = ?", displayName).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByEmail(email string) (*model.User, error) {
	var user model.User
	err := r.db.Where("email = ? AND email NOT LIKE ?", email, "%@pending.local").First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByPhone(phone string) (*model.User, error) {
	var user model.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) Save(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *UserRepository) List(offset, limit int) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	r.db.Model(&model.User{}).Count(&total)
	err := r.db.Order("id desc").Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

type UserListFilter struct {
	Keyword     string
	MemberLevel string
	Status      string
	Role        string
	SortBy      string
	SortOrder   string
}

func (r *UserRepository) ListFiltered(filter UserListFilter, offset, limit int) ([]model.User, int64, error) {
	var users []model.User
	var total int64
	query := r.db.Model(&model.User{})

	keyword := strings.TrimSpace(filter.Keyword)
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("display_name LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	if filter.MemberLevel != "" {
		query = query.Where("member_level = ?", filter.MemberLevel)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortBy := map[string]string{
		"id":              "id",
		"created_at":      "created_at",
		"expired_at":      "expired_at",
		"remaining_quota": "remaining_quota",
		"member_level":    "member_level",
	}[filter.SortBy]
	if sortBy == "" {
		sortBy = "id"
	}
	sortOrder := "desc"
	if strings.EqualFold(filter.SortOrder, "asc") {
		sortOrder = "asc"
	}

	err := query.Order(sortBy + " " + sortOrder).Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}
