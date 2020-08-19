package main

import (
	"fmt"
	"time"

	dl "../../downloader"
)

var (
	target             = "http://source.com/api/generic/my-project/my-repo/path/source.tgz"
	newFile            = "./local-source.tgz"
	headers            = map[string]string{}
	timeout            = 30 * time.Second
	concurrent         = 5
	limitBytesInSecond = int64(1024 * 1024 * 10) // 10MB
)

func main() {
	downloader := dl.NewDownloader(target, concurrent, headers, newFile)
	downloader.SetRateLimiterOption(dl.NewSimpleRateLimiter(limitBytesInSecond))

	timenow := time.Now()
	if err := downloader.Download(timeout); err != nil {
		downloader.Clean()
		panic(err)
	}
	fmt.Println("download success, cost: ", time.Since(timenow))
}
