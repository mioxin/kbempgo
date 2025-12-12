package pg

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type DBTestSuite struct {
	suite.Suite

	store *PgStore
	Sotrs []*kbv1.Sotr
	Deps  []*kbv1.Dep
}

func (suite *DBTestSuite) SetupTest() {
	var err error

	suite.store, err = New("postgres://kb:123456@localhost:5432/test_www_int", slog.Default())
	suite.Require().NoError(err)

	err = suite.store.Migrate(context.TODO(), false)
	suite.Require().NoError(err)

	// load SotrsResponse and DepsResponse
	depsResponse := &kbv1.DepsResponse{}
	suite.LoadJSONPb(suite.T(), "dep.json", depsResponse)
	suite.Deps = depsResponse.Deps

	sotrsResponse := &kbv1.SotrsResponse{}
	suite.LoadJSONPb(suite.T(), "sotr.json", sotrsResponse)
	suite.Sotrs = sotrsResponse.Sotrs
}

func (suite *DBTestSuite) TearDownTest() {
	err := suite.store.Migrate(context.TODO(), true)
	suite.Require().NoError(err)

	db := suite.store.DB
	tables, err := db.Migrator().GetTables()
	suite.Require().NoError(err)
	// defer

	for _, table := range tables {
		suite.store.Log.Debug("Drop table", "table", table)

		err = db.Migrator().DropTable(table) // nosemgrep
		suite.Require().NoError(err)
	}
}

func (suite *DBTestSuite) LoadJSONPb(t *testing.T, filename string, out protoreflect.ProtoMessage) {
	t.Helper()

	buf, err := os.ReadFile(filepath.Join("", "testdata", filename))
	require.NoError(t, err)

	err = protojson.Unmarshal(buf, out)
	require.NoError(t, err)
}

func (st *DBTestSuite) loadDB(t *testing.T) {
	t.Helper()
	var err error

	for _, d := range st.Deps {
		_, err = st.store.Save(context.Background(), d)
		require.NoError(t, err)
	}
	// 1st Flash should be skiped
	st.store.Flush(context.Background(), nil)

	for _, s := range st.Sotrs {
		_, err = st.store.Save(context.Background(), s)
		require.NoError(t, err)
	}

	_, err = st.store.Flush(context.Background(), nil)
	require.NoError(t, err)
}

func (suite *DBTestSuite) MustQueryCount(t *testing.T, query string, args ...any) (ret int) {
	t.Helper()
	db := suite.store.DB

	r := db.Exec(query, args...)
	suite.Require().NoError(r.Error)

	ret = int(r.RowsAffected)
	return
}

type Counts struct {
	deps, sotrs, sotr_deleteds, phones, mobiles, histories int
}

func (cn *Counts) AddDeps(i int) {
	cn.deps += i
}
func (cn *Counts) AddSotrs(i int) {
	cn.sotrs += i
}
func (cn *Counts) AddSotrsD(i int) {
	cn.sotr_deleteds += i
}
func (cn *Counts) AddPhones(i int) {
	cn.phones += i
}
func (cn *Counts) AddMobiles(i int) {
	cn.mobiles += i
}
func (cn *Counts) AddHistories(i int) {
	cn.histories += i
}
func (suite *DBTestSuite) counts(t *testing.T) *Counts {
	t.Helper()

	var ret int64

	c := &Counts{}
	db := suite.store.DB

	for _, tb := range []string{"deps", "sotrs", "sotr_deleteds", "phones", "mobiles", "histories"} {
		err := db.Table(tb).Count(&ret).Error
		suite.Require().NoError(err)

		switch tb {
		case "deps":
			c.deps = int(int(ret))
		case "sotrs":
			c.sotrs = int(ret)
		case "sotr_deleteds":
			c.sotr_deleteds = int(ret)
		case "phones":
			c.phones = int(ret)
		case "mobiles":
			c.mobiles = int(ret)
		case "histories":
			c.histories = int(ret)
		}
	}

	return c
}
