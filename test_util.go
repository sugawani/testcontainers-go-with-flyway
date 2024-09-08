package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/go-connections/nat"
	"github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	mysql2 "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type StdoutLogConsumer struct{}

// Accept prints the log to stdout
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}

var (
	dbContainerName = "mysqldb"
	dbName          = "mysql"
	dbPort          = 3306
	dbPortNat       = nat.Port("3306/tcp")
	mysqlImage      = "mysql:8.0"
	flywayImage     = "flyway/flyway:10.17.1"
)

func NewTestDB(ctx context.Context) (*gorm.DB, func(), string) {
	// disable testcontainers log
	testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	containerNetwork, err := network.New(ctx)
	if err != nil {
		panic(err)
	}

	mysqlC, cleanupFunc, err := createMySQLContainer(ctx, containerNetwork.Name)
	if err != nil {
		panic(err)
	}

	if err = execFlywayContainer(ctx, containerNetwork.Name); err != nil {
		panic(err)
	}

	db, err := createDBConnection(ctx, mysqlC)
	if err != nil {
		panic(err)
	}

	return db, cleanupFunc, containerNetwork.Name
}

func createMySQLContainer(ctx context.Context, networkName string) (testcontainers.Container, func(), error) {
	mysqlC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: mysqlImage,
			Env: map[string]string{
				"MYSQL_DATABASE":             dbName,
				"MYSQL_ALLOW_EMPTY_PASSWORD": "yes",
			},
			ExposedPorts: []string{fmt.Sprintf("%d/tcp", dbPort)},
			Tmpfs:        map[string]string{"/var/lib/mysql": "rw"},
			Networks:     []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {dbContainerName},
			},
			WaitingFor: wait.ForLog("port: 3306  MySQL Community Server"),
		},
		Started: true,
	})
	if err != nil {
		return nil, nil, err
	}

	cleanupFunc := func() {
		if err = mysqlC.Terminate(ctx); err != nil {
			panic(err)
		}
	}
	return mysqlC, cleanupFunc, nil
}

func execFlywayContainer(ctx context.Context, networkName string) error {
	mysqlDBUrl := fmt.Sprintf("-url=jdbc:mysql://%s:%d/%s?allowPublicKeyRetrieval=true", dbContainerName, dbPort, dbName)
	fmt.Printf("flyway networkName: %s\n", networkName)
	flywayC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: flywayImage,
			Cmd: []string{
				mysqlDBUrl, "-user=root",
				"baseline", "-baselineVersion=0.0",
				"-locations=filesystem:/flyway", "-validateOnMigrate=false", "migrate"},
			Networks: []string{networkName},
			Files: []testcontainers.ContainerFile{
				{
					HostFilePath:      "migrations",
					ContainerFilePath: "/flyway/sql",
					FileMode:          644,
				},
			},
			WaitingFor: wait.ForLog("Successfully applied|No migration necessary").AsRegexp(),
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
				Consumers: []testcontainers.LogConsumer{&StdoutLogConsumer{}},
			},
		},
	})

	err = backoff.Retry(func() error {
		err = flywayC.Start(ctx)
		if err != nil {
			fmt.Println("flyway Start Error", err)
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))

	time.Sleep(1 * time.Second)
	defer func() {
		if err = flywayC.Terminate(ctx); err != nil {
			panic(err)
		}
	}()
	return err
}

func createDBConnection(ctx context.Context, mysqlC testcontainers.Container) (*gorm.DB, error) {
	host, err := mysqlC.Host(ctx)
	if err != nil {
		fmt.Println("Get MySQL Host Error", err)
		return nil, err
	}
	port, err := mysqlC.MappedPort(ctx, dbPortNat)
	if err != nil {
		fmt.Println("Get MySQL Port Error", err)
		return nil, err
	}
	cfg := mysql.Config{
		DBName:    dbName,
		User:      "root",
		Addr:      fmt.Sprintf("%s:%d", host, port.Int()),
		Net:       "tcp",
		ParseTime: true,
	}
	var db *gorm.DB
	err = backoff.Retry(func() error {
		db, err = gorm.Open(mysql2.Open(cfg.FormatDSN()))
		if err != nil {
			fmt.Println("Open MySQL Error", err)
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))
	if err != nil {
		fmt.Println("Open MySQL Error MaxRetry Exceeded", err)
		return nil, err
	}
	db.Logger = logger.Discard
	err = backoff.Retry(func() error {
		sqlDB, _ := db.DB()
		if err = sqlDB.Ping(); err != nil {
			fmt.Println("Ping MySQL Error", err)
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))
	if err != nil {
		fmt.Println("Ping MySQL Error MaxRetry Exceeded", err)
		return nil, err
	}

	sqlDB, _ := db.DB()
	//sqlDB.SetMaxIdleConns(1)
	//sqlDB.SetMaxOpenConns(1)
	sqlDB.SetConnMaxLifetime(time.Second)
	return db, nil
}
