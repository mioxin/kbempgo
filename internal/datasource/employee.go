package datasource

import (
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Item interface {
	Conv2Kbv() *kbv1.Item
	GetChildren() bool
}

type Sotr struct {
	ID        uint      `json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	Idr       string    `json:"idr"`
	Tabnum    string    `json:"tabnum"`
	Name      string    `json:"name"`
	MidName   string    `json:"mid_name"`
	Phone     []string  `gorm:"-" json:"phone,omitempty"`
	Mobile    []string  `gorm:"-" json:"mobile,omitempty"`
	Email     string    `json:"email,omitempty"`
	Avatar    string    `json:"avatar"`
	Grade     string    `json:"grade"`
	Children  bool      `json:"children"`
	ParentID  string    `json:"parent_id"`
	HistoryID *uint     `json:"-"`
	Deleted   bool      `json:"-"`
}

// IsSotr indicate sotr node
func (d Sotr) GetChildren() bool {
	return d.Children
}

func (d Sotr) Conv2Kbv() *kbv1.Item {
	// p := []string{}
	// m := []string{}
	// if d.Phone != "" {
	// 	p = strings.Split(d.Phone, ",")
	// }
	// if d.Mobile != "" {
	// 	m = strings.Split(d.Mobile, ",")
	// }
	return &kbv1.Item{
		Var: &kbv1.Item_Sotr{
			Sotr: &kbv1.Sotr{
				Id:       uint64(d.ID),
				Idr:      d.Idr,
				Tabnum:   d.Tabnum,
				Name:     d.Name,
				MidName:  d.MidName,
				Phone:    d.Phone,
				Mobile:   d.Mobile,
				Email:    d.Email,
				Avatar:   d.Avatar,
				Grade:    d.Grade,
				Children: d.Children,
				ParentId: d.ParentID,
				Date:     timestamppb.New(d.CreatedAt),
			},
		},
	}
}

type Dep struct {
	ID        uint      `json:"-"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	Idr       string    `json:"idr"`
	Parent    string    `json:"parent"`
	Text      string    `json:"text"`
	Children  bool      `json:"children"`
	Deleted   bool      `json:"-"`
}

func (d Dep) GetChildren() bool {
	return d.Children
}

func (d Dep) Conv2Kbv() *kbv1.Item {
	return &kbv1.Item{
		Var: &kbv1.Item_Dep{
			Dep: &kbv1.Dep{
				Id:       uint64(d.ID),
				Idr:      d.Idr,
				Parent:   d.Parent,
				Text:     d.Text,
				Children: d.Children,
			},
		},
	}
}

type SotrDeleted struct {
	Sotr
}

type History struct {
	ID            uint
	CreatedAt     time.Time `json:"date"`
	Field         string    `json:"field"`
	OldValue      string    `json:"old_value"`
	SotrID        *uint     `json:"sotr_id,omitempty"`
	SotrDeletedID *uint     `json:"sotr_deleted_id,omitempty"`
}

type Phone struct {
	ID            uint
	SotrID        *uint  `json:"sotr_id,omitempty"`
	SotrDeletedID *uint  `json:"sotr_deleted_id,omitempty"`
	Phone         string `json:"phone"`
}

type Mobile struct {
	ID            uint
	SotrID        *uint  `json:"sotr_id,omitempty"`
	SotrDeletedID *uint  `json:"sotr_deleted_id,omitempty"`
	Mobile        string `json:"mobile"`
}
