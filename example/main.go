package main

import (
	"fmt"
	"time"

	dl "downloader"
)

var (
	target             = "http://source.com/api/generic/my-project/my-repo/path/source.tgz"
	newFile            = "./local-source.tgz"
	headers            = map[string]string{}
	concurrent         = 5
	limitBytesInSecond = 1024 * 1024 * 5 // 5MB
)

func main() {
	downloader := dl.NewDownloader(target, concurrent, headers, newFile)
	downloader.SetRateLimiterOption(&dl.SimpleRateLimiter{LimitNum: limitBytesInSecond})

	timenow := time.Now()
	if err := downloader.Download(); err != nil {
		downloader.Clean()
		panic(err)
	}
	fmt.Println("download success, cost: ", time.Since(timenow))
}
