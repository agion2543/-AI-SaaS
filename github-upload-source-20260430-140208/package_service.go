package service

import (
	"gorm.io/gorm"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
)

type PackageService struct {
	repo *repository.PackageRepository
}

func NewPackageService(db *gorm.DB) *PackageService {
	return &PackageService{repo: repository.NewPackageRepository(db)}
}

func (s *PackageService) ListPublished() ([]model.MembershipPackage, error) {
	return s.repo.ListPublished()
}

func (s *PackageService) ListAll() ([]model.MembershipPackage, error) {
	return s.repo.ListAll()
}

func (s *PackageService) Save(id uint, req dto.SavePackageRequest) (*model.MembershipPackage, error) {
	pkg := &model.MembershipPackage{}
	if id > 0 {
		exist, err := s.repo.FindByID(id)
		if err != nil {
			return nil, err
		}
		pkg = exist
	}

	pkg.Name = req.Name
	pkg.Code = req.Code
	pkg.DurationDays = req.DurationDays
	pkg.Price = req.Price
	pkg.OriginalPrice = req.OriginalPrice
	pkg.Quota = req.Quota
	pkg.Sort = req.Sort
	pkg.Status = req.Status
	pkg.IsLifetime = req.IsLifetime
	pkg.Description = req.Description

	return pkg, s.repo.Save(pkg)
}
