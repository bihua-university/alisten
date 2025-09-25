from dataclasses import dataclass
from typing import Any


@dataclass
class Task:
    """表示一个任务"""

    id: str
    type: str
    data: dict[str, str]

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "Task":
        return cls(id=data["id"], type=data["type"], data=data.get("payload", {}))


@dataclass
class Result:
    """表示任务执行结果"""

    id: str
    success: bool
    result: dict[str, Any] | None = None
    error: str | None = None

    def to_dict(self) -> dict[str, Any]:
        result_dict = {"id": self.id, "success": self.success}
        if self.result is not None:
            result_dict["result"] = self.result
        if self.error is not None:
            result_dict["error"] = self.error
        return result_dict

    @classmethod
    def new_result(cls, task_id: str, success: bool) -> "Result":
        return cls(id=task_id, success=success)

    @classmethod
    def new_result_with_error(cls, task_id: str, error: str) -> "Result":
        return cls(id=task_id, success=False, error=error)
