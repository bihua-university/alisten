import json
import os
import logging
from typing import Dict, Any, Optional


class StorageConfig:
    def __init__(self, config_data: Dict[str, Any]):
        self.type = config_data.get("type", "")
        self.qiniu = config_data.get("qiniu", {})
        self.s3 = config_data.get("s3", {})


class Config:
    def __init__(self, config_data: Dict[str, Any]):
        self.server_url = config_data.get("server_url", "")
        self.token = config_data.get("token", "")
        self.storage = StorageConfig(config_data.get("storage", {}))
        self.pgsql = config_data.get("pgsql", "")


def load_config(config_path: str = "config.json") -> Optional[Config]:
    """读取配置文件"""
    logging.info(f"尝试读取配置文件: {config_path}")
    
    try:
        with open(config_path, 'r', encoding='utf-8') as file:
            config_data = json.load(file)
            
        config = Config(config_data)
        logging.info(f"配置文件解析成功: ServerURL={config.server_url}, Token长度={len(config.token)}")
        return config
        
    except FileNotFoundError:
        logging.error(f"无法找到配置文件: {config_path}")
        return None
    except json.JSONDecodeError as e:
        logging.error(f"解析配置文件失败: {e}")
        return None
    except Exception as e:
        logging.error(f"读取配置文件时发生错误: {e}")
        return None