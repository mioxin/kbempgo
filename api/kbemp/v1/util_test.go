package kbv1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var oldSotr *Sotr = &Sotr{
	Idr:      "sotr9146",
	Tabnum:   "59029",
	Name:     "Бах Инд",
	MidName:  "Болатовна",
	Phone:    []string{"423-255", "423-254"},
	Mobile:   []string{"+7 (775) 017-36-00"},
	Email:    "Ind.Bakh@kaspi.kz",
	Avatar:   "/avatar/59029.jpg",
	Grade:    "Kaspi Гид",
	Children: false,
	ParentId: "razd86.99.2433",
}

var tc []struct {
	name string
	old  *Sotr
	new  *Sotr
	diff []*Diff
} = []struct {
	name string
	old  *Sotr
	new  *Sotr
	diff []*Diff
}{
	{
		name: "Equal sotr",
		old:  oldSotr,
		new: &Sotr{
			Idr:      "sotr9146",
			Tabnum:   "59029",
			Name:     "Бах Инд",
			MidName:  "Болатовна",
			Phone:    []string{"423-255", "423-254"},
			Mobile:   []string{"+7 (775) 017-36-00"},
			Email:    "Ind.Bakh@kaspi.kz",
			Avatar:   "/avatar/59029.jpg",
			Grade:    "Kaspi Гид",
			Children: false,
			ParentId: "razd86.99.2433",
		},
		diff: []*Diff{},
	},
	{
		name: "1 diff (Mobile)",
		old:  oldSotr,
		new: &Sotr{
			Idr:      "sotr9146",
			Tabnum:   "59029",
			Name:     "Бах Инд",
			MidName:  "Болатовна",
			Phone:    []string{"423-254", "423-255"},
			Mobile:   []string{"+7 (775) 017-99-99"},
			Email:    "Ind.Bakh@kaspi.kz",
			Avatar:   "/avatar/59029.jpg",
			Grade:    "Kaspi Гид",
			Children: false,
			ParentId: "razd86.99.2433",
		},
		diff: []*Diff{
			{
				FieldName: "Mobile",
				Val:       []string{"+7 (775) 017-36-00"},
			},
		},
	},
	{
		name: "2 diff (Mobile, Grade)",
		old:  oldSotr,
		new: &Sotr{
			Idr:      "sotr9146",
			Tabnum:   "59029",
			Name:     "Бах Инд",
			MidName:  "Болатовна",
			Phone:    []string{"423-255", "423-254"},
			Mobile:   []string{"+7 (775) 017-99-99"},
			Email:    "Ind.Bakh@kaspi.kz",
			Avatar:   "/avatar/59029.jpg",
			Grade:    "Kaspi BOSS",
			Children: false,
			ParentId: "razd86.99.2433",
		},
		diff: []*Diff{
			{
				FieldName: "Mobile",
				Val:       []string{"+7 (775) 017-36-00"},
			},
			{
				FieldName: "Grade",
				Val:       "Kaspi Гид",
			},
		},
	},
}

func TestCompareSotr(t *testing.T) {

	for _, test := range tc {
		t.Run(test.name, func(t *testing.T) {
			actualDiff, _ := CompareSotr([]*Sotr{test.old}, test.new)
			assert.Equal(t, test.diff, actualDiff)
		})
	}
}
