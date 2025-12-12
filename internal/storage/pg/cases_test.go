package pg

import (
	"github.com/mioxin/kbempgo/internal/datasource"
)

type histTest struct {
	name, tabMutate   string
	fieldsMutate      map[string]string
	expectedHistories []datasource.History
	updateCounts      func(c *Counts)
}

type getTest struct {
	by       string
	val      string
	expected string
}

var (
	testHistCases = []histTest{
		{
			name:      "add phone",
			tabMutate: "2681",
			fieldsMutate: map[string]string{
				"phone": "000-00-00, 111-11-11",
			},
			expectedHistories: []datasource.History{
				{
					Field:    "phone",
					OldValue: "400-16-32",
				},
			},
			updateCounts: func(c *Counts) {
				c.AddPhones(1)
				c.AddHistories(1)
			},
		},
		{
			name:      "decr phone,mobile",
			tabMutate: "1122",
			fieldsMutate: map[string]string{
				"phone":  "000-00-00",
				"mobile": "",
			},
			expectedHistories: []datasource.History{
				{
					Field:    "phone",
					OldValue: "400-99-91,000-00-00",
				},
				{
					Field:    "mobile",
					OldValue: "+7 (701) 0006080",
				},
			},
			updateCounts: func(c *Counts) {
				c.AddHistories(2)
				c.AddPhones(-1)
				c.AddMobiles(-1)
			},
		},
		{
			name:      "decr mobile",
			tabMutate: "60609",
			fieldsMutate: map[string]string{
				"mobile": "+7 (701) 000-67-01",
			},
			expectedHistories: []datasource.History{
				{
					Field:    "mobile",
					OldValue: "+7 (701) 0006700,+7 (701) 0006701",
				},
			},
			updateCounts: func(c *Counts) {
				c.AddMobiles(-1)
				c.AddHistories(1)
			},
		},
		{
			name:      "change name, avatar, grade",
			tabMutate: "60609",
			fieldsMutate: map[string]string{
				"name":   "Са5555 Асемгуль",
				"avatar": "/avatar/9999.jpg",
				"grade":  "LLLLLLLL",
			},
			expectedHistories: []datasource.History{
				{
					Field:    "name",
					OldValue: "Са44444 Асемгуль",
				},
				{
					Field:    "avatar",
					OldValue: "/avatar/60609.jpg",
				},
				{
					Field:    "grade",
					OldValue: "Главный Специалист",
				},
			},
			updateCounts: func(c *Counts) {
				c.AddHistories(3)
			},
		},
	}
)

var (
	testGetSotrsCases = []getTest{
		{
			by:       "IDR",
			val:      "sotr4918",
			expected: `{"sotrs":[{"idr":"sotr4918","tabnum":"60609","name":"Са44444 Асемгуль","midName":"Абатовна","phone":[],"mobile":["+7 (701) 0006700","+7 (701) 0006701"],"email":"Assemgul@k.kom","avatar":"/avatar/60609.jpg","grade":"Главный Специалист","children":false,"parentId":"razd1.27.2935.37.70","date":null}]}`,
		},
		{
			by:       "MOBILE",
			val:      "+7 (701) 000-67-00",
			expected: `{"sotrs":[{"idr":"sotr4918","tabnum":"60609","name":"Са44444 Асемгуль","midName":"Абатовна","phone":[],"mobile":["+7 (701) 0006700","+7 (701) 0006701"],"email":"Assemgul@k.kom","avatar":"/avatar/60609.jpg","grade":"Главный Специалист","children":false,"parentId":"razd1.27.2935.37.70","date":null}]}`,
		},
		{
			by:       "FIO",
			val:      "Са44444 Асемгуль Абатовна",
			expected: `{"sotrs":[{"idr":"sotr4918","tabnum":"60609","name":"Са44444 Асемгуль","midName":"Абатовна","phone":[],"mobile":["+7 (701) 0006700","+7 (701) 0006701"],"email":"Assemgul@k.kom","avatar":"/avatar/60609.jpg","grade":"Главный Специалист","children":false,"parentId":"razd1.27.2935.37.70","date":null}]}`,
		},
		{
			by:       "FIO",
			val:      "Са44444 Асемгуль",
			expected: `{"sotrs":[{"idr":"sotr4918","tabnum":"60609","name":"Са44444 Асемгуль","midName":"Абатовна","phone":[],"mobile":["+7 (701) 0006700","+7 (701) 0006701"],"email":"Assemgul@k.kom","avatar":"/avatar/60609.jpg","grade":"Главный Специалист","children":false,"parentId":"razd1.27.2935.37.70","date":null}]}`,
		},
		{
			by:       "FIO",
			val:      "Та4444 Сабина",
			expected: `{"sotrs":[{"idr":"sotr5590","tabnum":"52957","name":"Та4444 Сабина","midName":"","phone":["400-30-89"],"mobile":[],"email":"Sabina@k.kom","avatar":"/avatar/52957.jpg","grade":"Начальник Отдела","children":false,"parentId":"razd1.27.2935.37.70","date":null}]}`,
		},
		{
			by:  "TABNUM",
			val: "52957",
			expected: `{"sotrs":[{"idr":"sotr5590","tabnum":"52957","name":"Та4444 Сабина","midName":"Даулеткалиевна","phone":["400-30-89"],"mobile":[],"email":"Sabina@k.kom","avatar":"/avatar/52957.jpg","grade":"Начальник Отдела","children":false,"parentId":"razd1.27.2935.37.70","date":null},
{"idr":"sotr4918","tabnum":"60609","name":"Са44444 Асемгуль","midName":"Абатовна","phone":[],"mobile":["+7 (701) 0006700","+7 (701) 0006701"],"email":"Assemgul@k.kom","avatar":"/avatar/60609.jpg","grade":"Главный Специалист","children":false,"parentId":"razd1.27.2935.37.70","date":null}
]}`,
		},
	}

	testGetDepsCases = []getTest{
		{
			by:       "IDR",
			val:      "razd1.27.2935.69",
			expected: `{"deps":[{"idr":"razd1.27.2935.69","parent":"razd1.27.2935","text":"Отдел экспортно-импортных операций","children":true}]}`,
		},
		{
			by:  "PARENT",
			val: "razd1.27.2935",
			expected: `{"deps":[{"idr":"razd1.27.2935.37","parent":"razd1.27.2935","text":"Управление финансовых институтов","children":true},
{"idr":"razd1.27.2935.3849","parent":"razd1.27.2935","text":"Управление по Работе с Рынками Капитала","children":true},
{"idr":"razd1.27.2935.69","parent":"razd1.27.2935","text":"Отдел экспортно-импортных операций","children":true}
]}`,
		},
	}
)
