package service

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"currency-rates/internal/logger"
	"currency-rates/internal/storage"
	"currency-rates/internal/storage/postgres"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

var (
	version    string
	configFile = ".ini"
)

type Service struct {
	log   *logrus.Logger
	ini   *ini.File
	store storage.Storer

	CheckInterval   int64
	TimeoutResponse int64
	TimeoutRequest  int64
}

func Version() {
	fmt.Print("Version=", version)
}

func New() (*Service, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{
		SpaceBeforeInlineComment: true,
	}, configFile)
	if err != nil {
		return nil, fmt.Errorf("load config files:%s", err)
	}
	cfg.NameMapper = ini.TitleUnderscore
	cfgLog := logger.DefaultConfig()
	if err := cfg.Section("logger").MapTo(cfgLog); err != nil {
		return nil, fmt.Errorf("mapping logger config:%s", err)
	}

	return &Service{
		ini:             cfg,
		log:             logger.New(cfgLog),
		CheckInterval:   cfg.Section("service").Key("check_interval").MustInt64(10),
		TimeoutResponse: cfg.Section("service").Key("timeout_response").MustInt64(5),
		TimeoutRequest:  cfg.Section("service").Key("timeout_request").MustInt64(5),
	}, nil
}

func (s *Service) Start() {
	s.log.Infof("***********************SERVICE [%s] START***********************", version)
	mainCtx, globCancel := context.WithCancel(context.Background())
	defer globCancel()

	pool, err := pgxpool.Connect(mainCtx,
		fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s&pool_max_conns=%d",
			s.ini.Section("database").Key("user").MustString("postgres"),
			s.ini.Section("database").Key("password").MustString("postgres"),
			s.ini.Section("database").Key("host").MustString("localhost"),
			s.ini.Section("database").Key("port").MustString("5432"),
			s.ini.Section("database").Key("database").String(),
			s.ini.Section("database").Key("sslmode").MustString("disable"),
			int32(s.ini.Section("database").Key("max_open_conns").MustInt(25)),
		))
	if err != nil {
		s.log.Fatalln("database connect:", err)
	}

	if err := pool.Ping(mainCtx); err != nil {
		s.log.Fatalln("database ping:", err)
	}
	s.store = postgres.New(pool)

	go s.getPeriodiActions(mainCtx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit)

SERVICE:
	for {
		q := <-quit
		switch q {
		case os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
			s.log.Infof("recived signal: %v", q)
			break SERVICE
		default:
			time.Sleep(1 * time.Second)
		}
	}

	globCancel()
	s.log.Info("Service: database connection closed")
	s.log.Info("***********************SERVICE STOP************************")
}

func (s *Service) getPeriodiActions(ctx context.Context) {
	for {

		rates, err := s.currentFxRates()
		if err != nil {
			s.log.Errorln("get current rates:", err)
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		s.log.Infof("request 'getCurrentFxRates' returned [%d] curency rates", len(rates))

		currDate := rates["EUR"].Date

		lastUpd, err := s.store.LastDateUpdate(ctx)
		if err != nil {
			s.log.Errorln("get the date of the last update of the rates: ", err)
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		if currDate.Before(lastUpd) || currDate.Equal(lastUpd) {
			s.log.Infof(
				"the date of the current courses [%s] is not older than the last update [%s] - no update needed",
				currDate.Format("2006-01-02"),
				lastUpd.Format("2006-01-02"),
			)
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		s.log.Infof(
			"the date of the current courses [%s] is older than the last update [%s] - update needed",
			currDate.Format("2006-01-02"),
			lastUpd.Format("2006-01-02"),
		)

		list, err := s.curencyList()
		if err != nil {
			s.log.Errorln("get curency list:", err)
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}
		if len(list) == 0 {
			s.log.Warn("request 'getCurrencyList' returned empty")
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		s.log.Infof("request 'getCurrencyList' returned a successful response")

		for k, v := range list {
			if c, ok := rates[k]; ok {
				c.Name = v
				rates[k] = c
			}
		}

		ok, err := s.store.UpdateCurencyRates(ctx, rates)
		if err != nil {
			s.log.Errorln("update currency rates table:", err)
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		if !ok {
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		s.log.Info("successfully updated the data in the currensy rates table")

		if err := s.store.InvokeBulkLoadUpdate(ctx); err != nil {
			s.log.Errorln("invoke bulk load update procedure:", err)
			time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
			continue
		}

		s.log.Info("successfully called bulk load update procedure")

		time.Sleep(time.Duration(s.CheckInterval) * time.Minute)
	}
}
