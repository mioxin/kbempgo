package datasource

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type Item interface {
	Conv2Kbv() *kbv1.Item
	GetChildren() bool
}

type Sotr struct {
	gorm.Model
	// ID        uint      `gorm:"primaryKey" json:"-"`
	// CreatedAt time.Time `gorm:"index,option:CONCURRENTLY" json:"date"`
	// UpdatedAt time.Time `json:"-"`
	Idr       string   `gorm:"size:255;index,option:CONCURRENTLY" json:"idr"`
	Tabnum    string   `gorm:"size:16;uniqueIndex" json:"tabnum"`
	Name      string   `gorm:"size:255;index:idx_fio,option:CONCURRENTLY" json:"name"`
	MidName   *string  `gorm:"size:255;index:idx_fio,option:CONCURRENTLY" json:"midName"`
	Phone     []Phone  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	Mobile    []Mobile `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	Email     *string  `gorm:"index" json:"email,omitempty"`
	Avatar    string   `gorm:"size:255" json:"avatar"`
	Grade     string   `gorm:"size:255" json:"grade"`
	Children  bool     `json:"children"`
	ParentIdr string   `gorm:"size:255" json:"parentIdr"`

	DepID *uint `json:"-"`
	Dep   Dep   `json:"-"`

	History []History `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`

	// Deleted bool `gorm:"default:false" json:"-"`
}

func (d *Sotr) BeforeSave(tx *gorm.DB) (err error) {
	// generate history if exists old data
	oldSotrsResponse := []Sotr{}

	r := tx.Where("tabnum = ?", d.Tabnum).Preload("Phone").Preload("Mobile").Find(&oldSotrsResponse)
	if r.Error != nil {
		err = r.Error
		return
	}

	if len(oldSotrsResponse) > 0 {
		hist := d.Diff(oldSotrsResponse[len(oldSotrsResponse)-1])
		d.History = hist
	}
	d.Phone = nil
	d.Mobile = nil
	return
}

func (s Sotr) Diff(oldSotr Sotr) []History {
	var oldPhones, oldMobiles, sPhones, sMobiles []string
	h := make([]History, 0)

	kbv1OldSotr := oldSotr.Conv2Kbv().GetSotr()
	oldPhones = kbv1OldSotr.Phone
	oldMobiles = kbv1OldSotr.Mobile

	kbv1sSotr := s.Conv2Kbv().GetSotr()
	sPhones = kbv1sSotr.Phone
	sMobiles = kbv1sSotr.Mobile

	if !slices.Equal(sPhones, oldPhones) {
		h = append(h, History{Field: "phone", OldValue: strings.Join(oldPhones, ",")})
	}
	if !slices.Equal(sMobiles, oldMobiles) {
		h = append(h, History{Field: "mobile", OldValue: strings.Join(oldMobiles, ",")})
	}

	if s.Idr != oldSotr.Idr {
		h = append(h, History{Field: "idr", OldValue: oldSotr.Idr})
	}
	if s.Name != oldSotr.Name {
		h = append(h, History{Field: "name", OldValue: oldSotr.Name})
	}

	if *s.Email != *oldSotr.Email {
		h = append(h, History{Field: "email", OldValue: *oldSotr.Email})
	}
	if s.Avatar != oldSotr.Avatar {
		h = append(h, History{Field: "avatar", OldValue: oldSotr.Avatar})
	}
	if s.Grade != oldSotr.Grade {
		h = append(h, History{Field: "grade", OldValue: oldSotr.Grade})
	}
	if s.ParentIdr != oldSotr.ParentIdr {
		h = append(h, History{Field: "parent_idr", OldValue: oldSotr.ParentIdr})
	}
	return h
}
func (d Sotr) GetChildren() bool {
	return d.Children
}

func (d Sotr) Conv2Kbv() *kbv1.Item {
	sotr := &kbv1.Sotr{}
	var (
		p []string
		m []string
	)
	for _, ph := range d.Phone {
		p = append(p, ph.Phone)
	}
	for _, mob := range d.Mobile {
		m = append(m, mob.String())
	}
	sotr.Id = uint64(d.ID)
	sotr.Idr = d.Idr
	sotr.Tabnum = d.Tabnum
	sotr.Name = d.Name
	if d.MidName != nil {
		sotr.MidName = *d.MidName
	}
	sotr.Phone = p
	sotr.Mobile = m
	if d.Email != nil {
		sotr.Email = *d.Email
	}
	sotr.Avatar = d.Avatar
	sotr.Grade = d.Grade
	sotr.Children = d.Children
	sotr.ParentId = d.ParentIdr
	sotr.Date = timestamppb.New(d.CreatedAt)

	return &kbv1.Item{
		Var: &kbv1.Item_Sotr{
			Sotr: sotr,
		},
	}
}

type Dep struct {
	gorm.Model
	Idr      string `gorm:"size:255;index:idx_dep_idr;uniqueIndex:idx_dep_idr_parent_text" json:"idr"`
	Parent   string `gorm:"size:255;uniqueIndex:idx_dep_idr_parent_text" json:"parent"`
	Text     string `gorm:"uniqueIndex:idx_dep_idr_parent_text" json:"text"`
	Children bool   `json:"children"`
	// Deleted   bool      `gorm:"default:false" json:"-"`
}

func (d Dep) BeforeCreate(tx *gorm.DB) (err error) {
	return
}

func (d Dep) GetChildren() bool {
	return d.Children
}

func (d Dep) Conv2Kbv() *kbv1.Item {
	dep := &kbv1.Dep{}
	dep.Id = uint64(d.ID)
	dep.Idr = d.Idr
	dep.Parent = d.Parent
	dep.Text = d.Text
	dep.Children = d.Children

	return &kbv1.Item{
		Var: &kbv1.Item_Dep{
			Dep: dep,
		},
	}
}

type SotrDeleted struct {
	Sotr
	Phone  []Phone  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	Mobile []Mobile `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
}

type History struct {
	ID        uint      `gorm:"primaryKey"`
	CreatedAt time.Time `gorm:"index" json:"date"`
	Field     string    `gorm:"size:55" json:"field"`
	OldValue  string    `gorm:"size:255" json:"old_value"`

	SotrID *uint `json:"sotr_id,omitempty"`
	// Sotr   Sotr  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`

	SotrDeletedID *uint `json:"sotr_deleted_id,omitempty"`
	// SotrDeleted   SotrDeleted `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
}

// format struct as a value string for sql insert query like (value1,value2)
// for build query like a
// INSERT INTO table (field1, field2) VALUES ((value1,value2),(value1,value2)...)
func (p History) SqlInsertValueFormat() string {
	return fmt.Sprintf("(%d, '%s', '%s')", *p.SotrID, p.Field, p.OldValue)
}

type Phone struct {
	ID    uint   `gorm:"primaryKey"`
	Phone string `gorm:"size:16;index;uniqueIndex:idx_phone_sotrid;uniqueIndex:idx_phone_sotrdelid" json:"phone"`

	SotrID *uint `gorm:"uniqueIndex:idx_phone_sotrid" json:"sotr_id,omitempty"`
	// Sotr   Sotr  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`

	SotrDeletedID *uint `gorm:"uniqueIndex:idx_phone_sotrdelid" json:"sotr_deleted_id,omitempty"`
	// SotrDeleted   SotrDeleted `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
}

// format struct as a value string for sql insert query like (value1,value2)
// for build query like a
// INSERT INTO table (field1, field2) VALUES ((value1,value2),(value1,value2)...)
func (p Phone) SqlInsertValueFormat() string {
	return fmt.Sprintf("(%d, '%s')", *p.SotrID, p.Phone)
}

type Mobile struct {
	ID     uint `gorm:"primaryKey"`
	Mobile uint `gorm:"index;uniqueIndex:idx_mobile_sotrid;uniqueIndex:idx_mobile_sotrdelid" json:"mobile"`

	SotrID *uint `gorm:"uniqueIndex:idx_mobile_sotrid" json:"sotr_id,omitempty"`
	// Sotr   Sotr  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`

	SotrDeletedID *uint `gorm:"uniqueIndex:idx_mobile_sotrdelid" json:"sotr_deleted_id,omitempty"`
	// SotrDeleted   SotrDeleted `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
}

func (m *Mobile) String() string {
	sm := strconv.FormatUint(uint64(m.Mobile), 10)
	if len(sm) < 5 {
		return "0"
	}
	return fmt.Sprintf("+%s (%s) %s", sm[0:1], sm[1:4], sm[4:])
}

// format struct as a value string for sql insert query like (value1,value2)
// for build query like a
// INSERT INTO table (field1, field2) VALUES ((value1,value2),(value1,value2)...)
func (m Mobile) SqlInsertValueFormat() string {
	return fmt.Sprintf("(%d, '%d')", *m.SotrID, m.Mobile)
}
