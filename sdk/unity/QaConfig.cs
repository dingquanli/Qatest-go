// QaConfig — Unity (C#) SDK 配置
// 【填写】把下面三项改成你的真实值
using System;

public static class QaConfig
{
    // 平台上报地址，结尾不要带斜杠
    public static string BaseUrl  = "http://192.168.1.100:3000";
    // 你的 Token
    public static string Token    = "请填写你的Token";
    // 是否开启上报，调试时可先设 false 看日志，没问题再设 true
    public static bool   Enabled = true;
}
