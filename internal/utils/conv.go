package utils

import (
	"log/slog"
	"strconv"
	"strings"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/datasource"
	"github.com/mioxin/kbempgo/internal/models"
)

// convers kbv1 items struct to datasource items
// kbv1.Dep(Sotr) -> datasource.Dep(Sotr)
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
			var (
				mid   *string
				email *string
			)
			if sotr.MidName != "" {
				mid = &sotr.MidName
			}
			if sotr.Email != "" {
				email = &sotr.Email
			}
			return &datasource.Sotr{
				Idr:       sotr.Idr,
				Tabnum:    sotr.Tabnum,
				Name:      sotr.Name,
				MidName:   mid,
				Email:     email,
				Phone:     ConvKbv2Phone(sotr),
				Mobile:    ConvKbv2Mobile(sotr),
				Avatar:    sotr.Avatar,
				Grade:     sotr.Grade,
				Children:  sotr.Children,
				ParentIdr: sotr.ParentId,
			}
		}
	}

	return nil
}

func ConvKbv2Phone(sotr *kbv1.Sotr) (ds []datasource.Phone) {
	ds = []datasource.Phone{}
	if len(sotr.Phone) > 0 {
		for _, p := range sotr.Phone {
			ds = append(ds, datasource.Phone{Phone: p})
		}
	}
	return ds
}

func ConvKbv2Mobile(sotr *kbv1.Sotr) (ds []datasource.Mobile) {
	ds = []datasource.Mobile{}
	if len(sotr.Mobile) > 0 {
		for _, p := range sotr.Mobile {
			pdigit, err := strconv.Atoi(ExtractDigits(p))
			if err != nil {
				slog.Error("ConvKbv2Mobile:", "err", err)
			}
			ds = append(ds, datasource.Mobile{Mobile: uint(pdigit)})
		}
	}
	return ds
}

func ExtractDigits(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
