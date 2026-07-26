// QaSDK — Android (Java) SDK 主入口
// 上报地址约定：POST {BASE_URL}/api/qa/report
//
// 完整协议（与 FileApiLogger.cs 模板对齐）：
//   - 用例结果：report(name, result, message, tags)
//   - 自由日志：log(message)
//   - API 拦截事件：logRequest / logResponse / logError
//       （对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR 三类 gRPC 拦截事件）
//   - 任意原始上报：sendRaw(JSONObject)（可直接转发 FileApiLogger 的 JSONL 行）
// 上报前会对 request / response / headers 中的敏感字段自动脱敏。
// 注意：网络请求必须在子线程调用（SDK 内部已用异步线程）。
package com.qatest.qa;

import android.os.Handler;
import android.os.Looper;

import org.json.JSONException;
import org.json.JSONObject;

import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.regex.Pattern;

public class QaSDK {
    // 【填写】上报地址，结尾不要带斜杠
    private static String BASE_URL = "http://192.168.1.100:3000";
    // 【填写】Token
    private static String TOKEN = "请填写你的Token";
    private static boolean ENABLED = true;
    private static boolean initialized = false;

    private static final ExecutorService pool = Executors.newSingleThreadExecutor();

    // 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
    private static final String[] SENSITIVE_FIELDS = {"credential", "authtoken", "token", "password", "secret", "apikey", "key", "authorization"};

    public static void init(String baseUrl, String token, boolean enabled) {
        if (baseUrl != null && !baseUrl.isEmpty()) BASE_URL = baseUrl;
        if (token != null) TOKEN = token;
        ENABLED = enabled;
        initialized = true;
    }

    // 脱敏：把 JSON 字符串中敏感字段的值替换为 ***
    private static String maskSensitive(String json) {
        if (json == null || json.isEmpty()) return json;
        String out = json;
        for (String f : SENSITIVE_FIELDS) {
            try {
                out = Pattern.compile("(\"" + f + "\"\\s*:\\s*\")[^\"]*(\")", Pattern.CASE_INSENSITIVE)
                        .matcher(out).replaceAll("$1***$2");
            } catch (Exception ignored) {
            }
        }
        return out;
    }

    private static JSONObject buildPayload(String name, String result, String message, JSONObject tags, String event) {
        JSONObject p = new JSONObject();
        try {
            p.put("event", event == null || event.isEmpty() ? "case_result" : event);
            p.put("name", name == null ? "" : name);
            p.put("result", result == null || result.isEmpty() ? "passed" : result);
            p.put("message", message == null ? "" : message);
            p.put("tags", tags == null ? new JSONObject() : tags);
            p.put("timestamp", System.currentTimeMillis());
        } catch (JSONException ignored) {
        }
        return p;
    }

    private static boolean doSend(JSONObject payload) {
        if (!ENABLED) return false;
        if (!initialized) init(BASE_URL, TOKEN, ENABLED);
        try {
            String safe = maskSensitive(payload.toString());
            URL url = new URL(BASE_URL.replaceAll("/$", "") + "/api/qa/report");
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();
            conn.setRequestMethod("POST");
            conn.setConnectTimeout(10000);
            conn.setReadTimeout(10000);
            conn.setDoOutput(true);
            conn.setRequestProperty("Content-Type", "application/json");
            if (TOKEN != null && !TOKEN.isEmpty()) {
                conn.setRequestProperty("Authorization", "Bearer " + TOKEN);
            }
            try (OutputStream os = conn.getOutputStream()) {
                os.write(safe.getBytes(StandardCharsets.UTF_8));
            }
            int code = conn.getResponseCode();
            conn.disconnect();
            return code >= 200 && code < 300;
        } catch (Exception e) {
            android.util.Log.e("QaSDK", "上报失败: " + e.getMessage());
            return false;
        }
    }

    /** 上报一条用例结果（在子线程执行）。 */
    public static void report(String name, String result, String message) {
        report(name, result, message, null);
    }

    public static void report(final String name, final String result, final String message, final JSONObject tags) {
        pool.execute(() -> doSend(buildPayload(name, result, message, tags, "case_result")));
    }

    /** 上报一条日志（在子线程执行）。 */
    public static void log(final String message) {
        pool.execute(() -> doSend(buildPayload("log", "info", message, null, "log")));
    }

    /** 上报一次 API 请求（对应 FileApiLogger REQUEST）。 */
    public static void logRequest(final String method, final JSONObject headers, final JSONObject request, final int seq) {
        pool.execute(() -> {
            JSONObject p = buildPayload(method, "", "", null, "request");
            try {
                p.put("type", "REQUEST");
                p.put("method", method == null ? "" : method);
                p.put("headers", headers == null ? new JSONObject() : headers);
                p.put("request", request == null ? JSONObject.NULL : request);
                p.put("seq", seq);
            } catch (JSONException ignored) {
            }
            doSend(p);
        });
    }

    /** 上报一次 API 响应（对应 FileApiLogger RESPONSE）。 */
    public static void logResponse(final String method, final JSONObject headers, final JSONObject response, final double elapsedMs, final int seq) {
        pool.execute(() -> {
            JSONObject p = buildPayload(method, "", "", null, "response");
            try {
                p.put("type", "RESPONSE");
                p.put("method", method == null ? "" : method);
                p.put("headers", headers == null ? new JSONObject() : headers);
                p.put("response", response == null ? JSONObject.NULL : response);
                p.put("elapsed_ms", elapsedMs);
                p.put("seq", seq);
            } catch (JSONException ignored) {
            }
            doSend(p);
        });
    }

    /** 上报一次 API 错误（对应 FileApiLogger ERROR）。 */
    public static void logError(final String method, final String error, final double elapsedMs, final JSONObject headers, final int seq) {
        pool.execute(() -> {
            JSONObject p = buildPayload(method, "error", error, null, "error");
            try {
                p.put("type", "ERROR");
                p.put("method", method == null ? "" : method);
                p.put("headers", headers == null ? new JSONObject() : headers);
                p.put("error", error == null ? "" : error);
                p.put("elapsed_ms", elapsedMs);
                p.put("seq", seq);
            } catch (JSONException ignored) {
            }
            doSend(p);
        });
    }

    /** 任意原始上报（可直接转发 FileApiLogger 的 JSONL 行）。 */
    public static void sendRaw(final JSONObject payload) {
        pool.execute(() -> doSend(payload));
    }
}
