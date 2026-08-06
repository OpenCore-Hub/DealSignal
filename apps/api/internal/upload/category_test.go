package upload

import (
	"errors"
	"testing"
)

func TestNormalizeCreateCategory(t *testing.T) {
	cases := map[string]string{
		"":             CategoryGeneral,
		"GENERAL":      CategoryGeneral,
		"agreement":    CategoryAgreement,
		"deal_room":    CategoryGeneral,
		"Deal_Room":    CategoryGeneral,
		"uploaded":     CategoryGeneral,
		"  agreement ": CategoryAgreement,
	}
	for in, want := range cases {
		if got := NormalizeCreateCategory(in); got != want {
			t.Fatalf("NormalizeCreateCategory(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestValidateCreateCategory(t *testing.T) {
	if err := ValidateCreateCategory("general"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCreateCategory("agreement"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCreateCategory("deal_room"); !errors.Is(err, ErrCategoryDealRoomViaAPI) {
		t.Fatalf("got %v", err)
	}
	if err := ValidateCreateCategory(" Deal_Room "); !errors.Is(err, ErrCategoryDealRoomViaAPI) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateManualCategoryChange(t *testing.T) {
	if err := ValidateManualCategoryChange(CategoryGeneral, CategoryAgreement, 0); err != nil {
		t.Fatal(err)
	}
	if err := ValidateManualCategoryChange(CategoryDealRoom, CategoryAgreement, 0); !errors.Is(err, ErrCategoryImmutable) {
		t.Fatalf("got %v", err)
	}
	if err := ValidateManualCategoryChange(CategoryGeneral, CategoryDealRoom, 0); !errors.Is(err, ErrCategoryDealRoomViaAPI) {
		t.Fatalf("got %v", err)
	}
	if err := ValidateManualCategoryChange(CategoryGeneral, CategoryAgreement, 2); !errors.Is(err, ErrCategoryWhileInRoom) {
		t.Fatalf("got %v", err)
	}
}
