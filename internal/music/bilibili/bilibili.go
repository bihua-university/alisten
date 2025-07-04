package bilibili

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/qiniu/go-sdk/v7/storagev2/credentials"
	"github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/uploader"

	"github.com/bihua-university/alisten/internal/base"
	"github.com/bihua-university/alisten/internal/music/bihua"
)

type ctxt struct {
	bvId     string
	pic      string
	title    string
	owner    string
	duration int
}

func Upload(bvId string) {
	c := ctxt{bvId: bvId}

	// 保存路径
	savePath, err := GetSavePath()
	if err != nil {
		return
	}

	quality := c.audioQuality(bvId)
	audio, photo, _ := c.download(bvId, savePath, quality)
	defer func() {
		_ = os.Remove(photo)
		_ = os.Remove(audio)
	}()
	photo, err = c.upload(photo, "jpg")
	if err != nil {
		return
	}
	audio, err = c.upload(audio, "mp3")
	if err != nil {
		return
	}
	_ = bihua.InsertMusic(&bihua.MusicModel{
		MusicID:    bvId,
		Name:       c.title,
		Artist:     c.owner,
		AlbumName:  bvId,
		PictureURL: photo,
		WebURL:     fmt.Sprintf("https://www.bilibili.com/video/%s", bvId),
		Duration:   int64(c.duration),
		URL:        audio,
		Lyric:      "",
		PlayCount:  0,
	})
}

func (c *ctxt) upload(filename string, ext string) (string, error) {
	mac := credentials.NewCredentials(base.Config.QiniuAK, base.Config.QiniuSK)
	bucket := "bihua-oss"
	key := fmt.Sprintf("alisten/%s.%s", c.bvId, ext)
	uploadManager := uploader.NewUploadManager(&uploader.UploadManagerOptions{
		Options: http_client.Options{
			Credentials: mac,
		},
	})
	err := uploadManager.UploadFile(context.Background(), filename, &uploader.ObjectOptions{
		BucketName: bucket,
		ObjectName: &key,
		FileName:   key,
	}, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://bihua-oss.ggemo.com/alisten/%s.%s", c.bvId, ext), nil
}

// 下载媒体文件
func (c *ctxt) download(bvId, savePath string, qn int) (filename, photo string, err error) {
	var url string
	data := c.playerPlayUrl(bvId)
	for _, stream := range data.Data.Dash.Audio {
		if stream.ID == qn {
			url = stream.BaseURL
			break
		}
	}

	if url != "" {
		filename = fmt.Sprintf("%s_%v.mp3", bvId, time.Now().Unix())
		client := &http.Client{}
		request, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", "", err
		}
		setDefaultHeaders(request, bvId)
		SetCookie(request)

		rsp, err := client.Do(request)
		if err != nil {
			return "", "", err
		}
		defer rsp.Body.Close()

		path := filepath.Join(savePath, filename)
		out, err := os.Create(path)
		if err != nil {
			return "", "", err
		}
		defer out.Close()

		dr := &Downloader{
			rsp.Body,
			rsp.ContentLength,
			0,
		}
		io.Copy(out, dr)
	}

	if c.pic != "" {
		photo = fmt.Sprintf("%s_%v.jpg", bvId, time.Now().Unix())
		client := &http.Client{}
		request, err := http.NewRequest("GET", c.pic, nil)
		if err != nil {
			return "", "", err
		}
		setDefaultHeaders(request, bvId)
		SetCookie(request)

		rsp, err := client.Do(request)
		if err != nil {
			return "", "", err
		}
		defer rsp.Body.Close()

		path := filepath.Join(savePath, photo)
		out, err := os.Create(path)
		if err != nil {
			return "", "", err
		}
		defer out.Close()

		dr := &Downloader{
			rsp.Body,
			rsp.ContentLength,
			0,
		}
		io.Copy(out, dr)
	}
	return filename, photo, nil
}

func (c *ctxt) audioQuality(bvid string) int {
	data := c.playerPlayUrl(bvid)
	var quality int
	for _, audio := range data.Data.Dash.Audio {
		if audio.ID > quality {
			quality = audio.ID
		}
	}
	return quality
}

func (c *ctxt) playerPlayUrl(bvid string) (videoRespData *Response[VideoPlayRespData]) {
	webInterfaceViewRespData := webInterfaceView(bvid)
	params := make(map[string]interface{})
	params["fnval"] = 4048
	params["avid"] = webInterfaceViewRespData.Data.Aid
	params["cid"] = webInterfaceViewRespData.Data.Cid
	c.pic = webInterfaceViewRespData.Data.Pic
	c.title = webInterfaceViewRespData.Data.Title
	c.owner = webInterfaceViewRespData.Data.Owner.Name
	c.duration = webInterfaceViewRespData.Data.Duration * 1000
	videoRespData = ReqGet[VideoPlayRespData](PlayerPlayUrl, params)
	return
}

// 获取视频页面信息
func webInterfaceView(bvid string) (webInterfaceViewRespData *Response[WebInterfaceViewRespData]) {
	params := make(map[string]interface{})
	params["bvid"] = bvid
	webInterfaceViewRespData = ReqGet[WebInterfaceViewRespData](WebInterfaceView, params)
	return
}

func ReqGet[T WebInterfaceViewRespData | VideoPlayRespData](reqUrl string, params map[string]interface{}) (videoRespData *Response[T]) {
	client := req.C().
		SetTimeout(5 * time.Second)

	var errMsg Resp
	resp, err := client.R().
		SetQueryParamsAnyType(params).
		SetSuccessResult(&videoRespData). // Unmarshal response body into userInfo automatically if status code is between 200 and 299.
		SetErrorResult(&errMsg).          // Unmarshal response body into errMsg automatically if status code >= 400.
		EnableDump().                     // Enable dump at request level, only print dump content if there is an error or some unknown situation occurs to help troubleshoot.
		Get(reqUrl)
	if err != nil { // Error handling.
		log.Println("error:", err)
		log.Println("raw content:")
		log.Println(resp.Dump()) // Record raw content when error occurs.
		return
	}
	if resp.IsErrorState() { // Status code >= 400.
		fmt.Println(errMsg.Message) // Record error message returned.
		return
	}
	if resp.IsSuccessState() { // Status code is between 200 and 299.
		return
	}
	return
}

func setDefaultHeaders(req *http.Request, bvId string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Range", "bytes=0-")
	req.Header.Set("Referer", "https://www.bilibili.com/video/"+bvId)
	req.Header.Set("Origin", "https://www.bilibili.com")
	req.Header.Set("Connection", "keep-alive")
}

func SetCookie(req *http.Request) {
	cookie := http.Cookie{Name: "SESSDATA", Value: SessionData, Expires: time.Now().Add(30 * 24 * 60 * 60 * time.Second)}
	req.AddCookie(&cookie)
}

func GetSavePath() (savePath string, err error) {
	savePath, err = os.Getwd()
	savePath = strings.TrimSpace(savePath)
	return
}
