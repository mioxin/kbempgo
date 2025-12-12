package cli

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/datasource"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
	wrk "github.com/mioxin/kbempgo/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/emptypb"
)

var e *employCommand = &employCommand{
	Lg: slog.Default(),
}

func TestGetFileCollection(t *testing.T) {
	fexpected := map[string]wrk.AvatarInfo{
		"54747": {ActualName: "54747.jpg", Num: 1, Size: 29, Hash: "7549da98ec1383ce"},
		"54755": {ActualName: "54755.jpg", Num: 1, Size: 29, Hash: "001d9c68e09e3b2f"},
		"54760": {ActualName: "54760 (2).jpg", Num: 2, Size: 33, Hash: "a1b99ab927a22f02"},
		"54877": {ActualName: "54877 (2).jpg", Num: 2, Size: 33, Hash: "23974fabd80666c1"},
	}

	fc, err := e.getFileCollection("./testdata/avatars")
	require.NoError(t, err)

	assert.Equal(t, fc, fexpected)
}

type Gcli struct{}

var expextedJsons []string = []string{
	`{"idr":"razd2916.13.3115.3117","parent":"razd2916.13.3115","text":"Отдел Мониторинга Качества Работы Партнеров","children":true}`,
	`{"idr":"razd86.119.88","parent":"razd86.119","text":"Администрация","children":true}`,
	`{"idr":"sotr6323","tabnum":"1000380","name":"Гас Га","midName":"Александровна","phone":["400-11-27"],"mobile":["+7 (701) 872-98-99","+7 (701) 996-91-29"],"email":"Ga.Gas@kaspi.kz","avatar":"/avatar/1000380.jpg","grade":"Главный бухгалтер","children":false,"parent_id":"razd1985","date":"0001-01-01T00:00:00Z"}`,
	`{"idr":"sotr9146","tabnum":"59029","name":"Бах Инд","midName":"Болатовна","phone":["423-255, 423-254"],"mobile":["+7 (775) 017-36-00"],"email":"Ind.Bakh@kaspi.kz","avatar":"/avatar/59029.jpg","grade":"Kaspi Гид","children":false,"parent_id":"razd86.99.2433","date":"0001-01-01T00:00:00Z"}`,
}

func (c *Gcli) Save(ctx context.Context, in *kbv1.Item, opts ...grpc.CallOption) (_ *emptypb.Empty, err error) {

	var item models.Item

	t, _ := CtxValue[*testing.T](ctx)
	count, _ := CtxValue[*int](ctx)

	item = in.GetDep()

	if !item.GetChildren() {
		item = in.GetSotr()
		sotr := new(kbv1.Sotr)
		err = protojson.Unmarshal([]byte(expextedJsons[*count]), sotr)

		if err != nil {
			return nil, err
		}

		expected := utils.ConvKbv2Ds(sotr)
		actual := utils.ConvKbv2Ds(item).(*datasource.Sotr)
		assert.EqualValues(t, expected, actual)

	} else {
		dep := new(kbv1.Dep)
		err = json.Unmarshal([]byte(expextedJsons[*count]), dep)

		if err != nil {
			return nil, err
		}

		expected := utils.ConvKbv2Ds(dep)
		actual := utils.ConvKbv2Ds(item).(*datasource.Dep)
		assert.EqualValues(t, expected, actual)
	}

	(*count)++
	// fmt.Printf(">> %#v\n", string(jstr))
	return nil, nil
}

func TestInsert(t *testing.T) {
	counter := 0
	gcli := new(Gcli)
	ctx := WithValue(context.Background(), t)
	ctx = WithValue(ctx, &counter)

	err := e.insert(ctx, "./testdata/tmp/dep.json", gcli, true)
	assert.NoError(t, err)

	err = e.insert(ctx, "./testdata/tmp/sotr.json", gcli, false)
	assert.NoError(t, err)
}

func (c *Gcli) GetDepsBy(ctx context.Context, in *kbv1.DepRequest, opts ...grpc.CallOption) (*kbv1.DepsResponse, error) {
	return nil, nil
}
func (c *Gcli) GetSotrsBy(ctx context.Context, in *kbv1.SotrRequest, opts ...grpc.CallOption) (*kbv1.SotrsResponse, error) {
	return nil, nil
}
func (c *Gcli) Flush(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (c *Gcli) Update(ctx context.Context, in *kbv1.UpdateSotrRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (c *Gcli) GetHistory(ctx context.Context, in *kbv1.HistRequest, opts ...grpc.CallOption) (*kbv1.HistoryListResponse, error) {
	return nil, nil
}

type key[T any] struct{}

func WithValue[T any](ctx context.Context, val T) context.Context {
	return context.WithValue(ctx, key[T]{}, val)
}

func CtxValue[T any](ctx context.Context) (T, bool) {
	val, ok := ctx.Value(key[T]{}).(T)
	return val, ok
}
