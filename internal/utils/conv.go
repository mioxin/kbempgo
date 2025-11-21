package utils

import (
	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/datasource"
	"github.com/mioxin/kbempgo/internal/models"
)

func ConvKbv2Ds(item models.Item) (ds datasource.Item) {

	if item.GetChildren() {
		if dep, ok := item.(*kbv1.Dep); ok {
			return &datasource.Dep{
				Idr:      dep.Idr,
				Parent:   dep.Parent,
				Text:     dep.Text,
				Children: dep.Children,
			}
		}
	} else {
		if sotr, ok := item.(*kbv1.Sotr); ok {
			return &datasource.Sotr{
				Idr:      sotr.Idr,
				Tabnum:   sotr.Tabnum,
				Name:     sotr.Name,
				MidName:  sotr.MidName,
				Email:    sotr.Email,
				Phone:    sotr.Phone,
				Mobile:   sotr.Mobile,
				Avatar:   sotr.Avatar,
				Grade:    sotr.Grade,
				Children: sotr.Children,
				ParentID: sotr.ParentId,
			}
		}
	}

	return nil
}
