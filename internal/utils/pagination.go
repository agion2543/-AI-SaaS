package utils

func NormalizePage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	// Controllers and services calculate (page-1)*pageSize. Keep that
	// multiplication within int even when a query supplies an extreme page.
	maxInt := int(^uint(0) >> 1)
	if maxPage := maxInt / pageSize; page > maxPage {
		page = maxPage
	}
	return page, pageSize
}
