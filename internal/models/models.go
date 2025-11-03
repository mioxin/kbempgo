package models

// Models is implementation of objects from web source representation

import (
	"strings"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Item interface {
	proto.Message
	//	IsSotr() bool
	GetChildren() bool
}

type Dep struct {
	// Id       int    `json:"uid,omitempty"`
	Idr      string `json:"id"`
	Parent   string `json:"parent"`
	Text     string `json:"text"`
	Children bool   `json:"children"`
	Delete   bool   `json:"delete,omitempty"`
}

func (d Dep) GetChildren() bool {
	return d.Children
}

func (d Dep) Conv2Kbv() *kbv1.Item {
	return &kbv1.Item{
		Var: &kbv1.Item_Dep{
			Dep: &kbv1.Dep{
				Id:       uint64(0),
				Idr:      d.Idr,
				Parent:   d.Parent,
				Text:     d.Text,
				Children: d.Children,
			},
		},
	}
}

type Sotr struct {
	// Id       int    `json:"uid,omitempty"`
	Idr      string `json:"id"`
	Tabnum   string `json:"tabnum"`
	Name     string `json:"name"`
	MidName  string `json:"mid_name"`
	Phone    string `json:"phone"`
	Mobile   string `json:"mobile"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
	Grade    string `json:"grade"`
	Children bool   `json:"children"`
	ParentId string `json:"parent_id"`
	// Hist     []History `json:"history"`
}

// IsSotr indicate sotr node
func (d Sotr) GetChildren() bool {
	return d.Children
}

func (d Sotr) Conv2Kbv() *kbv1.Item {
	p := []string{}
	m := []string{}
	if d.Phone != "" {
		p = strings.Split(d.Phone, ",")
	}
	if d.Mobile != "" {
		m = strings.Split(d.Mobile, ",")
	}
	return &kbv1.Item{
		Var: &kbv1.Item_Sotr{
			Sotr: &kbv1.Sotr{
				Id:       uint64(0),
				Idr:      d.Idr,
				Tabnum:   d.Tabnum,
				Name:     d.Name,
				MidName:  d.MidName,
				Phone:    p,
				Mobile:   m,
				Email:    d.Email,
				Avatar:   d.Avatar,
				Grade:    d.Grade,
				Children: d.Children,
				ParentId: d.ParentId,
				Date:     timestamppb.Now(),
			},
		},
	}
}

type History struct {
	Date     time.Time `json:"date"`
	Field    string    `json:"field"`
	OldValue string    `json:"old_value"`
}
