package pg

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/datasource"
	"github.com/mioxin/kbempgo/internal/utils"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	sotrID1 = uint(16)
	sotrID2 = uint(17)
	sotrID3 = uint(14)
	dsotrID = uint(1)

	oldPh = []datasource.Phone{
		{
			Phone:  "400-19-67",
			SotrID: &sotrID1,
		},
		{
			Phone:  "400-94-45",
			SotrID: &sotrID2,
		},
		{
			Phone:  "400-30-89",
			SotrID: &sotrID3,
			// SotrDeletedID: &dsotrID,
		},
	}

	mid     string = "Gulim.99999999@k.kom"
	newSotr        = &datasource.Sotr{
		Idr:    "s99999",
		Tabnum: "99999",
		Name:   "999999999 Гулим",
		Email:  &mid,
		Phone: []datasource.Phone{
			{
				Phone: "400-88888",
			},
		},
		Avatar:    "/avatar/25301.jpg",
		Grade:     "Главный Специалист",
		Children:  false,
		ParentIdr: "razd1.27.2935.69",
	}
)

func (st *DBTestSuite) Test_AddSotr() {

	st.loadDB(st.T())
	expectedCounts := st.counts(st.T())

	// load new sotr
	st.store.Sotrmap[newSotr.Tabnum] = newSotr.Conv2Kbv().GetSotr()
	st.loadDB(st.T())

	actualCounts := st.counts(st.T())
	expectedCounts.AddSotrs(1)
	expectedCounts.AddPhones(1)

	st.Assert().EqualValues(expectedCounts, actualCounts)
}

func (st *DBTestSuite) Test_History() {
	actualHist := []datasource.History{}

	for _, tc := range testHistCases {
		st.T().Run(tc.name, func(t *testing.T) {
			// load original sotrs
			st.loadDB(st.T())
			expectedCounts := st.counts(st.T())

			// update DB
			updateSotr(st.store.Sotrmap[tc.tabMutate], tc)
			st.loadDB(st.T())

			tc.updateCounts(expectedCounts)
			actualCounts := st.counts(st.T())
			st.Assert().EqualValues(expectedCounts, actualCounts)

			sotrID := st.store.Sotrmap[tc.tabMutate].Id
			r := st.store.DB.Where("sotr_id = ?", sotrID).Find(&actualHist)

			for i := range actualHist {
				actualHist[i].ID = 0
				actualHist[i].CreatedAt = *new(time.Time)
				actualHist[i].SotrID = nil
				actualHist[i].SotrDeletedID = nil
			}

			if st.Assert().NoError(r.Error) {
				st.Assert().EqualValues(tc.expectedHistories, actualHist)
			}

			// clear updates
			st.store.DB.Exec("DELETE FROM phones")
			st.store.DB.Exec("DELETE FROM mobiles")
			st.store.DB.Exec("DELETE FROM histories")
			st.store.DB.Exec("DELETE FROM sotrs")
			clear(st.store.Sotrmap)
		})

	}
}

func (st *DBTestSuite) Test_GetSotrsBy() {
	var (
		q   *kbv1.SotrRequest
		err error
	)
	expextedSotrs := &kbv1.SotrsResponse{}
	st.loadDB(st.T())

	for _, tc := range testGetSotrsCases {
		st.T().Run(tc.by, func(t *testing.T) {
			err = protojson.Unmarshal([]byte(tc.expected), expextedSotrs)
			st.Require().NoError(err)

			if i, ok := kbv1.SotrRequest_DBField_value[tc.by]; ok {
				q = &kbv1.SotrRequest{
					Field: kbv1.SotrRequest_DBField(i),
					Str:   tc.val,
				}
			} else {
				st.Assert().FailNow("invalid field name for get sotrs by", tc.by)
			}

			sotrs, err := st.store.GetSotrsBy(context.Background(), q)

			if !st.Assert().NoError(err) {
				return
			}
			if len(expextedSotrs.Sotrs) != len(sotrs) {
				return
			}
			sotrs[0].Id = 0
			sotrs[0].Date = nil
			exp := utils.ConvKbv2Ds(expextedSotrs.Sotrs[0])
			actual := utils.ConvKbv2Ds(sotrs[0])

			st.Assert().EqualValues(exp, actual)

		})
	}
}

func (st *DBTestSuite) Test_GetDepsBy() {
	var (
		q   *kbv1.DepRequest
		err error
	)
	expextedDeps := &kbv1.DepsResponse{}
	st.loadDB(st.T())

	for _, tc := range testGetDepsCases {
		st.T().Run(tc.by, func(t *testing.T) {
			err = protojson.Unmarshal([]byte(tc.expected), expextedDeps)
			st.Require().NoError(err)

			if i, ok := kbv1.DepRequest_DBField_value[tc.by]; ok {
				q = &kbv1.DepRequest{
					Field: kbv1.DepRequest_DBField(i),
					Str:   tc.val,
				}
			} else {
				st.Assert().FailNow("invalid field name for get Deps by", tc.by)
			}

			deps, err := st.store.GetDepsBy(context.Background(), q)

			if !st.Assert().NoError(err) {
				return
			}
			if len(expextedDeps.Deps) != len(deps) {
				return
			}
			deps[0].Id = 0
			exp := utils.ConvKbv2Ds(expextedDeps.Deps[0])
			actual := utils.ConvKbv2Ds(deps[0])

			st.Assert().EqualValues(exp, actual)

		})
	}
}

func updateSotr(s *kbv1.Sotr, tc histTest) {
	for fl, v := range tc.fieldsMutate {
		switch fl {
		case "phone":
			v = strings.ReplaceAll(v, " ", "")
			ph := strings.Split(v, ",")
			s.Phone = ph
		case "mobile":
			if v == "" {
				s.Mobile = nil
				continue
			}
			m := strings.Split(v, ",")
			for i, mstr := range m {
				if len(mstr) > 5 {
					m[i] = fmt.Sprintf("+%s (%s) %s", mstr[0:1], mstr[1:4], mstr[4:])
				}
			}
			s.Mobile = m
		case "name":
			s.Name = v
		case "grade":
			s.Grade = v
		case "idr":
			s.Idr = v
		case "email":
			s.Email = v
		case "parenr_idr":
			s.ParentId = v
		case "avatar":
			s.Avatar = v
		}
	}
}

func TestDBSuite(t *testing.T) {
	suite.Run(t, new(DBTestSuite))
}
