// QaSDK — Unreal Engine (C++) SDK 主入口（头文件）
// 上报地址约定：POST {BaseUrl}/api/qa/report
#pragma once

#include <string>
#include <map>

struct QaConfig
{
    static inline std::string BaseUrl = "http://192.168.1.100:3000"; // 【填写】
    static inline std::string Token   = "请填写你的Token";            // 【填写】
    static inline bool         Enabled = true;
};

class QaSDK
{
public:
    static void Init(const std::string& baseUrl, const std::string& token, bool enabled);

    // 上报一条用例结果，返回是否发送成功
    static bool Report(const std::string& name,
                       const std::string& result = "passed",
                       const std::string& message = "",
                       const std::map<std::string, std::string>& tags = {});

    // 上报一条日志
    static bool Log(const std::string& message,
                    const std::map<std::string, std::string>& tags = {});

    // API 拦截事件（对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR）
    static bool LogRequest(const std::string& method,
                           const std::map<std::string, std::string>& headers,
                           const std::string& requestJson,
                           int64_t seq = 0);
    static bool LogResponse(const std::string& method,
                            const std::map<std::string, std::string>& headers,
                            const std::string& responseJson,
                            double elapsedMs = 0.0, int64_t seq = 0);
    static bool LogError(const std::string& method,
                         const std::string& error,
                         double elapsedMs = 0.0,
                         const std::map<std::string, std::string>& headers = {},
                         int64_t seq = 0);

    // 任意原始上报（可直接转发 FileApiLogger 的 JSONL 行）
    static bool SendRaw(const std::string& json);

private:
    // 内部发送实现（UE 用 FHttpModule，非 UE 可接入 libcurl）
    static bool SendImpl(const std::string& json);
};
