package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"kitchens/internal/config"
	kitchengrpc "kitchens/internal/grpc/kitchen"
	"kitchens/internal/http/handlers"
	kafkaprod "kitchens/internal/infrastructure/kafka"
	"kitchens/internal/repo"
	"kitchens/internal/repo/cache"
	"kitchens/internal/service"
	kitchenv1 "kitchens/pkg/api/kitchen/v1"
)

func main() {
	lgr := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg := config.MustLoad()
	lgr.Info("starting application", slog.String("env", cfg.Env))

	ctxBg := context.Background()

	pg, err := repo.NewPostgres(ctxBg, cfg.DB.ConnString)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer pg.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr})
	defer func() {
		if err := rdb.Close(); err != nil {
			lgr.Error("failed to close redis connection", slog.String("error", err.Error()))
		}
	}()

	pingCtx, pingCancel := context.WithTimeout(ctxBg, 2*time.Second)
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		lgr.Warn("redis ping failed, continuing without cache", slog.String("error", err.Error()))
	}
	pingCancel()

	kafkaBrokersStr := os.Getenv("KAFKA_BROKERS")
	if kafkaBrokersStr == "" {
		kafkaBrokersStr = "localhost:9092"
	}
	kafkaProducer := kafkaprod.NewOrderProducer([]string{kafkaBrokersStr}, lgr)
	defer func() {
		if err := kafkaProducer.Close(); err != nil {
			lgr.Error("failed to close kafka producer", slog.String("error", err.Error()))
		}
	}()

	orderRepo := repo.NewOrderRepo(pg)
	restRepo := repo.NewRestaurantRepo(pg)
	menuCache := cache.NewMenuCache(rdb)

	orderSvc := service.NewOrderService(orderRepo, restRepo, kafkaProducer, lgr)
	restSvc := service.NewRestaurantService(restRepo, menuCache, lgr)

	orderHandler := handlers.NewOrderHandler(orderSvc, lgr)
	restHandler := handlers.NewRestaurantHandler(restSvc, lgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /restaurants", restHandler.GetRestaurants)
	mux.HandleFunc("GET /restaurants/{id}/menu", restHandler.GetMenu)
	mux.HandleFunc("POST /orders", orderHandler.CreateOrder)
	mux.HandleFunc("GET /orders/{id}", orderHandler.GetOrder)
	mux.HandleFunc("GET /restaurant/orders", orderHandler.GetRestaurantOrders)
	mux.HandleFunc("PATCH /restaurant/orders/{id}/status", orderHandler.UpdateOrderStatus)

	httpSrv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: mux,
	}

	go func() {
		lgr.Info("HTTP server starting", slog.String("port", cfg.Server.Port))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			lgr.Error("http listen failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	grpcPort := "9000"
	grpcLis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen tcp for grpc: %v", err)
	}

	grpcSrv := grpc.NewServer()
	kitchenv1.RegisterKitchenServiceServer(grpcSrv, kitchengrpc.NewServer(orderSvc, lgr))

	go func() {
		lgr.Info("gRPC server starting", slog.String("port", grpcPort))
		if err := grpcSrv.Serve(grpcLis); err != nil {
			lgr.Error("grpc serve failed", slog.String("error", err.Error()))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	lgr.Info("Shutdown servers ...")

	grpcSrv.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		lgr.Error("HTTP shutdown failed", slog.String("error", err.Error()))
	}

	<-ctx.Done()
	lgr.Info("Servers exited successfully")
}
