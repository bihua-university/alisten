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
	log.Printf("尝试读取配置文件: %s", configPath)

	file, err := os.Open(configPath)
	if err != nil {
		log.Printf("无法打开配置文件 %s: %v", configPath, err)
		return nil, fmt.Errorf("无法打开配置文件: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Printf("解析配置文件失败: %v", err)
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	log.Printf("配置文件解析成功: ServerURL=%s, Token长度=%d",
		config.ServerURL, len(config.Token))
	return &config, nil
}

func main() {
	log.Println("=== Musiclet 启动 ===")

	// 读取配置文件
	log.Println("正在读取配置文件...")
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("配置文件读取失败: %v", err)
	}
	log.Printf("配置文件读取成功，服务器地址: %s", config.ServerURL)

	// init bilibili config
	bilibili.Config.QiniuAK = config.QiniuAK
	bilibili.Config.QiniuSK = config.QiniuSK
	log.Println("Bilibili 配置初始化完成")

	client := task.NewClient(config.ServerURL, config.Token)
	log.Println("任务客户端创建完成")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("开始任务循环...")
	taskCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Println("收到退出信号，正在关闭...")
			return
		default:
			log.Printf("正在获取任务... (已处理任务数: %d)", taskCount)
			t, err := client.GetTask(ctx)
			if err != nil {
				log.Printf("获取任务失败: %v", err)
				continue
			}

			if t == nil {
				log.Println("暂无任务，继续等待...")
				continue
			}

			log.Printf("收到任务: ID=%s, Type=%s", t.ID, t.Type)
			result := processTask(t)

			log.Printf("任务处理完成: ID=%s, Success=%t", result.ID, result.Success)
			err = client.SubmitResult(result)
			if err != nil {
				log.Printf("提交结果失败: %v", err)
			} else {
				log.Printf("任务结果提交成功: ID=%s", result.ID)
				taskCount++
			}
		}
	}
}

func processTask(t *task.Task) *task.Result {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("任务处理异常: %v", r)
		}
	}()

	log.Printf("开始处理任务: ID=%s, Type=%s", t.ID, t.Type)

	result := &task.Result{
		ID: t.ID,
	}

	switch t.Type {
	case "bilibili_upload":
		log.Printf("处理 Bilibili 上传任务: %s", t.ID)
		bvId, ok := t.Data["bvid"]
		if !ok {
			log.Printf("任务 %s 缺少 bvid 参数", t.ID)
			result.Success = false
			result.Error = "缺少bvid参数"
		} else {
			log.Printf("开始处理 Bilibili 视频: %s", bvId)
			res, err := bilibili.ProcessUpload(bvId)
			if err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("bilibili上传失败: %v", err)
				log.Printf("Bilibili 上传失败 (任务 %s, BV %s): %v", t.ID, bvId, err)
			} else {
				result.Result = res
				result.Success = true
				log.Printf("Bilibili 上传成功 (任务 %s, BV %s)", t.ID, bvId)
			}
		}

	default:
		result.Success = false
		result.Error = fmt.Sprintf("未知的任务类型: %s", t.Type)
		log.Printf("收到未知任务类型 (任务 %s): %s", t.ID, t.Type)
	}

	log.Printf("任务处理完成: ID=%s, Success=%t", result.ID, result.Success)
	return result
}
