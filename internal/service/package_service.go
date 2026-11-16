package service

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-web-gin-health/internal/dto"
	"go-web-gin-health/internal/model"
	"go-web-gin-health/internal/repository"
)

type PackageService struct {
	db   *gorm.DB
	repo *repository.PackageRepository
}

func NewPackageService(db *gorm.DB) *PackageService {
	return &PackageService{db: db, repo: repository.NewPackageRepository(db)}
}

func (s *PackageService) ListPublished() ([]model.MembershipPackage, error) {
	return s.repo.ListPublished()
}

func (s *PackageService) ListAll() ([]model.MembershipPackage, error) {
	return s.repo.ListAll()
}

func (s *PackageService) Save(id uint, req dto.SavePackageRequest, actor AuditActor) (*model.MembershipPackage, error) {
	var saved model.MembershipPackage
	err := s.db.Transaction(func(tx *gorm.DB) error {
		pkg := &model.MembershipPackage{}
		var before *membershipPackageAuditSnapshot
		if id > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(pkg, id).Error; err != nil {
				return err
			}
			previous := membershipPackageSnapshot(*pkg)
			before = &previous
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
		if err := tx.Save(pkg).Error; err != nil {
			return err
		}
		action := "membership_package_create"
		if id > 0 {
			action = "membership_package_update"
		}
		if err := appendPlanPackageAuditTx(tx, actor, action, "membership_package", pkg.ID, before, membershipPackageSnapshot(*pkg)); err != nil {
			return err
		}
		saved = *pkg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}
