package clientpool

import (
	"errors"
	"fmt"
	"html"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/imroc/req/v3"
)

// ClientsPool is Http clients pool based on chanal. Clients will created up to max value seted in constructor
type ClientsPool chan *req.Client

// NewClientsPool create Http clients pool based on chanal. Count is a max number of clients in the pool
func NewClientsPool(count, debLevel int) ClientsPool {
	ch := make(chan *req.Client, count)
	for range count {
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

		if debLevel > 2 {
			cli = cli.EnableDumpAll(). // Dump all requests.
							EnableDebugLog()
		}

		ch <- cli
	}

	return ch
}

// Get client from pool
func (p ClientsPool) Get() *req.Client {
	return <-p
}

// Push return client to pool after use one
func (p ClientsPool) Push(cli *req.Client) (err error) {
	select {
	case p <- cli:
	default:
		cli.CloseIdleConnections()

		err = errors.New("failed return http Client to pool")
	}

	return err
}

// ClientPool is Http clients pool based on a sync.Pool
type ClientPool struct {
	pool sync.Pool
}

// NewClientPool
func NewClientPool(debLevel int) *ClientPool {
	return &ClientPool{
		pool: sync.Pool{
			New: func() interface{} {
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
						fmt.Printf("Retry request %d: %s %s\nError %v\n", resp.Request.RetryAttempt, req.Method, req.URL, html.UnescapeString(err.Error()))
					}).
					AddCommonRetryCondition(func(resp *req.Response, err error) bool {
						return err != nil || resp.StatusCode >= 500
					})

				if debLevel > 2 {
					cli = cli.EnableDumpAll(). // Dump all requests.
									EnableDebugLog()
				}

				return cli
			},
		},
	}
}

// get
func (p *ClientPool) Get() *req.Client {
	return p.pool.Get().(*req.Client)
}

// Push
func (p *ClientPool) Push(cli *req.Client) {
	p.pool.Put(cli)
}
