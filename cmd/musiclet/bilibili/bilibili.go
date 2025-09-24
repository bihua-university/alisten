package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bihua-university/alisten/cmd/musiclet/bihua"
	"github.com/qiniu/go-sdk/v7/storagev2/credentials"
	"github.com/qiniu/go-sdk/v7/storagev2/http_client"
	"github.com/qiniu/go-sdk/v7/storagev2/uploader"

	"github.com/aws/aws-sdk-go-v2/config"
	awsCreds "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var QiniuConfig struct {
	Ak     string
	Sk     string
	Bucket string
	Domain string
}

var S3Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	Bucket          string
	EndpointURL     string
}

type ctxt struct {
	bvId     string
	pic      string
	title    string
	owner    string
	duration int
}

func InitQiniuConfig(ak, sk, bucket, domain string) {
	QiniuConfig.Ak = ak
	QiniuConfig.Sk = sk
	QiniuConfig.Bucket = bucket
	QiniuConfig.Domain = domain
}

func InitS3Config(accessKeyID, secretAccessKey, region, bucket, endpointURL string) {
	S3Config.AccessKeyID = accessKeyID
	S3Config.SecretAccessKey = secretAccessKey
	S3Config.Region = region
	S3Config.Bucket = bucket
	S3Config.EndpointURL = endpointURL
}

// ProcessUpload 处理B站视频上传任务
func ProcessUpload(bvId string) (*bihua.MusicModel, error) {
	c := ctxt{bvId: bvId}

	// 保存路径
	savePath, err := GetSavePath()
	if err != nil {
		return nil, fmt.Errorf("获取保存路径失败: %v", err)
	}

	quality := c.audioQuality(bvId)
	audio, photo, err := c.download(bvId, savePath, quality)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %v", err)
	}
	defer func() {
		_ = os.Remove(photo)
		_ = os.Remove(audio)
	}()

	photo, err = c.upload(photo, "jpg")
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %v", err)
	}

	audio, err = c.upload(audio, "mp3")
	if err != nil {
		return nil, fmt.Errorf("上传音频失败: %v", err)
	}

	res := &bihua.MusicModel{
		MusicID:    bvId,
		Name:       c.title,
		Artist:     c.owner,
		AlbumName:  bvId,
		PictureURL: photo,
		Duration:   int64(c.duration),
		URL:        audio,
	}
	return res, nil
}

func (c *ctxt) upload(filename string, ext string) (string, error) {
	if QiniuConfig.Ak != "" && QiniuConfig.Sk != "" && QiniuConfig.Bucket != "" {
		return c.qiniuUpload(filename, ext)
	} else if S3Config.AccessKeyID != "" && S3Config.SecretAccessKey != "" && S3Config.Bucket != "" {
		return c.s3Upload(filename, ext)
	}
	return "", fmt.Errorf("未配置上传参数")
}

func (c *ctxt) qiniuUpload(filename string, ext string) (string, error) {
	mac := credentials.NewCredentials(QiniuConfig.Ak, QiniuConfig.Sk)
	key := fmt.Sprintf("alisten/%s.%s", c.bvId, ext)
	uploadManager := uploader.NewUploadManager(&uploader.UploadManagerOptions{
		Options: http_client.Options{
			Credentials: mac,
		},
	})
	err := uploadManager.UploadFile(context.Background(), filename, &uploader.ObjectOptions{
		BucketName: QiniuConfig.Bucket,
		ObjectName: &key,
		FileName:   key,
	}, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/alisten/%s.%s", QiniuConfig.Domain, c.bvId, ext), nil
}

func (c *ctxt) s3Upload(filename string, ext string) (string, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(S3Config.Region),
		config.WithCredentialsProvider(awsCreds.NewStaticCredentialsProvider(S3Config.AccessKeyID, S3Config.SecretAccessKey, "")),
	)
	if err != nil {
		return "", err
	}

	// 兼容 minio
	var client *s3.Client
	if S3Config.EndpointURL != "" {
		client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = &S3Config.EndpointURL
			o.UsePathStyle = true
		})
	} else {
		client = s3.NewFromConfig(awsCfg)
	}
	key := fmt.Sprintf("alisten/%s.%s", c.bvId, ext)
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &S3Config.Bucket,
		Key:    &key,
		Body:   file,
	})
	if err != nil {
		return "", err
	}
	var url string
	if S3Config.EndpointURL != "" {
		url = fmt.Sprintf("%s/%s/%s", S3Config.EndpointURL, S3Config.Bucket, key)
	} else {
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", S3Config.Bucket, S3Config.Region, key)
	}
	return url, nil
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
	// 构建URL查询参数
	u, err := url.Parse(reqUrl)
	if err != nil {
		log.Printf("解析URL失败: %v", err)
		return nil
	}

	query := u.Query()
	for key, value := range params {
		query.Set(key, fmt.Sprintf("%v", value))
	}
	u.RawQuery = query.Encode()

	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// 创建HTTP请求
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		log.Printf("创建请求失败: %v", err)
		return nil
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应失败: %v", err)
		return nil
	}

	// 检查HTTP状态码
	if resp.StatusCode >= 400 {
		var errMsg Resp
		if err := json.Unmarshal(body, &errMsg); err == nil {
			log.Printf("API错误: %s", errMsg.Message)
		} else {
			log.Printf("HTTP错误: %d", resp.StatusCode)
		}
		return nil
	}

	// 解析JSON响应
	videoRespData = new(Response[T])
	if err := json.Unmarshal(body, videoRespData); err != nil {
		log.Printf("解析JSON失败: %v", err)
		log.Printf("响应内容: %s", string(body))
		return nil
	}

	return videoRespData
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
