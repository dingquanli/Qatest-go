// QaSDK — Unity (C#) SDK 主入口
// 上报地址约定：POST {BaseUrl}/api/qa/report
//
// 完整协议（与 FileApiLogger.cs 模板对齐）：
//   - 用例结果：Report(name, result, message, tags)
//   - 自由日志：Log(message, tags)
//   - API 拦截事件：LogRequest / LogResponse / LogError
//       （对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR 三类 gRPC 拦截事件）
//   - 任意原始上报：SendRaw(json)（可直接转发 FileApiLogger 的 JSONL 行）
// 上报前会对 request / response / headers 中的敏感字段自动脱敏（对应 FileApiLogger.MaskSensitiveFields）。
using System;
using System.Collections.Generic;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using UnityEngine;
using UnityEngine.Networking;

public static class QaSDK
{
    private static bool initialized = false;

    // 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
    private static readonly string[] SensitiveFields = { "credential", "authtoken", "token", "password", "secret", "apikey", "key", "authorization" };

    public static void Init(string baseUrl, string token, bool enabled)
    {
        if (!string.IsNullOrEmpty(baseUrl)) QaConfig.BaseUrl = baseUrl;
        if (token != null) QaConfig.Token = token;
        QaConfig.Enabled = enabled;
        initialized = true;
    }

    [Serializable]
    private class ReportPayload
    {
        public string eventType = "case_result";
        public string name;
        public string result = "passed";
        public string message;
        public Dictionary<string, string> tags = new Dictionary<string, string>();
        public long timestamp;
        // 拦截事件字段（镜像 FileApiLogger.cs）
        public string type;
        public string method;
        public Dictionary<string, string> headers;
        public string request;   // 原始 JSON 字符串或 null
        public string response;  // 原始 JSON 字符串或 null
        public string error;
        public double elapsedMs;
        public int seq;
    }

    private static ReportPayload Build(string name, string result, string message, Dictionary<string, string> tags, string eventType)
    {
        return new ReportPayload
        {
            eventType = string.IsNullOrEmpty(eventType) ? "case_result" : eventType,
            name = name ?? "",
            result = string.IsNullOrEmpty(result) ? "passed" : result,
            message = message ?? "",
            tags = tags ?? new Dictionary<string, string>(),
            timestamp = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
        };
    }

    private static async Task<bool> Send(ReportPayload p)
    {
        if (!QaConfig.Enabled) return false;
        if (!initialized) Init(QaConfig.BaseUrl, QaConfig.Token, QaConfig.Enabled);

        string json = MaskSensitive(ToJson(p));
        using (UnityWebRequest req = new UnityWebRequest(QaConfig.BaseUrl.TrimEnd('/') + "/api/qa/report", "POST"))
        {
            byte[] body = Encoding.UTF8.GetBytes(json);
            req.uploadHandler = new UploadHandlerRaw(body);
            req.downloadHandler = new DownloadHandlerBuffer();
            req.SetRequestHeader("Content-Type", "application/json");
            if (!string.IsNullOrEmpty(QaConfig.Token))
                req.SetRequestHeader("Authorization", "Bearer " + QaConfig.Token);

            await req.SendWebRequest();
            return !req.isNetworkError && !req.isHttpError;
        }
    }

    /// <summary>上报一条用例结果。</summary>
    public static async Task<bool> Report(string name, string result = "passed", string message = "", Dictionary<string, string> tags = null)
    {
        return await Send(Build(name, result, message, tags, "case_result"));
    }

    /// <summary>上报一条日志。</summary>
    public static async Task<bool> Log(string message, Dictionary<string, string> tags = null)
    {
        return await Send(Build("log", "info", message, tags, "log"));
    }

    /// <summary>上报一次 API 请求（对应 FileApiLogger REQUEST）。</summary>
    public static async Task<bool> LogRequest(string method, Dictionary<string, string> headers = null, string requestJson = null, int seq = 0, Dictionary<string, string> tags = null)
    {
        var p = Build(method, "", "", tags, "request");
        p.type = "REQUEST"; p.method = method; p.headers = headers; p.request = requestJson; p.seq = seq;
        return await Send(p);
    }

    /// <summary>上报一次 API 响应（对应 FileApiLogger RESPONSE）。</summary>
    public static async Task<bool> LogResponse(string method, Dictionary<string, string> headers = null, string responseJson = null, double elapsedMs = 0, int seq = 0, Dictionary<string, string> tags = null)
    {
        var p = Build(method, "", "", tags, "response");
        p.type = "RESPONSE"; p.method = method; p.headers = headers; p.response = responseJson; p.elapsedMs = elapsedMs; p.seq = seq;
        return await Send(p);
    }

    /// <summary>上报一次 API 错误（对应 FileApiLogger ERROR）。</summary>
    public static async Task<bool> LogError(string method, string error = "", double elapsedMs = 0, Dictionary<string, string> headers = null, int seq = 0, Dictionary<string, string> tags = null)
    {
        var p = Build(method, "error", error, tags, "error");
        p.type = "ERROR"; p.method = method; p.headers = headers; p.error = error; p.elapsedMs = elapsedMs; p.seq = seq;
        return await Send(p);
    }

    /// <summary>任意原始上报（可直接转发 FileApiLogger 的 JSONL 行，字段会被服务端归一）。</summary>
    public static async Task<bool> SendRaw(string json)
    {
        if (!QaConfig.Enabled) return false;
        if (!initialized) Init(QaConfig.BaseUrl, QaConfig.Token, QaConfig.Enabled);
        string safe = MaskSensitive(json);
        using (UnityWebRequest req = new UnityWebRequest(QaConfig.BaseUrl.TrimEnd('/') + "/api/qa/report", "POST"))
        {
            byte[] body = Encoding.UTF8.GetBytes(safe);
            req.uploadHandler = new UploadHandlerRaw(body);
            req.downloadHandler = new DownloadHandlerBuffer();
            req.SetRequestHeader("Content-Type", "application/json");
            if (!string.IsNullOrEmpty(QaConfig.Token))
                req.SetRequestHeader("Authorization", "Bearer " + QaConfig.Token);
            await req.SendWebRequest();
            return !req.isNetworkError && !req.isHttpError;
        }
    }

    // 手动序列化为 JSON。
    // 注意：Unity 的 JsonUtility 不支持 Dictionary（tags 会被序列化为 {}），
    // 且字段名直接作为 JSON key，会把 eventType 原样写出而非服务器契约要求的 event。
    // 因此这里手写序列化，确保与服务器 /api/qa/report 契约一致。
    private static string ToJson(ReportPayload p)
    {
        var sb = new StringBuilder();
        sb.Append('{');
        sb.Append("\"event\":\"").Append(Escape(p.eventType)).Append("\",");
        sb.Append("\"name\":\"").Append(Escape(p.name)).Append("\",");
        sb.Append("\"result\":\"").Append(Escape(p.result)).Append("\",");
        sb.Append("\"message\":\"").Append(Escape(p.message)).Append("\",");
        sb.Append("\"tags\":{");
        if (p.tags != null)
        {
            bool first = true;
            foreach (var kv in p.tags)
            {
                if (!first) sb.Append(',');
                sb.Append('"').Append(Escape(kv.Key)).Append("\":\"").Append(Escape(kv.Value)).Append('"');
                first = false;
            }
        }
        sb.Append("},");
        sb.Append("\"timestamp\":").Append(p.timestamp);
        // 可选拦截字段
        if (!string.IsNullOrEmpty(p.type)) sb.Append(",\"type\":\"").Append(Escape(p.type)).Append("\"");
        if (!string.IsNullOrEmpty(p.method)) sb.Append(",\"method\":\"").Append(Escape(p.method)).Append("\"");
        if (p.headers != null && p.headers.Count > 0)
        {
            sb.Append(",\"headers\":{");
            bool first = true;
            foreach (var kv in p.headers)
            {
                if (!first) sb.Append(',');
                sb.Append('"').Append(Escape(kv.Key)).Append("\":\"").Append(Escape(kv.Value)).Append('"');
                first = false;
            }
            sb.Append('}');
        }
        if (!string.IsNullOrEmpty(p.request)) sb.Append(",\"request\":").Append(WrapRaw(p.request));
        if (!string.IsNullOrEmpty(p.response)) sb.Append(",\"response\":").Append(WrapRaw(p.response));
        if (!string.IsNullOrEmpty(p.error)) sb.Append(",\"error\":\"").Append(Escape(p.error)).Append("\"");
        if (p.elapsedMs != 0) sb.Append(",\"elapsed_ms\":").Append(p.elapsedMs.ToString("F2"));
        if (p.seq != 0) sb.Append(",\"seq\":").Append(p.seq);
        sb.Append('}');
        return sb.ToString();
    }

    // 如果 payload 已是 JSON 对象/数组则原样嵌入，否则按字符串转义包裹
    private static string WrapRaw(string value)
    {
        if (string.IsNullOrEmpty(value)) return "null";
        var s = value.Trim();
        if (s.StartsWith("{") || s.StartsWith("[")) return s;
        return "\"" + Escape(value) + "\"";
    }

    private static string Escape(string s)
    {
        if (string.IsNullOrEmpty(s)) return "";
        return s.Replace("\\", "\\\\")
                .Replace("\"", "\\\"")
                .Replace("\n", "\\n")
                .Replace("\r", "\\r")
                .Replace("\t", "\\t");
    }

    // 敏感字段脱敏（对应 FileApiLogger.MaskSensitiveFields）
    private static string MaskSensitive(string json)
    {
        if (string.IsNullOrEmpty(json)) return json;
        var outStr = json;
        foreach (var field in SensitiveFields)
        {
            try
            {
                outStr = Regex.Replace(outStr,
                    "(\"" + field + "\"\\s*:\\s*\")[^\"]*(\")",
                    "$1***$2",
                    RegexOptions.IgnoreCase);
            }
            catch { }
        }
        return outStr;
    }
}
