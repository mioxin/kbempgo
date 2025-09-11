package clientpool

import (
	"errors"

	"github.com/imroc/req/v3"
)

type ClientsPool chan *req.Client

func NewClientsPool(count int) ClientsPool {
	return make(chan *req.Client, count)
}

func (p ClientsPool) Get(debLevel int) *req.Client {
	var cli *req.Client

	select {
	case cli = <-p:
	default:
		hdrs := map[string]string{
			"Accept":             "*/*",
			"Accept-Language":    "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7",
			"Connection":         "keep-alive",
			"DNT":                "1",
			"Referer":            "https://my.kaspi.kz/?&type=1297",
			"Sec-Fetch-Dest":     "empty",
			"Sec-Fetch-Mode":     "cors",
			"Sec-Fetch-Site":     "same-origin",
			"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36",
			"sec-ch-ua-platform": "\"Windows\"'\", \"Google Chrome\";v=\"139\", \"Chromium\";v=\"139\"",
		}

		cli = req.C().
			SetCommonHeaders(hdrs)

		if debLevel > 2 {
			cli = cli.EnableDumpAll(). // Dump all requests.
							EnableDebugLog()
		}
	}
	return cli
}

func (p ClientsPool) Push(cli *req.Client) (err error) {
	select {
	case p <- cli:
	default:
		err = errors.New("failed return http Client to pool")
	}
	return err
}
