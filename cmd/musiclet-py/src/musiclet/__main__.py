import asyncio
import logging
import os
import signal
import sys

# 添加当前目录到Python路径，以便导入本地模块
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from .client import TaskClient
from .config import load_config
from .database import convert_to_map, get_music_by_id, init_db, insert_music, search_music_by_db
from .downloader import AudioDownloader, create_uploader
from .task import Result, Task


class MusicletProcessor:
    """Musiclet处理器"""

    def __init__(self):
        self.config = None
        self.client = None
        self.downloader = None
        self.uploader = None
        self.task_count = 0
        self.running = True

    async def initialize(self):
        """初始化组件"""
        logging.info("=== Musiclet Python版 启动 ===")

        # 读取配置文件
        logging.info("正在读取配置文件...")
        self.config = load_config()
        if not self.config:
            logging.error("配置文件读取失败")
            return False

        logging.info(f"配置文件读取成功，服务器地址: {self.config.server_url}")

        # 初始化存储上传器
        self.uploader = create_uploader(self.config.storage)
        if not self.uploader:
            logging.error("存储上传器初始化失败")
            return False

        logging.info("S3存储配置初始化完成")

        # 初始化数据库
        try:
            init_db(self.config.pgsql)
            logging.info("数据库初始化完成")
        except Exception as e:
            logging.error(f"数据库初始化失败: {e}")
            return False

        # 初始化任务客户端
        self.client = TaskClient(self.config.server_url, self.config.token)
        logging.info("任务客户端创建完成")

        # 初始化音频下载器
        self.downloader = AudioDownloader()
        logging.info("音频下载器初始化完成")

        return True

    async def process_task(self, task: Task) -> Result:
        """处理任务"""
        logging.info(f"开始处理任务: ID={task.id}, Type={task.type}")

        result = Result.new_result(task.id, False)

        try:
            if task.type == "url_common:get_music":
                await self._process_url_music_task(task, task.data["id"], result)
            elif task.type == "bilibili:get_music":
                task.data["url"] = "https://www.bilibili.com/video/" + task.data["bvid"]
                await self._process_url_music_task(task, task.data["bvid"], result)
            elif task.type == "bilibili:search_music":
                await self._process_search_music_task(task, result)
            else:
                result.error = f"未知的任务类型: {task.type}"
                logging.warning(f"收到未知任务类型 (任务 {task.id}): {task.type}")

        except Exception as e:
            logging.error(f"处理任务时发生异常: {e}")
            result.error = f"处理任务异常: {e!s}"

        logging.info(f"任务处理完成: ID={result.id}, Success={result.success}")
        return result

    async def _process_url_music_task(self, task: Task, music_id: str, result: Result):
        """处理URL音乐任务"""
        url = task.data.get("url")
        if not url:
            result.error = "缺少 url 参数"
            logging.error(f"任务 {task.id} 缺少 url 参数")
            return

        logging.info(f"开始处理URL音频: {url}")

        # 下载音频（同时下载缩略图并上传）
        music_info = self.downloader.download_audio(url, self.uploader)
        if not music_info:
            result.error = "音频下载失败"
            logging.error(f"音频下载失败 (任务 {task.id}, URL {url})")
            return

        # 上传到存储服务
        audio_filename = f"{music_info['id']}.mp3"
        logging.info(f"开始上传 {audio_filename}")
        uploaded_url = self.uploader.upload_audio(music_info["local_path"], audio_filename)

        if not uploaded_url:
            result.error = "音频上传失败"
            logging.error(f"音频上传失败 (任务 {task.id}, URL {url})")
            # 清理本地文件
            self.downloader.cleanup_file(music_info["local_path"])
            return

        # 更新音频URL
        music_info["url"] = uploaded_url

        # 保存到数据库
        if not insert_music(music_id, music_info):
            result.error = "数据库保存失败"
            logging.error(f"数据库保存失败 (任务 {task.id}, URL {url})")
            # 清理本地文件
            self.downloader.cleanup_file(music_info["local_path"])
            return

        # 清理本地文件
        self.downloader.cleanup_file(music_info["local_path"])

        # 从数据库获取完整信息
        saved_music = get_music_by_id(music_id)
        if saved_music:
            result.success = True
            result.result = convert_to_map(saved_music)
            logging.info(f"URL音频处理成功: {music_info['name']}")
        else:
            result.error = "无法从数据库获取保存的音乐信息"

    async def _process_search_music_task(self, task: Task, result: Result):
        """处理搜索音乐任务"""
        keyword = task.data.get("keyword")
        if not keyword:
            result.error = "缺少 keyword 参数"
            logging.error(f"任务 {task.id} 缺少 keyword 参数")
            return

        # 获取分页参数，提供默认值
        try:
            page = int(task.data.get("page", "1"))
            page_size = int(task.data.get("pageSize", "20"))
        except (ValueError, TypeError) as e:
            result.error = f"分页参数格式错误: {e}"
            logging.error(f"任务 {task.id} 分页参数格式错误: {e}")
            return

        logging.info(f"开始搜索音乐: keyword={keyword}, page={page}, pageSize={page_size}")

        try:
            # 调用数据库搜索函数
            music_list, total = search_music_by_db(keyword, page, page_size)

            # 转换为API格式，类似于Go版本中的ConvertMusicList
            data_list = []
            for music in music_list:
                # 构建Music结构体格式的音乐信息
                music_data = {
                    "id": music.music_id,
                    "name": music.name,
                    "artist": music.artist,
                    "album": music.album_name,
                    "duration": music.duration,
                    "cover": music.picture_url,
                    "source": "db"  # 数据库中的音乐标记为db源
                }
                data_list.append(music_data)

            # 构建响应结果
            search_result = {
                "data": data_list,
                "total": total
            }

            result.success = True
            result.result = search_result
            logging.info(f"音乐搜索成功: 关键词={keyword}, 找到{total}条记录, 返回{len(data_list)}条")

        except Exception as e:
            result.error = f"搜索音乐失败: {e}"
            logging.error(f"搜索音乐失败 (任务 {task.id}, 关键词 {keyword}): {e}")

    async def run(self):
        """运行主循环"""
        if not await self.initialize():
            logging.error("初始化失败，程序退出")
            return

        logging.info("开始任务循环...")

        try:
            while self.running:
                try:
                    logging.info(f"正在获取任务... (已处理任务数: {self.task_count})")
                    task = await self.client.get_task()

                    if task is None:
                        logging.debug("暂无任务，继续等待...")
                        await asyncio.sleep(1)  # 短暂等待
                        continue

                    logging.info(f"收到任务: ID={task.id}, Type={task.type}")
                    result = await self.process_task(task)

                    logging.info(f"任务处理完成: ID={result.id}, Success={result.success}")
                    await self.client.submit_result(result)

                    logging.info(f"任务结果提交成功: ID={result.id}")
                    self.task_count += 1

                except Exception as e:
                    logging.error(f"任务循环中发生错误: {e}")
                    await asyncio.sleep(5)  # 错误后等待一段时间

        except KeyboardInterrupt:
            logging.info("收到中断信号，正在关闭...")
        except Exception as e:
            logging.error(f"主循环异常: {e}")
        finally:
            await self.cleanup()

    async def cleanup(self):
        """清理资源"""
        logging.info("正在清理资源...")
        if self.client:
            await self.client.close()
        logging.info("资源清理完成")

    def stop(self):
        """停止运行"""
        self.running = False


def setup_logging():
    """设置日志"""
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s - %(levelname)s - %(message)s",
        handlers=[logging.StreamHandler(sys.stdout), logging.FileHandler("musiclet.log", encoding="utf-8")],
    )


def main():
    """主函数"""
    setup_logging()

    processor = MusicletProcessor()

    # 设置信号处理
    def signal_handler(signum, frame):
        logging.info(f"收到信号 {signum}，准备退出...")
        processor.stop()

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    asyncio.run(processor.run())


if __name__ == "__main__":
    main()
