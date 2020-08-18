package main

import (
	"fmt"
	"time"

	"bk-bscp/pkg/downloader"
)

var (
	target     = "http://source.com/api/generic/my-project/my-repo/path/source.tgz"
	newFile    = "./local-source.tgz"
	headers    = map[string]string{}
	concurrent = 5
)

func main() {
	downloader := downloader.NewDownloader(target, concurrent, headers, newFile)

	timenow := time.Now()
	if err := downloader.Download(); err != nil {
		downloader.Clean()

		panic(err)
	}
	fmt.Println("download success, cost: ", time.Since(timenow))
}
