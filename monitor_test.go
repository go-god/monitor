package monitor

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type helloHandler struct {
}

func (h *helloHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello,world"))
}

// http://localhost:2337/debug/pprof/
// http://localhost:1337/hello
// http://localhost:1337/test
// http://localhost:2337/metrics

// TestMonitorHandler test monitor.
func TestMonitorHandler(t *testing.T) {
	// 添加prometheus性能监控指标
	prometheus.MustRegister(WebRequestTotal)
	prometheus.MustRegister(WebRequestDuration)

	prometheus.MustRegister(CpuTemp)
	prometheus.MustRegister(HdFailures)

	port := 1337

	// 性能监控的端口port+1000,只能在内网访问
	httpMux := http.NewServeMux()
	// 添加prometheus metrics处理器
	httpMux.Handle("/metrics", promhttp.Handler())
	httpMux.HandleFunc("/debug/pprof/", pprof.Index)
	httpMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	httpMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	httpMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	httpMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	httpMux.HandleFunc("/check", check)

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Println("PProf exec recover: ", err)
			}
		}()

		log.Println("server PProf run on: ", port)

		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port+1000), httpMux); err != nil {
			log.Println("PProf listen error: ", err)
		}

	}()

	router := http.NewServeMux()
	router.HandleFunc("/test", MonitorHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte("ok"))
	}))

	router.Handle("/hello", MonitorHandler(&helloHandler{}))

	// 服务server设置
	server := &http.Server{
		Handler:           router,
		Addr:              fmt.Sprintf("0.0.0.0:%d", port),
		IdleTimeout:       20 * time.Second, //tcp idle time
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	// 在独立携程中运行
	log.Println("server run on: ", port)
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	// mock server shutdown.
	time.Sleep(200 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	server.Shutdown(ctx)
	<-ctx.Done()

	log.Println("server shutdown")
}

// check PProf心跳检测
func check(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"alive": true}`))
}
