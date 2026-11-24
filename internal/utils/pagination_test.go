package utils

import (
	"fmt"
	"testing"
)

func TestNormalizePageKeepsOffsetMultiplicationInRange(t *testing.T) {
	maxInt := int(^uint(0) >> 1)

	for _, pageSize := range []int{1, 10, 100} {
		t.Run(fmt.Sprintf("page_size_%d", pageSize), func(t *testing.T) {
			page, normalizedPageSize := NormalizePage(maxInt, pageSize)
			if normalizedPageSize != pageSize {
				t.Fatalf("page size = %d, want %d", normalizedPageSize, pageSize)
			}
			if page > maxInt/pageSize {
				t.Fatalf("normalized page %d would overflow (page-1)*pageSize for size %d", page, pageSize)
			}
			offset := (page - 1) * normalizedPageSize
			if offset < 0 {
				t.Fatalf("normalized offset overflowed to %d", offset)
			}
		})
	}
}

func TestNormalizePagePreservesOrdinaryValuesAndDefaults(t *testing.T) {
	if page, pageSize := NormalizePage(3, 25); page != 3 || pageSize != 25 {
		t.Fatalf("NormalizePage(3, 25) = (%d, %d), want (3, 25)", page, pageSize)
	}
	if page, pageSize := NormalizePage(0, 101); page != 1 || pageSize != 10 {
		t.Fatalf("NormalizePage(0, 101) = (%d, %d), want (1, 10)", page, pageSize)
	}
}
