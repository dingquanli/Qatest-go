# QaSDK — Python SDK 主入口
# 上报地址约定：POST {BASE_URL}/api/qa/report
#
# 完整协议（与 FileApiLogger.cs 模板对齐）：
#   - 用例结果：report(name, result, message, tags)
#   - 自由日志：log(message, tags)
#   - API 拦截事件：log_request / log_response / log_error
#       （对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR 三类 gRPC 拦截事件）
#   - 任意原始上报：send_raw(payload)（可直接转发 FileApiLogger 的 JSONL 行）
# 上报前会对 request / response / headers 中的敏感字段自动脱敏。
# 依赖：pip install requests
import json
import time

import requests

import qa_config as cfg

_initialized = False

# 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
SENSITIVE_KEYS = {"credential", "authtoken", "token", "password", "secret", "apikey", "key", "authorization"}


def init(base_url=None, token=None, enabled=None):
    global _initialized
    if base_url is not None:
        cfg.BASE_URL = base_url
    if token is not None:
        cfg.TOKEN = token
    if enabled is not None:
        cfg.ENABLED = enabled
    _initialized = True


def _redact(value):
    """递归脱敏：把对象中敏感字段的值替换为 ***。"""
    if isinstance(value, list):
        return [_redact(v) for v in value]
    if isinstance(value, dict):
        return {k: ("***" if k.lower() in SENSITIVE_KEYS else _redact(v)) for k, v in value.items()}
    return value


def _build_payload(name, result, message, tags, event="case_result"):
    return {
        "event": event,
        "name": name or "",
        "result": result or "passed",
        "message": message or "",
        "tags": tags or {},
        "timestamp": int(time.time() * 1000),
    }


def _send(payload):
    if not cfg.ENABLED:
        return False
    if not _initialized:
        init()
    # 脱敏：request / response / headers 中的敏感字段
    safe = dict(payload)
    if "headers" in safe:
        safe["headers"] = _redact(safe.get("headers"))
    if "request" in safe:
        safe["request"] = _redact(safe.get("request"))
    if "response" in safe:
        safe["response"] = _redact(safe.get("response"))
    url = cfg.BASE_URL.rstrip("/") + "/api/qa/report"
    headers = {"Content-Type": "application/json"}
    if cfg.TOKEN:
        headers["Authorization"] = "Bearer " + cfg.TOKEN
    try:
        resp = requests.post(url, json=safe, headers=headers, timeout=10)
        return resp.ok
    except Exception as e:  # 上报失败不应阻断主流程
        print("[QaSDK] 上报失败:", e)
        return False


def report(name, result="passed", message="", tags=None):
    """上报一条用例结果。"""
    return _send(_build_payload(name, result, message, tags, "case_result"))


def log(message, tags=None):
    """上报一条日志。"""
    return _send(_build_payload("log", "info", message, tags, "log"))


def log_request(method, headers=None, request=None, seq=0, tags=None, message=""):
    """上报一次 API 请求（对应 FileApiLogger REQUEST）。"""
    return _send({
        "event": "request", "type": "REQUEST", "name": method or "",
        "method": method or "", "headers": headers or {}, "request": request,
        "seq": seq, "tags": tags or {}, "message": message,
        "timestamp": int(time.time() * 1000),
    })


def log_response(method, headers=None, response=None, elapsed_ms=0.0, seq=0, tags=None, message=""):
    """上报一次 API 响应（对应 FileApiLogger RESPONSE）。"""
    return _send({
        "event": "response", "type": "RESPONSE", "name": method or "",
        "method": method or "", "headers": headers or {}, "response": response,
        "elapsed_ms": elapsed_ms, "seq": seq, "tags": tags or {},
        "message": message, "timestamp": int(time.time() * 1000),
    })


def log_error(method, error="", elapsed_ms=0.0, headers=None, tags=None, seq=0):
    """上报一次 API 错误（对应 FileApiLogger ERROR）。"""
    return _send({
        "event": "error", "type": "ERROR", "name": method or "",
        "method": method or "", "headers": headers or {}, "error": error or "",
        "elapsed_ms": elapsed_ms, "seq": seq, "tags": tags or {},
        "message": error or "", "timestamp": int(time.time() * 1000),
    })


def send_raw(payload):
    """任意原始上报（可直接转发 FileApiLogger 的 JSONL 行）。"""
    return _send(payload)


if __name__ == "__main__":
    QaSDK = __import__("qa_sdk")
    QaSDK.init(cfg.BASE_URL, cfg.TOKEN, cfg.ENABLED)
    QaSDK.report("示例用例", "passed", "运行成功")
