package utils

import (
	"html"
	"regexp"
	"strings"

	"github.com/mioxin/kbempgo/internal/models"
)

func findBetween(s, start, end string) string {
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
func ParseSotr(htmlStr string) *models.Sotr {
	unescaped := html.UnescapeString(htmlStr)

	// Tabnum
	tabnum := findBetween(unescaped, `data-tabnum="`, `"`)

	// Avatar
	avatar := findBetween(unescaped, `<img src="`, `"`)
	// Cut avatar query-param
	if idx := strings.Index(avatar, "?"); idx != -1 {
		avatar = avatar[:idx]
	}

	// FIO
	fio := findBetween(unescaped, `<td width="300" class="s_1">`, `<span`)
	fio = strings.TrimSpace(fio)

	// Phone
	phone := findBetween(unescaped, `<span class="s_3">вн</span> <b>`, "</b>")
	phone = strings.TrimSpace(phone)

	// Mobile
	mobile := findBetween(unescaped, `<td width="130" class="s_2">`, "</td>")
	mobile = strings.TrimSpace(mobile)

	// Email
	email := findBetween(unescaped, `<a href="mailto:`, `"`)
	email = strings.TrimSpace(email)

	// Grade
	grade := findBetween(unescaped, `<td colspan="4"class="s_4">`, "</td>")
	grade = strings.TrimSpace(grade)

	return &models.Sotr{
		Tabnum: tabnum,
		Fio:    fio,
		Phone:  phone,
		Mobile: mobile,
		Email:  email,
		Avatar: avatar,
		Grade:  grade,
	}
}

// parsing by regexp
func ParseSotrRe(htmlStr string) (*models.Sotr, error) {
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
	mobile := findFirst(mobileRe, unescaped)
	email := findFirst(emailRe, unescaped)
	avatar := findFirst(avatarRe, unescaped)
	grade := strings.TrimSpace(findFirst(gradeRe, unescaped))

	return &models.Sotr{
		Tabnum: tabnum,
		Fio:    fio,
		Phone:  phone,
		Mobile: mobile,
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
