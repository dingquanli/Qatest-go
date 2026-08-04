package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"qatest/config"
	"qatest/database"
	"qatest/routes"

	"github.com/gin-gonic/gin"
)

func main() {
	config.Load()

	if err := database.Init(); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	if err := database.RunMigrations(); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 创建 Gin 引擎
	if config.AppConfig.LogLevel == "DEBUG" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 反代信任配置（P2 修复：默认不信任任何代理，c.ClientIP() 返回直连 IP，
	// 防止伪造 X-Forwarded-For 绕过限流；部署在反代后时用 TRUSTED_PROXIES 显式声明代理网段）
	r := gin.New()
	if len(config.AppConfig.TrustedProxies) == 0 {
		if err := r.SetTrustedProxies(nil); err != nil {
			log.Fatalf("设置 TrustedProxies 失败: %v", err)
		}
	} else {
		if err := r.SetTrustedProxies(config.AppConfig.TrustedProxies); err != nil {
			log.Fatalf("TRUSTED_PROXIES 配置无效: %v", err)
		}
	}

	routes.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.AppConfig.Port),
		Handler: r,
	}

	// 启动服务（goroutine）
	go func() {
		log.Printf("Qatest-go 服务启动，监听端口: %s", config.AppConfig.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务...")

	// 优雅关闭（5 秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务关闭失败: %v", err)
	}

	log.Println("服务已安全关闭")
}
