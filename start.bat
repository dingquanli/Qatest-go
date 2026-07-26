@echo off
cd /d "%~dp0"
REM 所有配置项统一从同目录下的 .env 读取（唯一配置源）。
REM 如需临时覆盖，可在本行之前用 set KEY=VALUE，真实环境变量优先级高于 .env。
if not exist ".env" (
    echo [ERROR] 未找到 .env 配置文件，请先根据 .env.example 创建 .env
    pause
    exit /b 1
)
echo Starting Qatest-go on http://localhost:3000  (Ctrl+C to stop)
qatest-server.exe
