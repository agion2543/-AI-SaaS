package repository

import (
	"go-web-gin-health/internal/model"
	"gorm.io/gorm"
)

type PackageRepository struct {
	db *gorm.DB
}

func NewPackageRepository(db *gorm.DB) *PackageRepository {
	return &PackageRepository{db: db}
}

func (r *PackageRepository) ListPublished() ([]model.MembershipPackage, error) {
	var packages []model.MembershipPackage
	err := r.db.Where("status = ?", "published").Order("sort asc, id asc").Find(&packages).Error
	return packages, err
}

func (r *PackageRepository) ListAll() ([]model.MembershipPackage, error) {
	var packages []model.MembershipPackage
	err := r.db.Order("sort asc, id asc").Find(&packages).Error
	return packages, err
}

func (r *PackageRepository) FindByID(id uint) (*model.MembershipPackage, error) {
	var pkg model.MembershipPackage
	err := r.db.First(&pkg, id).Error
	return &pkg, err
}

func (r *PackageRepository) Save(pkg *model.MembershipPackage) error {
	return r.db.Save(pkg).Error
}
