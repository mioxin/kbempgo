package utils

import (
	"bufio"
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	kbv1 "github.com/mioxin/kbempgo/api/kbemp/v1"
	"github.com/mioxin/kbempgo/internal/storage/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	MainURL  string = "https://my.kaspi.kz"
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
	midNameText []string = []string{`<div class=sotr_td3 onclick="searchG('Антропов Виталий Витальевич', 'SotrsResponseearchList');">
	<table>
		<tr>
		    <td rowspan="2"><img width="26" style="margin-right: 4px; border-radius: 3px;" alt="" src="/avatar/99996324.jpg?v=YI2A7EeWq5" /></td>
			<td class="s_1"><span style="background:#fdff90">Антропов</span> Виталий Витальевич</td>
		</tr>
		<tr>
			<td class="s_3"><span class="s_3"></span> <b></b></td>
		</tr>
	</table>
</div><div class=sotr_td3 onclick="searchG('Антропов Антон Викторович', 'SotrsResponseearchList');">
	<table>
		<tr>
		    <td rowspan="2"><img width="26" style="margin-right: 4px; border-radius: 3px;" alt="" src="/avatar/12227.jpg?v=RpewGpkwpQ" /></td>
			<td class="s_1"><span style="background:#fdff90">Антропов</span> Антон Викторович</td>
		</tr>
		<tr>
			<td class="s_3"><span class="s_3">вн</span> <b>408-250</b></td>
		</tr>
	</table>
</div><div class=s_1 style="border-bottom: 1px solid #eeeeee; cursor: pointer; display: flex; align-items: center;justify-content: space-around;padding: 15px 0;"><span class="s_1">← Сюда некуда</span> <span class="s_1">Туда некуда  →</span> </div>`,
		`<div class=sotr_td3 onclick="searchG('Палий Юлия Викторовна', 'sotrSearchList');">
					<table>
						<tr>
						    <td rowspan="2"><img width="26" style="margin-right: 4px; border-radius: 3px;" alt="" src="/avatar/2681.jpg?v=48H33Koas2" /></td>
							<td class="s_1"><span style="background:#fdff90">Палий Юлия</span> Викторовна</td>
						</tr>
						<tr>
							<td class="s_3"><span class="s_3">вн</span> <b>400-16-02</b></td>
						</tr>
					</table>
				</div><div class=s_1 style="border-bottom: 1px solid #eeeeee; cursor: pointer; display: flex; align-items: center;justify-content: space-around;padding: 15px 0;"><span class="s_1">← Сюда некуда</span> <span class="s_1">Туда некуда  →</span> </div>`,
	}
	sotr []kbv1.Sotr = []kbv1.Sotr{
		{
			Name:   "Антропов Антон",
			Avatar: "/avatar/12227.jpg",
		},
		{
			Name:   "Палий Юлия",
			Avatar: "/avatar/2681.jpg",
		},
	}

	expect *kbv1.Sotr = &kbv1.Sotr{
		Tabnum: "1000380",
		Name:   "ГасХХХХХХ Ольга",
		Phone:  []string{"400-11-27"},
		Mobile: nil, // "+7 (701) 872-11-11,+7 (701) 911-01-11",
		Email:  "Olga.Gasxxxxxx@xxxxx.kz",
		Avatar: "/avatar/1000380.jpg",
		Grade:  "Главный бухгалтер",
	}
	expectedMidName []string = []string{"Викторович", "Викторовна"}
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
	for i := range []int{1, 2} {
		mid := ParseMidName(&sotr[i], midNameText[i])
		assert.Equal(t, expectedMidName[i], mid)
	}
}

type TestCase struct {
	name     string
	text     string
	expected Mobile
}

func TestParseMobile(t *testing.T) {
	tc := []TestCase{
		{
			name:     "Mobile exists",
			text:     `{"success": true, "data": "+7 (775) 912-50-60"}`,
			expected: Mobile{Data: "+7 (775) 912-50-60", Success: true},
		},
		{
			name:     "Mobile not exists",
			text:     `{"success": true, "data": "+7 (775) 912-50-60"}`,
			expected: Mobile{Data: "+7 (775) 912-50-60", Success: true},
		},
	}

	for _, tst := range tc {
		t.Run(tst.name, func(t *testing.T) {
			mob, ok := ParseMobile(tst.text)
			assert.NoError(t, ok)
			assert.Equal(t, tst.expected, *mob)
		})
	}
}

func TestHasValidMobile(t *testing.T) {
	tests := []string{
		`{"success":true,"user":{"place":{"place_id":6190,"map_id":69},"placeLink":"https:\/\/www.com","access":true,"phones":["78"],"cansee":false,"canshow":false,"current_user":false,"target_blank":false,"editLink":null,"avatar":"https:\/\/www.com3tdImWeui1","absent":false,"reviews":null,"reviews_link":"\/?type=1610 &r_id=999940952","login":null,"tab_num":null,"motiw_login":null,"motiw_id":null,"canSeeLogins":false,"short_name":"","depth":null,"depth0":null,"depdata":{"url":"\/?type=1297&path=1941.2256","name":"Product Office"},"posts":["Product Manager"],"email":"","mobile":"+7 (747)","hobbyes":null,"isNotKaspiGid":true,"remote":false,"hide_vpn":false,"hide_vacation":false,"hide_numbers":false,"hide_for_deps":false,"birthday":"3 \u0438\u044e\u043b\u044f"}}`,
		`{"success":true,"user":{"place":{"place_id":6177,"map_id":69},"placeLink":"https:\/\www.com","access":true,"phones":["97"],"cansee":false,"canshow":false,"current_user":false,"target_blank":false,"editLink":null,"avatar":"https:\/\/www.comy9F64Ev7kG","absent":false,"reviews":null,"reviews_link":"\/?type=1610 &r_id=1000465","login":null,"tab_num":null,"motiw_login":null,"motiw_id":null,"canSeeLogins":false,"short_name":"","depth":null,"depth0":null,"depdata":{"url":"\/?type=1297&path=1941.2256","name":"Product Office"},"posts":["Head of Product"],"email":"","mobile":" ","hobbyes":null,"isNotKaspiGid":true,"remote":false,"hide_vpn":false,"hide_vacation":false,"hide_numbers":false,"hide_for_deps":false,"birthday":"25 \u0434\u0435\u043a\u0430\u0431\u0440\u044f"}}`,
		`{"success":true,"user":{"place":false,"placeLink":false,"access":true,"phones":[],"cansee":false,"canshow":false,"current_user":false,"target_blank":false,"editLink":null,"avatar":"https:\/\/www.comSGV8W1iXKY","absent":false,"reviews":null,"reviews_link":"\/?type=1610 &r_id=8724","login":null,"tab_num":null,"motiw_login":null,"motiw_id":null,"canSeeLogins":false,"short_name":"","depth":null,"depth0":null,"depdata":{"url":"\/?type=1297&path=1941","name":""},"posts":["","Head of Kaspi Travel"],"email":"","mobile":"","hobbyes":null,"isNotKaspiGid":true,"remote":false,"hide_vpn":false,"hide_vacation":false,"hide_numbers":false,"hide_for_deps":false,"birthday":"1 \u0430\u0432\u0433\u0443\u0441\u0442\u0430"}}`,
	}
	expected := []bool{true, false, false}

	for i, str := range tests {
		assert.Equal(t, expected[i], HasValidMobile(str))
	}
}

func BenchmarkParseSotrRegexp(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ParseSotrRe(sotrText)
	}
}

func BenchmarkParseSotrsResponsetrings(b *testing.B) {
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

	t.Skip("test for local test a scraped data")

	stor, err := file.NewFileStore("../../.tmp", slog.Default())

	require.NoError(t, err)
	defer stor.Close()

	s, err := stor.GetSotrsBy(context.TODO(), &kbv1.SotrRequest{Field: kbv1.SotrRequest_NONE, Str: ""})
	if err != io.EOF {
		require.NoError(t, err)
	}

	users, err := ToMap(s)
	require.NoError(t, err)
	require.LessOrEqual(t, 0, len(users))

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

		defer file.Close() // nolint

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

func ToMap(sl []*kbv1.Sotr) (map[string]*kbv1.Sotr, error) {
	users := make(map[string]*kbv1.Sotr)

	for _, v := range sl {
		users[v.Tabnum] = v
	}
	return users, nil
}

// func getSotr(f string) (map[string]*kbv1.Sotr, error) {
// 	users := make(map[string]*kbv1.Sotr, 1000)

// 	file, err := os.Open(f)
// 	if err != nil {
// 		return nil, err
// 	}

// 	defer file.Close() // nolint

// 	scan := bufio.NewScanner(file)

// 	for scan.Scan() {
// 		text := scan.Text()

// 		if err := scan.Err(); err != nil {
// 			fmt.Println("reading standard input:", err)
// 		}

// 		user := datasource.Sotr{}

// 		err := json.Unmarshal([]byte(text), &user)
// 		if err != nil {
// 			fmt.Printf("err unmarshall: %v\ntext: %s", err, text)
// 			continue
// 		}

// 		users[user.Tabnum] = &kbv1.Sotr{
// 			Id:       0,
// 			Idr:      user.Idr,
// 			Name:     user.Name,
// 			MidName:  user.MidName,
// 			Tabnum:   user.Tabnum,
// 			Phone:    strings.Split(user.Phone, ","),
// 			Mobile:   strings.Split(user.Mobile, ","),
// 			Email:    user.Email,
// 			Avatar:   user.Avatar,
// 			Grade:    user.Grade,
// 			Children: user.Children,
// 			ParentId: user.ParentID,
// 			Date:     timestamppb.Now(),
// 		}
// 	}

// 	return users, nil
// }

// type ErrorMessage struct {
// 	Message string `json:"message"`
// }

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
// 		cli.SetBaseURL(MainURL).
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

// func download(cli *req.Client, u *kbv1.Sotr, name string, count int) {
// 	var errMsg ErrorMessage

// 	filename := filepath.Join("../../.kbemp-store", u.Avatar)
// 	f, err := os.Stat(filename)

// 	if err == nil && !f.IsDir() {
// 		r := cli.Head(u.Avatar).
// 			Do()

// 		if r.Err != nil {
// 			fmt.Println(r.Err.Error(), u.Avatar)
// 		}

// 		if r.ContentLength == f.Size() {
// 			fmt.Println(count, ": >>>Skip existing file:", u.Avatar)
// 			return
// 		}
// 	}

// 	callback := func(info req.DownloadInfo) {
// 		if info.Response.Response != nil {
// 			fmt.Printf("downloaded %.2f%% (%s)\n", float64(info.DownloadedSize)/float64(info.Response.ContentLength)*100.0, info.Response.Request.URL.String())
// 		}
// 	}

// 	fl, _ := strings.CutPrefix(u.Avatar, "/")

// 	resp, err := cli.R().
// 		SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
// 		SetOutputFile(fl).
// 		SetDownloadCallback(callback).
// 		Get(u.Avatar)
// 	if err != nil { // Error handling.
// 		fmt.Println("Worker:", "error handling", err)
// 	}

// 	if resp.IsErrorState() { // Status code >= 400.
// 		fmt.Println("Worker:", "err", errMsg.Message) // Record error message returned.
// 	}

// 	if resp.IsSuccessState() { // Status code is between 200 and 299.
// 		fmt.Printf("%d: %s  %s %d byte\n", count, name, u.Avatar, resp.ContentLength)
// 	}
// }

// func TestUpdateMobile(t *testing.T) {
// 	var (
// 		errMsg ErrorMessage
// 		mt     sync.Mutex
// 	)
// 	fPath := filepath.Join("../../.kbemp-store", "sotr.json")

// 	flS, err := os.OpenFile(fPath, os.O_WRONLY|os.O_CREATE, 0644)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	defer func() {
// 		e := flS.Close()
// 		if e != nil {
// 			fmt.Printf("%v\n", e)
// 		}
// 	}()

// 	wrSotr := bufio.NewWriter(flS)
// 	defer func() {
// 		e1 := wrSotr.Flush()
// 		if e1 != nil {
// 			fmt.Printf("%v\n", e1)
// 		}
// 	}()

// 	wg := &sync.WaitGroup{}

// 	workers := 20
// 	clientsPool := clientpool.NewClientsPool(workers, 1)

// 	users, err := getSotr("../../.tmp/sotr.json")
// 	require.NoError(t, err)
// 	require.Less(t, 0, len(users))

// 	count := 1
// 	for name, u := range users {
// 		// if count > 10 {
// 		// 	break
// 		// }
// 		cli := clientsPool.Get()
// 		cli.SetBaseURL(MainURL).
// 			SetTimeout(5 * time.Second)

// 		wg.Add(1)

// 		go func() {
// 			defer clientsPool.Push(cli)
// 			defer wg.Done()
// 			resp, err := cli.R().
// 				SetErrorResult(&errMsg). // Unmarshal response body into errMsg automatically if status code >= 400.
// 				Get("/modules/employee_card/functions.php?task=showMobile&s_tab_num=" + u.Tabnum)

// 			if err != nil { // Error handling.
// 				fmt.Println("Worker:", "error handling", err)
// 			}

// 			if resp.IsErrorState() { // Status code >= 400.
// 				fmt.Println("Worker:", "err", errMsg.Message) // Record error message returned.
// 			}

// 			if resp.IsSuccessState() { // Status code is between 200 and 299.
// 				text := resp.String()
// 				mob, err := ParseMobile(text)
// 				if err != nil {
// 					fmt.Printf("error parsing: %v, name: %s, text %s\n", err, name, text) // Record error message returned.
// 				}
// 				u.Mobile = mob.Data

// 				err = saveUser(wrSotr, u, &mt)

// 				// str, err := json.Marshal(u)
// 				if err != nil {
// 					fmt.Printf("error marshal: %v, user %v\n", err, u) // Record error message returned.
// 				}
// 				// fmt.Sprintf("%s\n", str)
// 			}
// 		}()

// 		count++
// 	}
// 	wg.Wait()
// }

// func saveUser(wrSotr io.Writer, u *kbv1.Sotr, mt *sync.Mutex) (err error) {
// 	b, err := json.Marshal(u)
// 	if err != nil {
// 		return
// 	}

// 	b = append(b, "\n"...)

// 	mt.Lock()
// 	defer mt.Unlock()

// 	_, err = wrSotr.Write(b)

// 	return
// }
