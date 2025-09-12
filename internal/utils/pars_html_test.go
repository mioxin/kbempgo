package utils

import (
	"testing"

	"github.com/mioxin/kbempgo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var str string = "\u003cdiv data-tabnum=\"1000380\"  class=\"sotr_block\"\u003e\r\n\t\t\t\t\t\t  \u003ctable cellpadding=\"0\" cellspacing=\"0\" width=100%\u003e\r\n\t\t\t\t\t\t\t  \u003ctr onMouseOver=\"tr_over(this)\" onMouseOut=\"tr_out(this)\" onClick=\"click_tr(this)\"\u003e\u003ctd\u003e\r\n\t\t\t\t\t\t\t\t  \u003ctable\u003e\r\n\t\t\t\t\t\t\t\t\t  \u003ctr\u003e\u003ctd height=5/\u003e\u003c/tr\u003e\r\n\t\t\t\t\t\t\t\t\t  \u003ctr\u003e\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd width=\"5\"rowspan=2/\u003e\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd width=\"35\" rowspan=\"2\"\u003e\u003cimg src=\"/avatar/1000380.jpg?v=KMS6TdPNde\" width=30\u003e\u003c/td\u003e\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd width=\"300\" class=\"s_1\"\u003eГасХХХХХХ Ольга\u003cspan class=\"s_1_1\"\u003e\u003c/span\u003e \u003cspan class=\"s_1_2\"\u003e\u003c/span\u003e\u003c/td\u003e\r\n\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd width=\"200\" class=\"s_2\"\u003e\u003cspan class=\"s_3\"\u003eвн\u003c/span\u003e \u003cb\u003e100-11-11\u003c/b\u003e\u003c/td\u003e\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd width=\"130\" class=\"s_2\"\u003e+7 (701) 872-11-11,+7 (701) 911-01-11\u003c/td\u003e\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd width=\"300\" class=\"s_2\"\u003e\u003ca href=\"mailto:Olga.Gasxxxxxx@xxxxx.kz\" class=\"ln7\"\u003eOlga.Gasxxxxxx@xxxxx.kz\u003c/a\u003e\u003c/td\u003e\r\n                                          \u003ctd width=\"50\" rowspan=\"2\"\u003e\u003cspan\u003e\u003c/span\u003e\u003c/td\u003e\r\n                                          \u003ctd width=\"50\" rowspan=\"2\"\u003e\u003ca href=\"?type=1788#/map/30/2378\" target=\"_blank\" class=sotr_ln2\u003e\u003cimg title=\"Место на карте\" src=\"../image/sotr_point_ico.png\"\u003e\u003c/a\u003e\u003c/td\u003e\r\n\t\t\t\t\t\t\t\t\t  \u003c/tr\u003e\r\n\t\t\t\t\t\t\t\t\t  \u003ctr\u003e\r\n\t\t\t\t\t\t\t\t\t\t  \u003ctd colspan=\"4\"class=\"s_4\"\u003eГлавный бухгалтер\u003c/td\u003e\r\n\t\t\t\t\t\t\t\t\t  \u003c/tr\u003e\r\n\t\t\t\t\t\t\t\t\t  \u003ctr\u003e\u003ctd height=5/\u003e\u003c/tr\u003e\r\n\t\t\t\t\t\t\t\t  \u003c/table\u003e\r\n\t\t\t\t\t\t\t  \u003c/td\u003e\u003c/tr\u003e\r\n\t\t\t\t\t\t  \u003c/table\u003e\r\n\t\t\t\t\t  \u003c/div\u003e"
var expect *models.Sotr = &models.Sotr{
	Tabnum: "1000380",
	Fio:    "ГасХХХХХХ Ольга",
	Phone:  "100-11-11",
	Mobile: "+7 (701) 872-11-11,+7 (701) 911-01-11",
	Email:  "Olga.Gasxxxxxx@xxxxx.kz",
	Avatar: "/avatar/1000380.jpg",
	Grade:  "Главный бухгалтер",
}

func TestParseSotrRe(t *testing.T) {
	sotr, err := ParseSotrRe(str)

	require.NoError(t, err)
	assert.Equal(t, expect, sotr)
}

func TestParseSotr(t *testing.T) {
	sotr := ParseSotr(str)

	assert.Equal(t, expect, sotr)
}

func BenchmarkParseSotrRegexp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ParseSotrRe(str)
	}
}

func BenchmarkParseSotrStrings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ParseSotr(str)
	}
}
