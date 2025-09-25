import logging
import ssl

import aiohttp

from .task import Result, Task


class TaskClient:
    """任务客户端，负责获取任务和提交结果"""

    def __init__(self, server_url: str, token: str):
        self.server_url = server_url
        self.token = token
        self.poll_timeout = 30  # 30秒超时

        # 创建SSL上下文，信任所有证书
        ssl_context = ssl.create_default_context()
        ssl_context.check_hostname = False
        ssl_context.verify_mode = ssl.CERT_NONE

        # 创建连接器，支持连接池和长连接
        self.connector = aiohttp.TCPConnector(
            ssl=ssl_context,
            limit=10,  # 连接池大小
            limit_per_host=5,  # 每个主机的最大连接数
            keepalive_timeout=60,  # 保持连接60秒
            enable_cleanup_closed=True,
        )

        # 创建持久化会话
        timeout = aiohttp.ClientTimeout(total=35)  # 稍微长于轮询超时
        self.session = aiohttp.ClientSession(connector=self.connector, timeout=timeout)

    def _get_headers(self) -> dict:
        """获取请求头"""
        headers = {"Music-Let-Version": "v0.0.2"}
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        return headers

    async def get_task(self) -> Task | None:
        """通过长轮询获取任务"""
        url = f"{self.server_url}/tasks/poll?timeout={self.poll_timeout}"

        try:
            async with self.session.get(url, headers=self._get_headers()) as response:
                if response.status == 204:  # No Content
                    return None

                if response.status != 200:
                    raise Exception(f"服务器返回错误状态: {response.status}")

                data = await response.json()
                return Task.from_dict(data)

        except TimeoutError:
            logging.debug("获取任务超时")
            return None
        except Exception as e:
            logging.error(f"获取任务失败: {e}")
            raise

    async def submit_result(self, result: Result) -> None:
        """提交任务结果"""
        url = f"{self.server_url}/tasks/result"

        try:
            headers = self._get_headers()
            headers["Content-Type"] = "application/json"

            async with self.session.post(url, json=result.to_dict(), headers=headers) as response:
                if response.status not in [200, 202]:
                    raise Exception(f"服务器返回错误状态: {response.status}")

        except Exception as e:
            logging.error(f"提交结果失败: {e}")
            raise

    async def close(self):
        """关闭会话和连接器"""
        if self.session and not self.session.closed:
            await self.session.close()
            logging.debug("HTTP会话已关闭")

        if self.connector and not self.connector.closed:
            await self.connector.close()
            logging.debug("HTTP连接器已关闭")
