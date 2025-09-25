import hashlib
import logging
import os
from typing import Any

import yt_dlp


class AudioDownloader:
    """使用yt-dlp下载音频的类"""

    def __init__(self, download_dir: str | None = None):
        self.download_dir = download_dir or "./downloads"
        # 确保下载目录存在
        os.makedirs(self.download_dir, exist_ok=True)
        self.ytdl_opts = {
            "format": "bestaudio/best",
            "extractaudio": True,
            "audioformat": "mp3",
            "outtmpl": os.path.join(self.download_dir, "%(id)s.%(ext)s"),
            "noplaylist": True,
            "quiet": True,
            "no_warnings": True,
            "writethumbnail": True,  # 下载缩略图
            "postprocessors": [
                {
                    "key": "FFmpegExtractAudio",
                    "preferredcodec": "mp3",
                    "preferredquality": "0",
                }
            ],
        }

    def _generate_id_from_url(self, url: str) -> str:
        """从URL生成唯一ID"""
        return hashlib.md5(url.encode()).hexdigest()

    def download_audio(self, url: str, uploader: "StorageUploader | None" = None) -> dict[str, Any] | None:
        """
        下载音频并返回音频信息

        Args:
            url: 要下载的URL
            uploader: 存储上传器，用于上传缩略图

        Returns:
            包含音频信息的字典，如果失败返回None
        """
        try:
            with yt_dlp.YoutubeDL(self.ytdl_opts) as ytdl:
                # 首先获取视频信息
                info = ytdl.extract_info(url, download=False)

                if not info:
                    logging.error(f"无法获取URL信息: {url}")
                    return None

                # 生成音乐ID
                music_id = info.get("id") or self._generate_id_from_url(url)

                # 下载音频
                logging.info(f"开始下载音频: {info.get('title', 'Unknown')}")
                _ = ytdl.download([url])

                # 构建音频文件路径
                audio_filename = f"{music_id}.mp3"
                audio_path = os.path.join(self.download_dir, audio_filename)

                # 检查下载的文件是否存在
                if not os.path.exists(audio_path):
                    # 尝试查找其他可能的文件扩展名
                    for ext in ["mp3", "m4a", "webm", "opus"]:
                        potential_path = os.path.join(self.download_dir, f"{music_id}.{ext}")
                        if os.path.exists(potential_path):
                            audio_path = potential_path
                            break
                    else:
                        logging.error(f"下载的音频文件不存在: {audio_path}")
                        return None

                # 处理缩略图
                thumbnail_url = info.get("thumbnail", "")
                uploaded_thumbnail_url = thumbnail_url  # 默认使用原始缩略图URL

                if uploader:
                    # 查找下载的缩略图文件
                    thumbnail_path = None
                    for ext in ["jpg", "jpeg", "png", "webp"]:
                        potential_thumbnail = os.path.join(self.download_dir, f"{music_id}.{ext}")
                        if os.path.exists(potential_thumbnail):
                            thumbnail_path = potential_thumbnail
                            break

                    if thumbnail_path:
                        # 上传缩略图到存储服务
                        thumbnail_filename = f"{music_id}_thumbnail.{thumbnail_path.split('.')[-1]}"
                        uploaded_thumbnail_url = uploader.upload_file(thumbnail_path, thumbnail_filename)
                        if uploaded_thumbnail_url:
                            logging.info(f"缩略图上传成功: {uploaded_thumbnail_url}")
                        else:
                            logging.warning(f"缩略图上传失败，使用原始URL: {thumbnail_url}")
                            uploaded_thumbnail_url = thumbnail_url

                        # 清理本地缩略图文件
                        self.cleanup_file(thumbnail_path)
                    else:
                        logging.debug(f"未找到本地缩略图文件: {music_id}")

                # 提取音频信息
                music_info = {
                    "id": music_id,
                    "name": info.get("title", "Unknown"),
                    "artist": info.get("uploader", "Unknown"),
                    "album": info.get("album", ""),
                    "duration": int(info.get("duration", 0)),
                    "picture_url": uploaded_thumbnail_url,
                    "web_url": url,
                    "local_path": audio_path,
                    "description": info.get("description", ""),
                    "upload_date": info.get("upload_date", ""),
                    "view_count": info.get("view_count", 0),
                }

                logging.info(f"音频下载成功: {music_info['name']}")
                return music_info

        except Exception as e:
            logging.error(f"下载音频时发生错误: {e}")
            return None

    def cleanup_file(self, file_path: str) -> None:
        """清理临时文件"""
        try:
            if os.path.exists(file_path):
                os.remove(file_path)
                logging.debug(f"已删除临时文件: {file_path}")
        except Exception as e:
            logging.error(f"删除临时文件失败: {e}")


class StorageUploader:
    """存储上传器基类"""

    def upload_audio(self, file_path: str, filename: str) -> str | None:
        """上传音频文件并返回访问URL"""
        raise NotImplementedError

    def upload_file(self, file_path: str, filename: str) -> str | None:
        """上传文件并返回访问URL"""
        raise NotImplementedError


class S3Uploader(StorageUploader):
    """S3存储上传器"""

    def __init__(self, access_key: str, secret_key: str, region: str, bucket: str, endpoint_url: str = None):
        try:
            import boto3

            self.s3_client = boto3.client(
                "s3",
                aws_access_key_id=access_key,
                aws_secret_access_key=secret_key,
                region_name=region,
                endpoint_url=endpoint_url,
            )
            self.bucket = bucket
            self.endpoint_url = endpoint_url
        except ImportError:
            logging.error("boto3 SDK未安装，请运行: pip install boto3")
            raise

    def upload_audio(self, file_path: str, filename: str) -> str | None:
        """上传音频到S3"""
        return self.upload_file(file_path, filename)

    def upload_file(self, file_path: str, filename: str) -> str | None:
        """上传文件到S3"""
        try:
            self.s3_client.upload_file(file_path, self.bucket, filename)

            if self.endpoint_url:
                url = f"{self.endpoint_url}/{self.bucket}/{filename}"
            else:
                url = f"https://{self.bucket}.s3.amazonaws.com/{filename}"

            logging.info(f"文件上传到S3成功: {url}")
            return url
        except Exception as e:
            logging.error(f"S3上传错误: {e}")
            return None


def create_uploader(storage_config) -> StorageUploader | None:
    """根据配置创建上传器"""
    if storage_config.type == "s3":
        return S3Uploader(
            storage_config.s3.get("access_key_id"),
            storage_config.s3.get("secret_access_key"),
            storage_config.s3.get("region"),
            storage_config.s3.get("bucket"),
            storage_config.s3.get("endpoint_url"),
        )
    else:
        logging.error(f"不支持的存储类型: {storage_config.type}，仅支持s3")
        return None
