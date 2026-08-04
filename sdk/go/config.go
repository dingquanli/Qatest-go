package qa

// QaConfig — Go SDK 配置
// 【填写】把下面三项改成你的真实值：
//   - BaseURL：Qatest 平台地址，结尾不要带斜杠，例如 "http://your-server:3000"
//   - Token：平台「系统设置 → SDK 上报」获取的 report_token
var (
	BaseURL = ""                     // 平台上报地址，必填，结尾不要带斜杠
	Token   = "请填写你的Token"         // 平台「系统设置 → SDK 上报」获取
	Enabled = true                    // 是否开启上报
)
