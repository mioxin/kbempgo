package httpclient

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/imroc/req/v3"
)

const MaxIdleConnsPerHost int = 20

// NewClientsPool create Http clients pool based on chanal. Count is a max number of clients in the pool
func NewHTTPClient(debLevel int) *req.Client {
	hdrs := map[string]string{
		"Accept":                    "*/*",
		"Accept-Language":           "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
		"Cache-Control":             "max-age=0",
		"Connection":                "keep-alive",
		"Cookie":                    "PHPSESSID=88ae87fcac04e76c600c14d250e041a4; ssaid=bcf81880-7c02-11f0-91d1-fde5c8f9f783; test.user.group=26; redirected=true; test.user.group_exp=76; test.user.group_exp2=13; __tld__=null; NSC_nz.lbtqj.la-443=ffffffff091900d245525d5f4f58455e445a4a423660",
		"DNT":                       "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
		"sec-ch-ua":                 "\"Chromium\";v=\"140\", \"Not=A?Brand\";v=\"24\", \"Google Chrome\";v=\"140\"",
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        "\"Windows\"",
	}

	cli := req.C().
		SetCommonHeaders(hdrs).
		SetCommonRetryCount(3).
		SetCommonRetryBackoffInterval(1*time.Second, 5*time.Second).
		SetCommonRetryInterval(func(resp *req.Response, attempt int) time.Duration {
			// Sleep seconds from "Retry-After" response header if it is present and correct.
			// https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html
			if resp.Response != nil {
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					after, err := strconv.Atoi(ra)
					if err == nil {
						return time.Duration(after) * time.Second
					}
				}
			}

			sleep := 0.01 * math.Exp2(float64(attempt))

			return time.Duration(math.Min(2, sleep)) * time.Second
		}).
		AddCommonRetryHook(func(resp *req.Response, err error) {
			req := resp.Request.RawRequest
			if err != nil {
				fmt.Println("Retry request:", req.Method, req.URL, "; err: ", err)
			} else {
				fmt.Println("Retry request:", req.Method, req.URL, "; time: ", resp.TotalTime())
			}
		}).
		AddCommonRetryCondition(func(resp *req.Response, err error) bool {
			return err != nil || resp.StatusCode >= 500
		})

	cli.Transport.MaxIdleConnsPerHost = MaxIdleConnsPerHost
	cli.Transport.IdleConnTimeout = 90 * time.Second

	if debLevel > 1 {
		cli = cli.EnableDebugLog()
	}

	if debLevel > 2 {
		cli = cli.EnableDumpAll() // Dump all requests.
	}

	return cli
}
