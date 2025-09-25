import logging
from dataclasses import dataclass
from datetime import datetime
from typing import Any

import psycopg2
from psycopg2.extras import DictCursor


@dataclass
class MusicModel:
    """音乐数据模型"""

    id: int | None = None
    music_id: str = ""
    name: str = ""
    artist: str = ""
    album_name: str = ""
    picture_url: str = ""
    duration: int = 0
    url: str = ""
    lyric: str = ""
    play_count: int = 0
    created_at: datetime | None = None
    updated_at: datetime | None = None
    deleted_at: datetime | None = None


class Database:
    """数据库操作类"""

    def __init__(self, dsn: str):
        self.dsn = dsn
        self.connection = None
        self.init_db()

    def init_db(self):
        """初始化数据库连接和表结构"""
        try:
            self.connection = psycopg2.connect(self.dsn)
            self.connection.autocommit = True

            # 创建音乐表
            self.create_music_table()
            logging.info("数据库连接和表结构初始化成功")

        except Exception as e:
            logging.error(f"数据库初始化失败: {e}")
            raise

    def create_music_table(self):
        """创建音乐表"""
        create_table_sql = """
        CREATE TABLE IF NOT EXISTS music_models (
            id SERIAL PRIMARY KEY,
            music_id VARCHAR(255) UNIQUE NOT NULL,
            name VARCHAR(255) NOT NULL,
            artist VARCHAR(255),
            album_name VARCHAR(255),
            picture_url TEXT,
            duration BIGINT DEFAULT 0,
            url TEXT,
            lyric TEXT,
            play_count INTEGER DEFAULT 0,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            deleted_at TIMESTAMP WITH TIME ZONE
        );

        CREATE INDEX IF NOT EXISTS idx_music_models_music_id ON music_models(music_id);
        CREATE INDEX IF NOT EXISTS idx_music_models_name ON music_models(name);
        CREATE INDEX IF NOT EXISTS idx_music_models_artist ON music_models(artist);
        CREATE INDEX IF NOT EXISTS idx_music_models_deleted_at ON music_models(deleted_at);
        """

        with self.connection.cursor() as cursor:
            cursor.execute(create_table_sql)

    def insert_music(self, music_id: str, music_info: dict[str, Any]) -> bool:
        """插入或更新音乐记录"""
        if not self.connection:
            logging.warning("数据库未初始化，跳过保存")
            return False

        try:
            # 检查音乐是否已存在
            existing_music = self.get_music_by_id(music_id)

            if existing_music:
                # 更新现有记录
                update_sql = """
                UPDATE music_models
                SET name = %s, artist = %s, album_name = %s, picture_url = %s,
                    duration = %s, url = %s, lyric = %s, updated_at = CURRENT_TIMESTAMP
                WHERE music_id = %s AND deleted_at IS NULL
                """

                with self.connection.cursor() as cursor:
                    cursor.execute(
                        update_sql,
                        (
                            music_info.get("name", ""),
                            music_info.get("artist", ""),
                            music_info.get("album", ""),
                            music_info.get("picture_url", ""),
                            music_info.get("duration", 0),
                            music_info.get("url", ""),
                            music_info.get("description", ""),  # 使用description作为lyric
                            music_id,
                        ),
                    )

                logging.info(f"音乐记录更新成功: {music_id}")
                return True
            else:
                # 插入新记录
                insert_sql = """
                INSERT INTO music_models (music_id, name, artist, album_name, picture_url, duration, url, lyric)
                VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                """

                with self.connection.cursor() as cursor:
                    cursor.execute(
                        insert_sql,
                        (
                            music_id,
                            music_info.get("name", ""),
                            music_info.get("artist", ""),
                            music_info.get("album", ""),
                            music_info.get("picture_url", ""),
                            music_info.get("duration", 0),
                            music_info.get("url", ""),
                            music_info.get("description", ""),  # 使用description作为lyric
                        ),
                    )

                logging.info(f"音乐记录插入成功: {music_id}")
                return True

        except Exception as e:
            logging.error(f"保存音乐记录失败: {e}")
            return False

    def get_music_by_id(self, music_id: str) -> MusicModel | None:
        """根据ID获取音乐记录"""
        if not self.connection:
            return None

        try:
            select_sql = """
            SELECT id, music_id, name, artist, album_name, picture_url, duration, url, lyric, play_count,
                   created_at, updated_at, deleted_at
            FROM music_models
            WHERE music_id = %s AND deleted_at IS NULL
            """

            with self.connection.cursor(cursor_factory=DictCursor) as cursor:
                cursor.execute(select_sql, (music_id,))
                row = cursor.fetchone()

                if row:
                    return MusicModel(
                        id=row["id"],
                        music_id=row["music_id"],
                        name=row["name"],
                        artist=row["artist"],
                        album_name=row["album_name"],
                        picture_url=row["picture_url"],
                        duration=row["duration"],
                        url=row["url"],
                        lyric=row["lyric"],
                        play_count=row["play_count"],
                        created_at=row["created_at"],
                        updated_at=row["updated_at"],
                        deleted_at=row["deleted_at"],
                    )
                return None

        except Exception as e:
            logging.error(f"查询音乐记录失败: {e}")
            return None

    def search_music(self, keyword: str, page: int = 1, page_size: int = 20) -> tuple[list[MusicModel], int]:
        """搜索音乐"""
        if not self.connection:
            return [], 0

        try:
            # 计算偏移量
            offset = (page - 1) * page_size

            # 获取总数
            count_sql = """
            SELECT COUNT(*) as total
            FROM music_models
            WHERE (name ILIKE %s OR artist ILIKE %s) AND deleted_at IS NULL
            """

            with self.connection.cursor() as cursor:
                search_term = f"%{keyword}%"
                cursor.execute(count_sql, (search_term, search_term))
                total = cursor.fetchone()[0]

            # 获取分页数据
            search_sql = """
            SELECT id, music_id, name, artist, album_name, picture_url, duration, url, lyric, play_count,
                   created_at, updated_at, deleted_at
            FROM music_models
            WHERE (name ILIKE %s OR artist ILIKE %s) AND deleted_at IS NULL
            ORDER BY play_count DESC
            LIMIT %s OFFSET %s
            """

            with self.connection.cursor(cursor_factory=DictCursor) as cursor:
                cursor.execute(search_sql, (search_term, search_term, page_size, offset))
                rows = cursor.fetchall()

                musics = []
                for row in rows:
                    musics.append(
                        MusicModel(
                            id=row["id"],
                            music_id=row["music_id"],
                            name=row["name"],
                            artist=row["artist"],
                            album_name=row["album_name"],
                            picture_url=row["picture_url"],
                            duration=row["duration"],
                            url=row["url"],
                            lyric=row["lyric"],
                            play_count=row["play_count"],
                            created_at=row["created_at"],
                            updated_at=row["updated_at"],
                            deleted_at=row["deleted_at"],
                        )
                    )

                return musics, total

        except Exception as e:
            logging.error(f"搜索音乐失败: {e}")
            return [], 0

    def convert_to_map(self, music: MusicModel) -> dict[str, str]:
        """将MusicModel转换为API格式的字典"""
        web_url = music.url if music.url else ""

        return {
            "type": "music",
            "id": music.music_id,
            "url": music.url,
            "webUrl": web_url,
            "pictureUrl": music.picture_url,
            "duration": str(music.duration),
            "lyric": music.lyric,
            "artist": music.artist,
            "name": music.name,
            "album": music.album_name,
            "playCount": str(music.play_count),
        }

    def close(self):
        """关闭数据库连接"""
        if self.connection:
            self.connection.close()
            logging.info("数据库连接已关闭")


# 全局数据库实例
db_instance: Database | None = None


def init_db(dsn: str):
    """初始化数据库"""
    global db_instance
    db_instance = Database(dsn)


def get_music_by_id(music_id: str) -> MusicModel | None:
    """根据ID获取音乐"""
    if db_instance:
        return db_instance.get_music_by_id(music_id)
    return None


def insert_music(music_id: str, music_info: dict[str, Any]) -> bool:
    """插入音乐记录"""
    if db_instance:
        return db_instance.insert_music(music_id, music_info)
    return False


def search_music_by_db(keyword: str, page: int, page_size: int) -> tuple[list[MusicModel], int]:
    """搜索音乐"""
    if db_instance:
        return db_instance.search_music(keyword, page, page_size)
    return [], 0


def convert_to_map(music: MusicModel) -> dict[str, str]:
    """转换为字典格式"""
    if db_instance:
        return db_instance.convert_to_map(music)
    return {}
