package file

import (
	"context"
	"io"
	"log/slog"
	"testing"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestGetDepsBy(t *testing.T) {
	expected := kbv1.Dep{
		Idr:      "razd1941.840",
		Parent:   "razd1941",
		Text:     "Управление разработки",
		Children: true,
	}
	stor, err := NewFileStore("./testdata", slog.Default())

	require.NoError(t, err)
	defer stor.Close()

	d, err := stor.GetDepsBy(context.TODO(), &kbv1.DepRequest{Field: kbv1.DepRequest_IDR, Str: "razd1941.840"})
	if err != io.EOF {
		require.NoError(t, err)
	}

	require.Less(t, 0, len(d))

	assert.True(t, proto.Equal(&expected, d[0]))
}

var expectSotr kbv1.Sotr = kbv1.Sotr{
	Idr:      "sotr9146",
	Tabnum:   "59029",
	Name:     "Бах Инд",
	MidName:  "Болатовна",
	Phone:    []string{"423-255, 423-254"},
	Mobile:   []string{"+7 (775) 017-36-00"},
	Email:    "Ind.Bakh@kaspi.kz",
	Avatar:   "/avatar/59029.jpg",
	Grade:    "Kaspi Гид",
	Children: false,
	ParentId: "razd86.99.2433",
}

type field struct {
	name  kbv1.SotrRequest_DBField
	value string
	err   error
}

func TestSotrsResponseBy(t *testing.T) {

	var fields = []field{
		{
			name:  (kbv1.SotrRequest_TABNUM),
			value: "59029",
			err:   nil,
		},
		{
			name:  (kbv1.SotrRequest_FIO),
			value: "Бах Инд",
			err:   nil,
		},
		{
			name:  10,
			value: "",
			err:   &FieldNameError{"undefined"},
		},
	}

	d := make([]*kbv1.Sotr, 0)

	stor, err := NewFileStore("./testdata", slog.Default())

	require.NoError(t, err)
	defer stor.Close()

	for _, f := range fields {
		t.Run(f.name.String(), func(t *testing.T) {
			d, err = stor.GetSotrsBy(context.TODO(), &kbv1.SotrRequest{Field: f.name, Str: f.value})
			if f.name != 10 {
				require.Less(t, 0, len(d))
			}
			if err != io.EOF && err != nil {
				require.Equal(t, err.Error(), f.err.Error())
			} else {
				assert.Equal(t, expectSotr.Idr, d[0].Idr)
				assert.Equal(t, expectSotr.Name, d[0].Name)
				assert.Equal(t, expectSotr.Phone, d[0].Phone)
			}
			// stor.flS.Seek(0, io.SeekStart)
		})
	}
}
