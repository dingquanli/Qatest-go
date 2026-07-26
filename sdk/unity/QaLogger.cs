// QaLogger — Unity (C#) 日志采集与上报
// 便捷封装 QaSDK.Log，方便在游戏里随处打点。
using System.Collections.Generic;
using System.Threading.Tasks;

public static class QaLogger
{
    /// <summary>上报一条日志，tags 可附带模块等信息。</summary>
    public static Task<bool> Log(string message, Dictionary<string, string> tags = null)
    {
        return QaSDK.Log(message, tags);
    }

    public static Task<bool> Log(string message, string module)
    {
        var tags = new Dictionary<string, string>();
        if (!string.IsNullOrEmpty(module)) tags["module"] = module;
        return QaSDK.Log(message, tags);
    }
}
