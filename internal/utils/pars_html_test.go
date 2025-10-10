package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/imroc/req/v3"
	"github.com/mioxin/kbempgo/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	sotrText string = `<div data-tabnum="1000380"  class="sotr_block">
						  <table cellpadding="0" cellspacing="0" width=100%>
							  <tr onMouseOver="tr_over(this)" onMouseOut="tr_out(this)" onClick="click_tr(this)"><td>
								  <table>
									  <tr><td height=5/></tr>
									  <tr>
										  <td width="5"rowspan=2/>
										  <td width="35" rowspan="2"><img src="/avatar/1000380.jpg?v=KMS6TdPNde" width=30></td>
										  <td width="300" class="s_1">ГасХХХХХХ Ольга<span class="s_1_1"></span> <span class="s_1_2"></span></td>

										  <td width="200" class="s_2"><span class="s_3">вн</span> <b>400-11-27</b></td>
										  <td width="130" class="s_2"></td>
										  <td width="300" class="s_2"><a href="mailto:Olga.Gasxxxxxx@xxxxx.kz" class="ln7">Olga.Gasxxxxxx@xxxxx.kz</a></td>
                                          <td width="50" rowspan="2"><span><i style='font-size: 18px; color:rgb(204, 204, 204); margin:0;' class='icon-home' title='Работает удаленно'></i></span></td>
                                          <td width="50" rowspan="2"><a href="?type=1788#/map/30/2378" target="_blank" class=sotr_ln2><img title="Место на карте" src="../image/sotr_point_ico.png"></a></td>
									  </tr>
									  <tr>
										  <td colspan="4"class="s_4">Главный бухгалтер</td>
									  </tr>
									  <tr><td height=5/></tr>
								  </table>
							  </td></tr>
						  </table>
					  </div>`
	midNameText string = `<div class=sotr_td3 onclick="searchG('Антропов Виталий Витальевич', 'sotrSearchList');">
					<table>
						<tr>
						    <td rowspan="2"><img width="26" style="margin-right: 4px; border-radius: 3px;" alt="" src="/avatar/99996324.jpg?v=YI2A7EeWq5" /></td>
							<td class="s_1"><span style="background:#fdff90">Антропов</span> Виталий Витальевич</td>
						</tr>
						<tr>
							<td class="s_3"><span class="s_3"></span> <b></b></td>
						</tr>
					</table>
				</div><div class=sotr_td3 onclick="searchG('Антропов Антон Викторович', 'sotrSearchList');">
					<table>
						<tr>
						    <td rowspan="2"><img width="26" style="margin-right: 4px; border-radius: 3px;" alt="" src="/avatar/12227.jpg?v=RpewGpkwpQ" /></td>
							<td class="s_1"><span style="background:#fdff90">Антропов</span> Антон Викторович</td>
						</tr>
						<tr>
							<td class="s_3"><span class="s_3">вн</span> <b>408-250</b></td>
						</tr>
					</table>
				</div><div class=s_1 style="border-bottom: 1px solid #eeeeee; cursor: pointer; display: flex; align-items: center;justify-content: space-around;padding: 15px 0;"><span class="s_1">← Сюда некуда</span> <span class="s_1">Туда некуда  →</span> </div>`

	sotr models.Sotr = models.Sotr{
		Name:   "Антропов Антон",
		Avatar: "/avatar/12227.jpg",
	}

	expect *models.Sotr = &models.Sotr{
		Tabnum: "1000380",
		Name:   "ГасХХХХХХ Ольга",
		Phone:  "400-11-27",
		Mobile: "", // "+7 (701) 872-11-11,+7 (701) 911-01-11",
		Email:  "Olga.Gasxxxxxx@xxxxx.kz",
		Avatar: "/avatar/1000380.jpg",
		Grade:  "Главный бухгалтер",
	}
	expectedMidName string = "Викторович"
)

func TestParseSotrRe(t *testing.T) {
	sotr, err := ParseSotrRe(sotrText)

	require.NoError(t, err)
	assert.Equal(t, expect, sotr)
}

func TestParseSotr(t *testing.T) {
	sotr := ParseSotr(sotrText)

	assert.Equal(t, expect, sotr)
}

func TestParseMidName(t *testing.T) {
	mid := ParseMidName(&sotr, midNameText)
	assert.Equal(t, expectedMidName, mid)

}

func BenchmarkParseSotrRegexp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ParseSotrRe(sotrText)
	}
}

func BenchmarkParseSotrStrings(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ParseSotr(sotrText)
	}
}

var fname []string = []string{
	"/mnt/c/Arch/UTIL/curl/bin/1.html",
	"/mnt/c/Arch/UTIL/curl/bin/10.html",
	"/mnt/c/Arch/UTIL/curl/bin/11.html",
	"/mnt/c/Arch/UTIL/curl/bin/12.html",
	"/mnt/c/Arch/UTIL/curl/bin/2.html",
	"/mnt/c/Arch/UTIL/curl/bin/3.html",
	"/mnt/c/Arch/UTIL/curl/bin/4.html",
	"/mnt/c/Arch/UTIL/curl/bin/5.html",
	"/mnt/c/Arch/UTIL/curl/bin/6.html",
	"/mnt/c/Arch/UTIL/curl/bin/7.html",
	"/mnt/c/Arch/UTIL/curl/bin/8.html",
	"/mnt/c/Arch/UTIL/curl/bin/9.html",
}

func TestCheckSotr(t *testing.T) {

	users, err := getSotr()
	require.NoError(t, err)
	require.Less(t, 0, len(users))

	onDiv := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		i := strings.Index(string(data), "class=div5b")
		if i == -1 {
			if !atEOF {
				return 0, nil, nil
			}
			return 0, data, bufio.ErrFinalToken
		}
		// Otherwise, return the token before .
		return i + 1, data[:i], nil
	}

	lookFIO := func(count int, fn string) int {
		file, err := os.Open(fn)
		require.NoError(t, err)

		defer file.Close()

		scan := bufio.NewScanner(file)

		scan.Split(onDiv)

		for scan.Scan() {
			sotrText = html.UnescapeString(scan.Text())
			tabnum := FindBetween(sotrText, `onclick="opencard('`, `')"`)

			if tabnum == "" {
				continue
			}
			fio := FindBetween(sotrText, `class="ln4">`, `</a>`)
			dep := FindBetween(sotrText, `color:#666;">`, `</span>`)

			_, ok := users[tabnum]
			if ok {
				continue
			}

			fmt.Printf("%d:\t%s\t%s\t%s\n", count, tabnum, fio, dep)
			count++
		}

		return count
	}

	count := 1
	for i, fn := range fname {
		if i > 12 {
			break
		}
		count = lookFIO(count, fn)
	}
}

func getSotr() (map[string]*models.Sotr, error) {

	users := make(map[string]*models.Sotr, 1000)
	file, err := os.Open("../../.tmp/sotr.json")
	if err != nil {
		return nil, err
	}

	defer file.Close()

	scan := bufio.NewScanner(file)

	for scan.Scan() {
		text := scan.Text()
		if err := scan.Err(); err != nil {
			fmt.Println("reading standard input:", err)
		}
		user := models.Sotr{}
		err := json.Unmarshal([]byte(text), &user)
		if err != nil {
			fmt.Printf("err unmarshall: %v\n", err)
			continue
		}
		users[user.Tabnum] = &user
	}
	return users, nil
}

type ErrorMessage struct {
	Message string `json:"message"`
}

// func TestCheckAvater(t *testing.T) {
// 	wg := &sync.WaitGroup{}

// 	workers := 20
// 	clientsPool := clientpool.NewClientsPool(workers, 1)

// 	users, err := getSotr()
// 	require.NoError(t, err)
// 	require.Less(t, 0, len(users))

// 	count := 1
// 	for name, u := range users {
// 		// if count > 10 {
// 		// 	break
// 		// }
// 		cli := clientsPool.Get()
// 		cli.SetBaseURL("https://my.kaspi.kz").
// 			SetTimeout(5 * time.Second).
// 			SetOutputDirectory("../../.kbemp-store")

// 		wg.Add(1)

// 		go func() {
// 			defer clientsPool.Push(cli)
// 			defer wg.Done()
// 			download(cli, u, name, count)
// 		}()

// 		count++
// 	}
// 	wg.Wait()
// }

func download(cli *req.Client, u *models.Sotr, name string, count int) {
	var errMsg ErrorMessage

	filename := filepath.Join("../../.kbemp-store", u.Avatar)
	f, err := os.Stat(filename)

	if err == nil && !f.IsDir() {
		r := cli.Head(u.Avatar).
			Do()

		if r.Err != nil {
			fmt.Println(r.Err.Error(), u.Avatar)
		}
		if r.ContentLength == f.Size() {
			fmt.Println(count, ": >>>Skip existing file:", u.Avatar)
			return
		}
	}

	callback := func(info req.DownloadInfo) {
		if info.Response.Response != nil {
			fmt.Printf("downloaded %.2f%% (%s)\n", float64(info.DownloadedSize)/float64(info.Response.ContentLength)*100.0, info.Response.Request.URL.String())
		}
	}

	fl, _ := strings.CutPrefix(u.Avatar, "/")

	resp, err := cli.R().
		SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
		SetOutputFile(fl).
		SetDownloadCallback(callback).
		Get(u.Avatar)
	if err != nil { // Error handling.
		fmt.Println("Worker:", "error handling", err)
	}

	if resp.IsErrorState() { // Status code >= 400.
		fmt.Println("Worker:", "err", errMsg.Message) // Record error message returned.
	}

	if resp.IsSuccessState() { // Status code is between 200 and 299.

		fmt.Printf("%d: %s  %s %d byte\n", count, name, u.Avatar, resp.ContentLength)
	}

}
