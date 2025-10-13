package models

import "time"

type Item interface {
	IsSotr() bool
}

type Dep struct {
	Id       int    `json:"uid,omitempty"`
	Idr      string `json:"id"`
	Parent   string `json:"parent"`
	Text     string `json:"text"`
	Children bool   `json:"children"`
	Delete   bool   `json:"delete,omitempty"`
}

func (d *Dep) IsSotr() bool {
	return !d.Children
}

type Sotr struct {
	Id       int       `json:"uid,omitempty"`
	Idr      string    `json:"id"`
	Tabnum   string    `json:"tabnum"`
	Name     string    `json:"name"`
	MidName  string    `json:"mid_name"`
	Phone    string    `json:"phone"`
	Mobile   string    `json:"mobile"`
	Email    string    `json:"email"`
	Avatar   string    `json:"avatar"`
	Grade    string    `json:"grade"`
	Children bool      `json:"children"`
	ParentId string    `json:"parent_id"`
	Hist     []History `json:"history"`
}

// IsSotr indicate sotr node
func (d *Sotr) IsSotr() bool {
	return !d.Children
}

type History struct {
	Date     time.Time `json:"date"`
	Field    string    `json:"field"`
	OldValue string    `json:"old_value"`
}
