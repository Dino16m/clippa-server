package main

import (
	"fmt"
	"net/http"

	"github.com/dino16m/clippa-server/internal/data"
	"github.com/dino16m/clippa-server/internal/manager"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func configure() {
	viper.AutomaticEnv()
	viper.SetDefault("Logger.Level", "info")
	viper.SetDefault("PORT", 8080)
	viper.SetDefault("DATABASE_URL", "clippa.db")
}

func main() {
	configure()
	db_url := viper.GetString("DATABASE_URL")
	if db_url == "" {
		logrus.Panic("DATABASE_URL is not set")
	}
	db, err := gorm.Open(sqlite.Open(db_url), &gorm.Config{})
	if err != nil {
		logrus.Panic("failed to connect database")
	}
	db.AutoMigrate(&data.Party{})

	// create the store backed by gorm.DB
	store := data.NewPartyStore(db)

	// create a logger and set level from config
	logger := logrus.New()
	if lvl, err := logrus.ParseLevel(viper.GetString("Logger.Level")); err == nil {
		logger.SetLevel(lvl)
	}

	// instantiate manager controller
	mc := manager.NewManagerCtrl(store, logger)

	// create global API mux and register manager routes
	globalMux := http.NewServeMux()
	mc.RegisterRoutes(globalMux)

	// mount under /api/
	topMux := http.NewServeMux()
	topMux.Handle("/api/", http.StripPrefix("/api", globalMux))

	httpHandle := RequestLogger(logger, topMux)
	portInt := viper.GetInt("PORT")
	listenAddr := fmt.Sprintf(":%d", portInt)
	logrus.Infof("starting server on %s", listenAddr)
	if err := http.ListenAndServe(listenAddr, httpHandle); err != nil {
		logrus.WithError(err).Fatal("server exited")
	}
}

func RequestLogger(logger *logrus.Logger, mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.Infof("Got request %s %s", r.Method, r.URL)
		mux.ServeHTTP(w, r)
		logger.Infof("Finished request %s %s", r.Method, r.URL)
	})
}
