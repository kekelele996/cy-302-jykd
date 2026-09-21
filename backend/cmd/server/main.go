package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbexam/online-exam/internal/config"
	"github.com/gbexam/online-exam/internal/handler"
	"github.com/gbexam/online-exam/internal/middleware"
	"github.com/gbexam/online-exam/internal/repository"
	"github.com/gbexam/online-exam/internal/router"
	"github.com/gbexam/online-exam/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	db, err := connectDB(cfg, logger)
	if err != nil {
		logger.Error("connect db", "error", err)
		os.Exit(1)
	}

	repo := repository.NewRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		logger.Error("auto migrate", "error", err)
		os.Exit(1)
	}

	authService := service.NewAuthService(repo, cfg, logger)
	userService := service.NewUserService(repo, logger)
	questionService := service.NewQuestionService(repo, logger)
	examService := service.NewExamService(repo, repo, logger)
	attemptService := service.NewAttemptService(repo, repo, repo, repo, repo, logger)
	statsService := service.NewStatsService(repo, logger)
	wrongService := service.NewWrongQuestionService(repo, repo, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := authService.SeedAdmin(ctx); err != nil {
		logger.Error("seed admin", "error", err)
		os.Exit(1)
	}

	server := handler.NewServer(logger, authService, userService, questionService, examService, attemptService, statsService, wrongService)
	engine := router.New(server, middleware.Auth(authService))

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("server starting", "port", cfg.ServerPort, "env", cfg.AppEnv)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

func connectDB(cfg *config.Config, logger *slog.Logger) (*gorm.DB, error) {
	gormConfig := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Warn)}
	var db *gorm.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = gorm.Open(mysql.Open(cfg.DSN()), gormConfig)
		if err == nil {
			var sqlDB *sql.DB
			sqlDB, err = db.DB()
			if err == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					sqlDB.SetMaxOpenConns(20)
					sqlDB.SetMaxIdleConns(10)
					sqlDB.SetConnMaxLifetime(time.Hour)
					return db, nil
				}
			}
		}
		logger.Warn("database not ready, retrying", "attempt", i+1, "error", err)
		time.Sleep(2 * time.Second)
	}
	return nil, fmt.Errorf("connect mysql: %w", err)
}
