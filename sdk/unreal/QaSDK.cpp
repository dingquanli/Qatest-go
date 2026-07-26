// QaSDK — Unreal Engine (C++) SDK 主入口（实现）
// 依赖：UE 自带 FHttpModule；非 UE 环境可替换为 libcurl。
#include "QaSDK.h"

#if WITH_EDITOR || PLATFORM_WINDOWS
#include "HttpModule.h"
#include "Interfaces/IHttpRequest.h"
#include "Interfaces/IHttpResponse.h"
#endif

#include <sstream>
#include <thread>
#include <regex>
#include <vector>
#include <cstdint>

namespace
{
    bool g_initialized = false;

    // 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
    const std::vector<std::string> SensitiveFields = {
        "credential", "authtoken", "token", "password", "secret", "apikey", "key", "authorization"
    };

    std::string Escape(const std::string& s)
    {
        std::string o;
        o.reserve(s.size());
        for (char c : s)
        {
            switch (c)
            {
                case '\\': o += "\\\\"; break;
                case '"': o += "\\\""; break;
                case '\n': o += "\\n"; break;
                case '\r': o += "\\r"; break;
                case '\t': o += "\\t"; break;
                default: o += c;
            }
        }
        return o;
    }

    // 若已是 JSON 对象/数组则原样嵌入，否则转义包裹
    std::string WrapRaw(const std::string& value)
    {
        if (value.empty()) return "null";
        std::string s = value;
        size_t a = s.find_first_not_of(" \t\r\n");
        size_t b = s.find_last_not_of(" \t\r\n");
        if (a == std::string::npos) return "null";
        s = s.substr(a, b - a + 1);
        if (s.front() == '{' || s.front() == '[') return s;
        return "\"" + Escape(s) + "\"";
    }

    // 敏感字段脱敏（对应 FileApiLogger.MaskSensitiveFields）
    std::string MaskSensitive(const std::string& json)
    {
        if (json.empty()) return json;
        std::string out = json;
        for (const auto& f : SensitiveFields)
        {
            try
            {
                std::regex re("(\"" + f + "\"" + "\\s*:\\s*\")" + "[^\"]*" + "(\")",
                              std::regex_constants::icase);
                out = std::regex_replace(out, re, "$1***$2");
            }
            catch (...) {}
        }
        return out;
    }

    std::string BuildJson(const std::string& name,
                          const std::string& result,
                          const std::string& message,
                          const std::map<std::string, std::string>& tags,
                          const std::string& eventType)
    {
        std::ostringstream os;
        os << "{";
        os << "\"event\":\"" << eventType << "\",";
        os << "\"name\":\"" << Escape(name) << "\",";
        os << "\"result\":\"" << (result.empty() ? "passed" : result) << "\",";
        os << "\"message\":\"" << Escape(message) << "\",";
        os << "\"timestamp\":0";
        os << "}";
        return os.str();
    }

    // 拦截事件 JSON（request / response / error / headers）
    std::string BuildApiJson(const std::string& eventType,
                             const std::string& method,
                             const std::map<std::string, std::string>& headers,
                             const std::string& requestJson,
                             const std::string& responseJson,
                             const std::string& error,
                             double elapsedMs,
                             int64_t seq)
    {
        std::ostringstream os;
        os << "{";
        os << "\"event\":\"" << eventType << "\",";
        os << "\"name\":\"" << Escape(method) << "\",";
        os << "\"method\":\"" << Escape(method) << "\",";
        os << "\"type\":\"" << eventType << "\",";
        os << "\"headers\":{";
        {
            bool first = true;
            for (const auto& kv : headers)
            {
                if (!first) os << ",";
                os << "\"" << Escape(kv.first) << "\":\"" << Escape(kv.second) << "\"";
                first = false;
            }
        }
        os << "}";
        if (!requestJson.empty()) os << ",\"request\":" << WrapRaw(requestJson);
        if (!responseJson.empty()) os << ",\"response\":" << WrapRaw(responseJson);
        if (!error.empty()) os << ",\"error\":\"" << Escape(error) << "\"";
        if (elapsedMs != 0.0) os << ",\"elapsed_ms\":" << elapsedMs;
        if (seq != 0) os << ",\"seq\":" << seq;
        os << ",\"timestamp\":0";
        os << "}";
        return os.str();
    }
}

void QaSDK::Init(const std::string& baseUrl, const std::string& token, bool enabled)
{
    if (!baseUrl.empty()) QaConfig::BaseUrl = baseUrl;
    if (!token.empty()) QaConfig::Token = token;
    QaConfig::Enabled = enabled;
    g_initialized = true;
}

bool QaSDK::Report(const std::string& name,
                   const std::string& result,
                   const std::string& message,
                   const std::map<std::string, std::string>& tags)
{
    return SendImpl(BuildJson(name, result, message, tags, "case_result"));
}

bool QaSDK::Log(const std::string& message,
                const std::map<std::string, std::string>& tags)
{
    return SendImpl(BuildJson("log", "info", message, tags, "log"));
}

bool QaSDK::LogRequest(const std::string& method,
                       const std::map<std::string, std::string>& headers,
                       const std::string& requestJson,
                       int64_t seq)
{
    return SendImpl(BuildApiJson("request", method, headers, requestJson, "", "", 0.0, seq));
}

bool QaSDK::LogResponse(const std::string& method,
                        const std::map<std::string, std::string>& headers,
                        const std::string& responseJson,
                        double elapsedMs,
                        int64_t seq)
{
    return SendImpl(BuildApiJson("response", method, headers, "", responseJson, "", elapsedMs, seq));
}

bool QaSDK::LogError(const std::string& method,
                     const std::string& error,
                     double elapsedMs,
                     const std::map<std::string, std::string>& headers,
                     int64_t seq)
{
    return SendImpl(BuildApiJson("error", method, headers, "", "", error, elapsedMs, seq));
}

bool QaSDK::SendRaw(const std::string& json)
{
    return SendImpl(json);
}

// 内部发送：UE 环境用 FHttpModule，非 UE 环境可在此接入 libcurl
bool QaSDK::SendImpl(const std::string& json)
{
    if (!QaConfig::Enabled) return false;
    if (!g_initialized) Init(QaConfig::BaseUrl, QaConfig::Token, QaConfig::Enabled);

    std::string safe = MaskSensitive(json);

    std::string url = QaConfig::BaseUrl;
    while (!url.empty() && url.back() == '/') url.pop_back();
    url += "/api/qa/report";

#if WITH_EDITOR || PLATFORM_WINDOWS
    TSharedRef<IHttpRequest, ESPMode::ThreadSafe> req = FHttpModule::Get().CreateRequest();
    req->SetURL(UTF8_TO_TCHAR(url.c_str()));
    req->SetVerb(TEXT("POST"));
    req->SetHeader(TEXT("Content-Type"), TEXT("application/json"));
    if (!QaConfig::Token.empty())
    {
        req->SetHeader(TEXT("Authorization"), UTF8_TO_TCHAR(("Bearer " + QaConfig::Token).c_str()));
    }
    req->SetContentAsString(UTF8_TO_TCHAR(safe.c_str()));
    req->ProcessRequest();
    return true;
#else
    // 非 UE 环境：在子线程用 libcurl 发送（此处留空，按项目接入）
    std::thread([url, safe]() {
        // curl_easy_init() ... 发送 url + safe
    }).detach();
    return true;
#endif
}
