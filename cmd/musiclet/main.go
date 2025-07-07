package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/bihua-university/alisten/cmd/musiclet/bilibili"
	"github.com/bihua-university/alisten/internal/task"
)

type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	QiniuAK   string `json:"qiniu_ak"`
	QiniuSK   string `json:"qiniu_sk"`
}

// loadConfig 读取配置文件
func loadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开配置文件: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return &config, nil
}

func main() {
	// 读取配置文件
	config, err := loadConfig("config.json")
	if err != nil {
		panic(err)
	}
	// init bilibili config
	bilibili.Config.QiniuAK = config.QiniuAK
	bilibili.Config.QiniuSK = config.QiniuSK

	client := task.NewClient(config.ServerURL, config.Token)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			t, err := client.GetTask(ctx)
			if err != nil {
				continue
			}

			if t == nil {
				continue
			}

			result := processTask(t)

			err = client.SubmitResult(result)
			if err != nil {
				log.Printf("提交结果失败: %v", err)
			}
		}
	}
}

func processTask(t *task.Task) *task.Result {
	result := &task.Result{
		ID: t.ID,
	}

	switch t.Type {
	case "bilibili_upload":
		bvId, ok := t.Data["bvid"]
		if !ok {
			result.Success = false
			result.Error = "缺少bvid参数"
		} else {
			res, err := bilibili.ProcessUpload(bvId)
			if err != nil {
				result.Success = false

				result.Error = fmt.Sprintf("bilibili上传失败: %v", err)
				log.Printf("bilibili上传失败: %v", err)
			} else {
				result.Result = res
				result.Success = true
			}
		}

	default:
		result.Success = false
		result.Error = fmt.Sprintf("未知的任务类型: %s", t.Type)
		log.Printf("未知任务类型: %s", t.Type)
	}

	return result
}
