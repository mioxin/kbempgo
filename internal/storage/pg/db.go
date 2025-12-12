package pg

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/datasource"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/mioxin/kbempgo/internal/utils"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	DepDuplicateDefineFields []clause.Column = []clause.Column{
		{Name: "idr"},
		{Name: "parent"},
		{Name: "text"},
	}
	SotrDuplicateDefineFields   []clause.Column = []clause.Column{{Name: "tabnum"}}
	PhoneDuplicateDefineFields  []clause.Column = []clause.Column{{Name: "sotr_id"}, {Name: "phone"}}
	MobileDuplicateDefineFields []clause.Column = []clause.Column{{Name: "sotr_id"}, {Name: "mobile"}}
)

type PgStore struct {
	kbv1.UnimplementedStorAPIServer

	DB  *gorm.DB
	Log *slog.Logger
	// Counter difine 2nd flash it's means load to internal maps 2nd and final part of data
	// after that we can sync DB with internal maps
	FlashCounter atomic.Int32
	// Internal map contains Sotr's items by Tabnum key for save
	Sotrmap map[string]*kbv1.Sotr
	// Internal map contains Dep's items by Idr key for save
	Depmap map[string]*kbv1.Dep
}

func New(dsn string, log *slog.Logger) (pgs *PgStore, err error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		err = fmt.Errorf("error create Store, invalid source, %s. %w", dsn, err)
		return
	}
	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})

	if err != nil {
		err = fmt.Errorf("error create Store, invalid source, %s. %w", dsn, err)
		return
	}

	sqlDB, err = db.DB()
	if err != nil {
		return nil, err
	}
	_, err = sqlDB.Exec("SET search_path TO public")
	if err != nil {
		return nil, fmt.Errorf("set search_path: %w", err)
	}

	return &PgStore{
		DB:      db,
		Log:     log.With("storage", "postgres"),
		Depmap:  make(map[string]*kbv1.Dep, 50),
		Sotrmap: make(map[string]*kbv1.Sotr, 100),
	}, nil
}

func (p *PgStore) GetDepsBy(ctx context.Context, q *kbv1.DepRequest) (deps []*kbv1.Dep, err error) {
	var (
		r     *gorm.DB
		items []datasource.Dep
	)
	field := q.Field.Enum().String()
	if field == "NONE" {
		r = p.DB.Find(&items)
	} else {
		r = p.DB.Where(fmt.Sprintf("%s = ?", field), q.Str).Find(&items)
	}

	if r.Error != nil {
		err = r.Error
		return
	}

	for _, dsDep := range items {
		deps = append(deps, dsDep.Conv2Kbv().GetDep())
	}
	return
}

// GetSotr returns employee data
func (p *PgStore) GetSotrsBy(ctx context.Context, q *kbv1.SotrRequest) (sotrs []*kbv1.Sotr, err error) {
	var (
		datasourceSotrs []datasource.Sotr
		sotrIds         []int
		r               *gorm.DB
	)
	f := q.Field.Enum().String()

	switch f {
	case "MOBILE":
		// rows := make([]datasource.Mobile, 0)
		mob, err := strconv.Atoi(utils.ExtractDigits(q.Str))
		if err != nil {
			return nil, err
		}
		r = p.DB.Model(&datasource.Mobile{}).Where("mobile = ?", mob).Pluck("sotr_id", &sotrIds)
		if r.Error != nil {
			err = r.Error
			return nil, err
		}

		r = p.DB.Preload("Phone").Preload("Mobile").Find(&datasourceSotrs, sotrIds)

	case "FIO":
		// split FIO on name and mid_name
		slFio := strings.Fields(q.Str)
		name := fmt.Sprintf("%s %s", slFio[0], slFio[1])
		midName := ""

		if len(slFio) > 2 {
			midName = slFio[2]
			r = p.DB.Where("name = ? and mid_name = ?", name, midName).Preload("Phone").Preload("Mobile").Find(&datasourceSotrs)
		} else {
			r = p.DB.Where("name = ?", name).Preload("Phone").Preload("Mobile").Find(&datasourceSotrs)
		}

	case "NONE":
		r = p.DB.Preload("Mobile").Find(&datasourceSotrs)

	default:
		r = p.DB.Where(fmt.Sprintf("%s = ?", q.Field.Enum().String()), q.Str).Preload("Phone").Preload("Mobile").Find(&datasourceSotrs)
	}

	if r.Error != nil {
		err = r.Error
		return
	}

	for _, dsSotr := range datasourceSotrs {
		sotrs = append(sotrs, dsSotr.Conv2Kbv().GetSotr())
	}
	return
}

// Save Item data to internal maps
func (p *PgStore) Save(ctx context.Context, item models.Item) (em *emptypb.Empty, err error) {
	if item.GetChildren() {
		// **************************
		// save Dep
		// **************************
		kbvDep, ok := item.(*kbv1.Dep)
		if !ok {
			err = fmt.Errorf("not kbv1_item: %v", item)
			return
		}

		p.Depmap[kbvDep.Idr] = kbvDep
	} else {
		// **************************
		// save Sotr
		// **************************
		kbvSotr, ok := item.(*kbv1.Sotr)
		if !ok {
			err = fmt.Errorf("not kbv1_item: %v", item)
			return
		}

		p.Sotrmap[kbvSotr.Tabnum] = kbvSotr
	}
	return
}

func (p *PgStore) Update(ctx context.Context, q *kbv1.UpdateSotrRequest) (em *emptypb.Empty, err error) {
	return
}

func (p *PgStore) Close() (err error) {
	return
}

// Sync data in DB with internal maps
func (p *PgStore) Flush(ctx context.Context, _ *emptypb.Empty) (_ *emptypb.Empty, err error) {

	p.FlashCounter.Add(1)
	// 1st Flash after saving DepsResponse, and 2nd final flash after saving SotrsResponse
	if p.FlashCounter.Load() < 2 {
		return
	}

	p.DB = p.DB.Debug()

	// sync DB
	p.Log.Info("Start transaction flash ...")
	p.DB.Transaction(func(tx *gorm.DB) error {
		var e error
		defer func() {
			if e != nil {
				p.Log.Error("Rollback flash on error...", "err", e)
			}
		}()

		dsDepMap, e := p.prepareDepsResponse(tx)
		if e != nil {
			return e
		}

		slSotr, e := p.prepareSotrsResponse(tx, dsDepMap)
		if e != nil {
			return e
		}

		// set actual ID to kbv1.Sotr after upsert sotrs
		for _, s := range slSotr {
			p.Sotrmap[s.Tabnum].Id = uint64(s.ID)
		}

		e = p.preparePhones(tx, slSotr)
		if e != nil {
			return e
		}

		return e
	})
	p.FlashCounter.Store(0)
	return
}

func (p *PgStore) prepareDepsResponse(tx *gorm.DB) (dsDepMap map[string]*datasource.Dep, err error) {
	// prepare DepsResponse
	slDep := make([]*datasource.Dep, 0, 100)
	dsDepMap = make(map[string]*datasource.Dep, 10)

	for idr, ds := range p.Depmap {
		dep := utils.ConvKbv2Ds(ds).(*datasource.Dep)
		slDep = append(slDep, dep)
		// use for set sotr.DepID
		dsDepMap[idr] = dep
		p.Log.Info("Flash: prepare DepsResponse appended to Dep slice", "Dep", dep)
	}

	gdb := tx.Clauses(clause.OnConflict{
		Columns:   DepDuplicateDefineFields,
		UpdateAll: true,
	}).CreateInBatches(&slDep, 100)

	if gdb.Error != nil {
		err = gdb.Error
		p.Log.Error("Flash: prepare DepsResponse", "num", gdb.RowsAffected, "err", gdb.Error)
	} else {
		p.Log.Info("Flash: prepare DepsResponse", "num", gdb.RowsAffected, "len_Sotrmap", len(slDep))
	}
	return
}

// prepare SotrsResponse
func (p *PgStore) prepareSotrsResponse(tx *gorm.DB, dsDepMap map[string]*datasource.Dep) (slSotr []*datasource.Sotr, err error) {
	slSotr = make([]*datasource.Sotr, 0, 100)

	for _, s := range p.Sotrmap {

		ds := utils.ConvKbv2Ds(s).(*datasource.Sotr)

		dep, ok := dsDepMap[ds.ParentIdr]
		if !ok {
			p.Log.Warn("Flash: skip item, Dep not found for", "kbv1_sotr", ds)
			continue
		}

		// fill DepID field if it exists in DB (seted in prepare Dep stage)
		if dep.ID > 0 { // Dep exists in DB
			ds.DepID = &dep.ID
		} else {
			ds.Dep = *dep
		}

		// fill Phone and Mobile for get hidtory (compare with old row)
		// but it will set to nil becourse GORM don't solve conflicts in dependent fields
		// We will upsert Phones and Mobiles later in preparePhone
		if len(s.Phone) > 0 {
			phone := utils.ConvKbv2Phone(s)
			ds.Phone = phone
		} else {
			ds.Phone = make([]datasource.Phone, 0)
		}

		if len(s.Mobile) > 0 {
			mobile := utils.ConvKbv2Mobile(s)
			ds.Mobile = mobile
		} else {
			ds.Mobile = make([]datasource.Mobile, 0)
		}

		slSotr = append(slSotr, ds)
		p.Log.Info("Flash: appended to Sotr slice", "sotr", ds)
	}

	gdb := tx.Clauses(clause.OnConflict{
		Columns:   SotrDuplicateDefineFields,
		UpdateAll: true,
	}).CreateInBatches(&slSotr, 100)

	if gdb.Error != nil {
		err = gdb.Error
		p.Log.Error("Flash: sync Sotr", "num", gdb.RowsAffected, "err", gdb.Error)
	} else {
		p.Log.Info("Flash: sync Sotr", "num", gdb.RowsAffected, "len_Sotrmap", len(slSotr))
	}

	return
}

// prepare Phone and Mobile
func (p *PgStore) preparePhones(tx *gorm.DB, slSotr []*datasource.Sotr) (err error) {
	var (
		PhonesForAdd  []datasource.Phone
		MobilesForAdd []datasource.Mobile
		PhonesForDel  []datasource.Phone
		MobilesForDel []datasource.Mobile
	)

	for _, s := range slSotr {
		sotrID := s.ID
		kbvSotr := p.Sotrmap[s.Tabnum]

		// get new phones&mobiles for upsert
		// becouse we can't solve a conflicts in dependent fields (Phone Mobile) while upsert a main struct (Sotr)
		if len(kbvSotr.Phone) > 0 {
			slPhones := utils.ConvKbv2Phone(kbvSotr)
			for i := range slPhones {
				slPhones[i].SotrID = &sotrID
			}
			PhonesForAdd = append(PhonesForAdd, slPhones...)
		}

		if len(kbvSotr.Mobile) > 0 {
			slMobile := utils.ConvKbv2Mobile(kbvSotr)
			for i := range slMobile {
				slMobile[i].SotrID = &sotrID
			}
			MobilesForAdd = append(MobilesForAdd, slMobile...)
		}

		// get old phones&mobiles from last history for delete
		if len(s.History) > 0 {
			for _, hist := range s.History {
				if hist.Field == "phone" {
					oldSotr := &kbv1.Sotr{
						Phone: strings.Split(hist.OldValue, ","),
					}
					slPhones := utils.ConvKbv2Phone(oldSotr)
					for i := range slPhones {
						slPhones[i].SotrID = &sotrID
					}
					PhonesForDel = append(PhonesForDel, slPhones...)
				}
				if hist.Field == "mobile" {
					oldSotr := &kbv1.Sotr{
						Mobile: strings.Split(hist.OldValue, ","),
					}
					slMobile := utils.ConvKbv2Mobile(oldSotr)
					for i := range slMobile {
						slMobile[i].SotrID = &sotrID
					}
					MobilesForDel = append(MobilesForDel, slMobile...)
				}
			}
		}
	}

	// del phone and Mobile.
	if len(PhonesForDel) > 0 {
		query, err := deleteQuery("phones", buildValuesStr(PhonesForDel))
		if err != nil {
			return err
		}

		if result := tx.Exec(query); result.Error != nil {
			err = result.Error
			return err
		} else {
			p.Log.Info("Delete phones", "num", result.RowsAffected)
		}
	}

	if len(MobilesForDel) > 0 {
		query, err := deleteQuery("mobiles", buildValuesStr(MobilesForDel))
		if err != nil {
			return err
		}

		if result := tx.Exec(query); result.Error != nil {
			err = result.Error
			return err
		} else {
			p.Log.Info("Delete mobiles", "num", result.RowsAffected)
		}
	}

	// upsert on conflict
	if len(PhonesForAdd) > 0 {
		if r := tx.Clauses(clause.OnConflict{
			Columns: PhoneDuplicateDefineFields,
			// UpdateAll: true,
			DoNothing: true,
		}).CreateInBatches(&PhonesForAdd, 100); r.Error != nil {
			err = r.Error
			return
		} else {
			p.Log.Info("Upsert phones", "num", r.RowsAffected)
		}
	}

	if len(MobilesForAdd) > 0 {
		if r := tx.Clauses(clause.OnConflict{
			Columns: MobileDuplicateDefineFields,
			// UpdateAll: true,
			DoNothing: true,
		}).CreateInBatches(&MobilesForAdd, 100); r.Error != nil {
			err = r.Error
			return
		} else {
			p.Log.Info("Upsert mobiles", "num", r.RowsAffected)
		}
	}

	return
}

func (p *PgStore) PromCollector() (prom prometheus.Collector) {
	return
}

// Migrate apply migrations to the DB
func (p *PgStore) Migrate(ctx context.Context, down bool) (err error) {
	p.DB.Exec("SET search_path TO public")
	errs := make([]error, 6)
	err = p.DB.AutoMigrate(&datasource.Dep{})
	errs = append(errs, err)
	err = p.DB.AutoMigrate(&datasource.Sotr{})
	errs = append(errs, err)
	err = p.DB.AutoMigrate(&datasource.SotrDeleted{})
	errs = append(errs, err)
	err = p.DB.AutoMigrate(&datasource.Phone{})
	errs = append(errs, err)
	err = p.DB.AutoMigrate(&datasource.Mobile{})
	errs = append(errs, err)
	err = p.DB.AutoMigrate(&datasource.History{})
	errs = append(errs, err)

	return errors.Join(errs...)
}

// Retention deletes entries older than passed time
func (p *PgStore) Retention(ctx context.Context, olderThan time.Time) (err error) {
	return
}

type Params struct {
	TableName string
	KeyField  string
	KeyType   string // Cast для uint
	Values    string
}

func deleteQuery(tab string, valuesStr string) (q string, err error) {

	tmpl, err := template.New("universalQuery").Parse(deleteQueryTemplate)
	if err != nil {
		return "", fmt.Errorf("template parse failed: %w", err)
	}

	switch tab {
	case "mobiles":
		mobilesParams := Params{
			TableName: "mobiles",
			KeyField:  "mobile",
			KeyType:   "bigint",
			Values:    valuesStr,
		}
		var sqlBuf bytes.Buffer
		if err := tmpl.Execute(&sqlBuf, mobilesParams); err != nil {
			return "", fmt.Errorf("template execute failed: %w", err)
		}
		q = sqlBuf.String()

	case "phones":
		phonesParams := Params{
			TableName: "phones",
			KeyField:  "phone",
			Values:    valuesStr,
		}
		var sqlBuf bytes.Buffer
		if err := tmpl.Execute(&sqlBuf, phonesParams); err != nil {
			return "", fmt.Errorf("template execute failed: %w", err)
		}
		q = sqlBuf.String()

	default:
		return "", fmt.Errorf("ivalid table name '%s'", tab)
	}
	return
}

type SqlInsertValuer interface {
	// format struct as a value string for sql insert query like (value1,value2)
	// for build query like a
	// INSERT INTO table (field1, field2) VALUES ((value1,value2),(value1,value2)...)
	SqlInsertValueFormat() string
}

func buildValuesStr[T SqlInsertValuer](items []T) string {
	var values []string
	for _, item := range items {
		values = append(values, item.SqlInsertValueFormat())
	}
	return strings.Join(values, ", ")
}
