package kbv1

import (
	"maps"
)

type Diff struct {
	FieldName string
	Val       any
}

func CompareSotr(oldSotr *Sotr, newSotr *Sotr) ([]*Diff, error) {

	diffs := make([]*Diff, 0)

	for _, f := range []string{"Name", "Phone", "Mobile", "Email", "Avatar", "Grade", "ParentId"} {
		switch f {
		case "Name":
			if oldSotr.Name != newSotr.Name {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.Name})
			}
		case "Phone":
			old := slice2map(oldSotr.Phone)
			new := slice2map(newSotr.Phone)
			if !maps.Equal(old, new) {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.Phone})
			}
		case "Mobile":
			old := slice2map(oldSotr.Mobile)
			new := slice2map(newSotr.Mobile)
			if !maps.Equal(old, new) {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.Mobile})
			}
		case "Email":
			if oldSotr.Email != newSotr.Email {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.Email})
			}
		case "Avatar":
			if oldSotr.Avatar != newSotr.Avatar {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.Avatar})
			}
		case "Grade":
			if oldSotr.Grade != newSotr.Grade {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.Grade})
			}
		case "ParentId":
			if oldSotr.ParentId != newSotr.ParentId {
				diffs = append(diffs, &Diff{FieldName: f, Val: oldSotr.ParentId})
			}
		}
	}
	return diffs, nil
}

func slice2map[T comparable](sl []T) map[T]struct{} {
	m := make(map[T]struct{})
	for _, v := range sl {
		m[v] = struct{}{}
	}
	return m
}
