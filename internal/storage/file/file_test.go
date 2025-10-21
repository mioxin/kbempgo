package file

import (
	"fmt"
	"io"
	"testing"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDepByIdr(t *testing.T) {
	expected := models.Dep{
		Idr:      "razd1941.840",
		Parent:   "razd1941",
		Text:     "Управление разработки",
		Children: true,
	}
	stor, err := NewFileStore[*models.Dep]("./testdata")

	require.NoError(t, err)
	defer stor.Close()

	d, err := stor.GetDepByIdr(&kbv1.QueryString{Str: "razd1941.840"})
	if err != io.EOF {
		require.NoError(t, err)
	}

	assert.Equal(t, expected, d)
}

var expectSotr models.Sotr = models.Sotr{
	Idr:      "sotr9146",
	Tabnum:   "59029",
	Name:     "Бах Инд",
	MidName:  "Болатовна",
	Phone:    "423-255, 423-254",
	Mobile:   "+7 (775) 017-36-00",
	Email:    "Ind.Bakh@kaspi.kz",
	Avatar:   "/avatar/59029.jpg",
	Grade:    "Kaspi Гид",
	Children: false,
	ParentId: "razd86.99.2433",
}

type field struct {
	name  string
	value string
	err   error
}

func TestSotrByTabnum(t *testing.T) {

	var fields = []field{
		{
			name:  "tabnum",
			value: "59029",
			err:   nil,
		},
		{
			name:  "fio",
			value: "Бах Инд",
			err:   nil,
		},
		{
			name:  "invalid field",
			value: "",
			err:   &FieldNameError{"invalid field"},
		},
	}

	d := &models.Sotr{}

	stor, err := NewFileStore[*models.Sotr]("./testdata")

	require.NoError(t, err)
	defer stor.Close()

	for _, f := range fields {
		t.Run(f.name, func(t *testing.T) {
			d, err = stor.GetSotrByField(f.name, &kbv1.QueryString{Str: f.value})

			if err != io.EOF && err != nil {
				require.IsType(t, f.err, err)
			} else {
				assert.Equal(t, expectSotr, *d)
			}
			stor.flS.Seek(0, io.SeekStart)
		})
	}

	d, err = stor.GetSotrByTabnum(&kbv1.QueryString{Str: "59029"})
	if err != io.EOF {
		require.NoError(t, err)
	}

	assert.Equal(t, expectSotr, *d)
}

func TestSotrsByTabnum(t *testing.T) {

	var fields = []field{
		{
			name:  "mobile",
			value: "+7 (775) 017-36-00",
			err:   nil,
		},
		{
			name:  "invalid field",
			value: "",
			err:   &FieldNameError{"invalid field"},
		},
	}

	stor, err := NewFileStore[*models.Sotr]("./testdata")

	require.NoError(t, err)
	defer stor.Close()

	for _, f := range fields {
		d := make([]*models.Sotr, 0, 3)

		t.Run(f.name, func(t *testing.T) {
			d, err = stor.GetSotrsByField(f.name, &kbv1.QueryString{Str: f.value})

			if err != io.EOF && err != nil {
				require.IsType(t, f.err, err)
			} else {
				assert.Equal(t, []*models.Sotr{&expectSotr}, d)
			}
			stor.flS.Seek(0, io.SeekStart)
		})
	}
}

func BenchmarkGetSotrByTabnum(b *testing.B) {
	stor, err := NewFileStore[*models.Sotr]("./testdata")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer stor.Close()

	for i := 0; i < b.N; i++ {
		_, _ = stor.GetSotrByTabnum(&kbv1.QueryString{Str: "59029"})
	}

}

func BenchmarkGetSotrByField(b *testing.B) {
	stor, err := NewFileStore[*models.Sotr]("./testdata")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer stor.Close()

	for i := 0; i < b.N; i++ {
		_, _ = stor.GetSotrByField("tabnum", &kbv1.QueryString{Str: "59029"})
	}
}

func BenchmarkGetDepByIdr(b *testing.B) {
	stor, err := NewFileStore[*models.Sotr]("./testdata")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer stor.Close()

	for i := 0; i < b.N; i++ {
		_, _ = stor.GetDepByIdr(&kbv1.QueryString{Str: "razd1941.840"})
	}

}

func BenchmarkGetDepByIdr1(b *testing.B) {
	stor, err := NewFileStore[*models.Sotr]("./testdata")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer stor.Close()

	for i := 0; i < b.N; i++ {
		_, _ = stor.GetDepByIdr1(&kbv1.QueryString{Str: "razd1941.840"})
	}
}
