package utils

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
)

func FindBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}

	i += len(start)

	j := strings.Index(s[i:], end)
	if j < 0 {
		return ""
	}

	return s[i : i+j]
}

// parsing by find string index
func ParseSotr(unescaped string) *kbv1.Sotr {
	var p, m []string
	// Tabnum
	tabnum := FindBetween(unescaped, `data-tabnum="`, `"`)

	// Avatar
	avatar := FindBetween(unescaped, `<img src="`, `"`)
	// Cut avatar query-param
	if idx := strings.Index(avatar, "?"); idx != -1 {
		avatar = avatar[:idx]
	}

	// FIO
	fio := FindBetween(unescaped, `<td width="300" class="s_1">`, `<span`)
	fio = strings.TrimSpace(fio)

	// Phone
	phone := FindBetween(unescaped, `<span class="s_3">вн</span> <b>`, "</b>")
	phone = strings.TrimSpace(phone)
	if phone != "" {
		p = strings.Split(phone, ",")
	}

	// Mobile
	mobile := FindBetween(unescaped, `<td width="130" class="s_2">`, "</td>")
	mobile = strings.TrimSpace(mobile)
	if mobile != "" {
		m = strings.Split(mobile, ",")
	}

	// Email
	email := FindBetween(unescaped, `<a href="mailto:`, `"`)
	email = strings.TrimSpace(email)

	// Grade
	grade := FindBetween(unescaped, `<td colspan="4"class="s_4">`, "</td>")
	grade = strings.TrimSpace(grade)

	return &kbv1.Sotr{
		Tabnum: tabnum,
		Name:   fio,
		Phone:  p,
		Mobile: m,
		Email:  email,
		Avatar: avatar,
		Grade:  grade,
	}
}

// parsing mid name
func ParseMidName(sotr *kbv1.Sotr, unescaped string) string {
	slText := strings.Split(unescaped, "</div><div class=sotr_td3")

	for _, t := range slText {
		// Avatar
		avatar := FindBetween(t, `alt="" src="`, `"`)
		// Cut avatar query-param
		if idx := strings.Index(avatar, "?"); idx != -1 {
			avatar = avatar[:idx]
		}
		// FIO
		fio := FindBetween(t, `onclick="searchG('`, `', 'SotrsResponseearchList')`)
		fio = strings.TrimSpace(fio)
		mid, ok := strings.CutPrefix(fio, sotr.Name)
		// slog.Debug("PARSE MIDNAME:", "mid", mid, "fio", fio, "avatar", avatar, "sotr.avatar", sotr.Avatar)
		if ok && avatar == sotr.Avatar {
			return strings.TrimSpace(mid)
		}
	}

	return ""
}

type Mobile struct {
	Data    string
	Success bool
}

// parsing mobile
func ParseMobile(unescaped string) (*Mobile, error) {
	m := new(Mobile)
	err := json.Unmarshal([]byte(unescaped), m)
	m.Data = strings.Trim(m.Data, " \n\r\t")

	return m, err
}

// parsing by regexp
func ParseSotrRe(htmlStr string) (*kbv1.Sotr, error) {
	var p, m []string

	unescaped := html.UnescapeString(htmlStr)

	tabnumRe := regexp.MustCompile(`data-tabnum="(\d+)"`)
	fioRe := regexp.MustCompile(`<td[^>]*class="s_1"[^>]*>([^<]+)`)
	phoneRe := regexp.MustCompile(`<td[^>]*class="s_2"[^>]*>.*?<b>([^<]+)</b>`)
	mobileRe := regexp.MustCompile(`<td[^>]*class="s_2"[^>]*>(\+7[^\<]+(?:,\+7[^\<]+)*)`)
	emailRe := regexp.MustCompile(`<a href="mailto:([^"]+)"[^>]*>`)
	avatarRe := regexp.MustCompile(`<img src="([^"]+\.jpg)`)
	gradeRe := regexp.MustCompile(`<td colspan="4"[^>]*class="s_4"[^>]*>([^<]+)`)

	tabnum := findFirst(tabnumRe, unescaped)
	fio := strings.TrimSpace(findFirst(fioRe, unescaped))
	phone := findFirst(phoneRe, unescaped)
	if phone != "" {
		p = strings.Split(phone, ",")
	}

	mobile := findFirst(mobileRe, unescaped)
	if mobile != "" {
		m = strings.Split(mobile, ",")
	}
	email := findFirst(emailRe, unescaped)
	avatar := findFirst(avatarRe, unescaped)
	grade := strings.TrimSpace(findFirst(gradeRe, unescaped))

	return &kbv1.Sotr{
		Tabnum: tabnum,
		Name:   fio,
		Phone:  p,
		Mobile: m,
		Email:  email,
		Avatar: avatar,
		Grade:  grade,
	}, nil
}

func findFirst(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return m[1]
	}

	return ""
}
