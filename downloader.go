package downloader

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	// defaultConcurrent is default concurrent num.
	defaultConcurrent = 1

	// maxGCroutineNum is max gcroutine num.
	maxGCroutineNum = 100
)

// Downloader is http ranges downloader.
type Downloader struct {
	// url is target file source url which should support ranges bytes mode.
	url string

	// headers is http headers setups from users.
	headers map[string]string

	// file is final file FD for target source.
	file *os.File

	// fileSize is size of target file source.
	fileSize int64

	// newFile is new file path-name for target source.
	newFile string

	// concurrent is download range handler concurrent num.
	concurrent int

	// limiter is download rate limiter, not limit if it's nil.
	limiter Limiter

	// isCanceled is download cancel flag.
	isCanceled bool

	// finalErr is download action final error.
	finalErr error

	// syncWG makes it keep waitting until all download ranges done.
	syncWG sync.WaitGroup
}

// NewDownloader creates a new Downloader object.
func NewDownloader(url string, concurrent int, headers map[string]string, newFile string) *Downloader {
	return &Downloader{
		url:        url,
		headers:    headers,
		concurrent: concurrent,
		newFile:    newFile,
	}
}

// SetRateLimiterOption setups limiter option.
func (d *Downloader) SetRateLimiterOption(limiter Limiter) {
	d.limiter = limiter
}

// setupRateLimiter setups limiter num base on gcroutines.
func (d *Downloader) setupRateLimiter() {
	if d.limiter == nil {
		return
	}
	totalLimitNum := d.limiter.LimitNum()
	d.limiter.Reset(totalLimitNum / int64(d.concurrent))
}

// Download starts and downloads target source in ranges mode.
func (d *Downloader) Download(timeout time.Duration) error {
	if len(d.url) == 0 {
		return errors.New("empty url")
	}

	if d.concurrent <= 0 || d.concurrent > maxGCroutineNum {
		// reset to default concurrent.
		d.concurrent = defaultConcurrent
	}

	// check target source support range bytes mode or not.
	size, err := d.checkRangeSupport()
	if err != nil {
		return err
	}
	// target source file total size.
	d.fileSize = size

	// re-create new file for target source.
	file, err := os.OpenFile(d.newFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	d.file = file

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// starts download now.
	d.download(ctx)

	return d.finalErr
}

// Clean cleans downloaded or temp files.
func (d *Downloader) Clean() error {
	mvFile := base64.StdEncoding.EncodeToString([]byte(d.newFile))
	return os.Rename(d.newFile, fmt.Sprintf("/tmp/%s.%d", mvFile, time.Now().UnixNano()))
}

// download processes http range bytes download action.
func (d *Downloader) download(ctx context.Context) {
	if d.fileSize < int64(d.concurrent) {
		// reset to default concurrent.
		d.concurrent = defaultConcurrent
	}

	go func() {
		<-ctx.Done()
		d.isCanceled = true
	}()

	// partial size for every download gcroutine.
	partialSize := d.fileSize / int64(d.concurrent)

	// setup final limit for each gcroutine.
	d.setupRateLimiter()

	// split for every gcroutine.
	var start, end int64

	for n := 0; n < d.concurrent; n++ {
		if n == d.concurrent-1 {
			// last part gcroutine handles the left datas.
			end = d.fileSize
		} else {
			end = start + partialSize
		}

		// add one gcroutine.
		d.syncWG.Add(1)

		go func(partN int, start int64, end int64) {
			if err := d.downloadRange(partN, start, end); err != nil {
				d.finalErr = err
			}
		}(n, start, end-1)

		// reset new start point for next gcroutine.
		start = end
	}

	// waitting for all download gcroutine.
	d.syncWG.Wait()
}

// downloadRange downloads target range datas base on start and end point.
func (d *Downloader) downloadRange(partN int, start int64, end int64) error {
	// total written bytes num.
	var written int64

	// http range data, the size is range data size, and the body is http response
	// body which need to be close by caller.
	body, size, err := d.rangeData(start, end)
	if err != nil {
		return err
	}

	// download gcroutine done.
	defer d.syncWG.Done()
	defer body.Close()

	// make buffer to read and write file data.
	buf := make([]byte, 4*1024)

	// keep range and read/write datas.
	for {
		if d.isCanceled {
			return errors.New("download timeout")
		}

		// count written bytes num.
		if d.limiter != nil {
			d.limiter.Wait(written)
		}

		// read file range datas.
		rn, err := body.Read(buf)

		// write data.
		if rn > 0 {
			wn, err := d.file.WriteAt(buf[0:rn], start)
			if err != nil {
				return err
			}

			// check read/write datas num.
			if rn != wn {
				return errors.New("read/write data errors")
			}
			start += int64(wn)

			// count total written num.
			if wn > 0 {
				written += int64(wn)
			}
		}

		if err == nil {
			// process success in this round, try next.
			continue
		}

		if err.Error() != "EOF" {
			return fmt.Errorf("part[%d] download failed, %+v", partN, err)
		}

		// check final range datas download result.
		if size != written {
			return fmt.Errorf("part[%d] not success", partN)
		}

		// all range datas processed success.
		return nil
	}
}

// rangeData returns target range data http body and content size.
func (d *Downloader) rangeData(start int64, end int64) (io.ReadCloser, int64, error) {
	client := &http.Client{}

	request, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range d.headers {
		request.Header.Set(k, v)
	}

	// set range.
	request.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	response, err := client.Do(request)
	if err != nil {
		return nil, 0, err
	}

	if response.StatusCode != http.StatusPartialContent {
		return nil, 0, fmt.Errorf("response status[%+v]", response.StatusCode)
	}
	respHeader := response.Header

	if len(respHeader["Content-Length"]) == 0 {
		return nil, 0, errors.New("unknown file range size")
	}

	// get range content length.
	size, err := strconv.ParseInt(respHeader["Content-Length"][0], 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("can't parse file range size, %+v", err)
	}

	return response.Body, size, nil
}

// checkRangeSupport checks target source support range download or not.
func (d *Downloader) checkRangeSupport() (int64, error) {
	client := &http.Client{}

	request, err := http.NewRequest("GET", d.url, nil)
	if err != nil {
		return 0, err
	}

	for k, v := range d.headers {
		request.Header.Set(k, v)
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("response status[%+v]", response.StatusCode)
	}
	respHeader := response.Header

	acceptRanges, isSupported := respHeader["Accept-Ranges"]
	if !isSupported {
		return 0, errors.New("not support ranges download mode")
	}

	if len(acceptRanges) == 0 || acceptRanges[0] != "bytes" {
		return 0, errors.New("support ranges download, but not bytes mode")
	}

	if len(respHeader["Content-Length"]) == 0 {
		return 0, errors.New("unknown file ranges content total size")
	}

	// get target source file content length.
	size, err := strconv.ParseInt(respHeader["Content-Length"][0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("can't parse file ranges content total size, %+v", err)
	}

	return size, nil
}
