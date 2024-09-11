package util

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/go-connections/nat"
	"github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	mysql2 "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	dbContainerName = "mysqldb"
	dbName          = "mysql"
	dbPort          = 3306
	dbPortNat       = nat.Port("3306/tcp")
	mysqlImage      = "mysql:8.0"
	flywayImage     = "flyway/flyway:10.17.1"
)

type StdoutLogConsumer struct{}

// Accept prints the log to stdout
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Print(string(l.Content))
}

func NewTestDB(ctx context.Context) (*gorm.DB, func(), string, string, string) {
	// disable testcontainers log
	//testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	containerNetwork, err := network.New(ctx)
	if err != nil {
		panic(err)
	}

	mysqlC, cleanupFunc, err := createMySQLContainer(ctx, containerNetwork.Name)
	if err != nil {
		panic(err)
	}

	ip, _ := mysqlC.ContainerIP(ctx)
	if err = execFlywayContainer(ctx, containerNetwork.Name, ip); err != nil {
		panic(err)
	}

	db, err := createDBConnection(ctx, mysqlC)
	if err != nil {
		panic(err)
	}
	cleanupF := func() {
		cleanupFunc()
		if err = containerNetwork.Remove(ctx); err != nil {
			panic(err)
		}
	}

	host, _ := mysqlC.Host(ctx)
	port, _ := mysqlC.MappedPort(ctx, dbPortNat)

	return db, cleanupF, host, port.Port(), ip
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

func execFlywayContainer(ctx context.Context, networkName string, ip string) error {
	mysqlDBUrl := fmt.Sprintf("-url=jdbc:mysql://%s:%d/%s?allowPublicKeyRetrieval=true", ip, dbPort, dbName)
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
					HostFilePath:      "../migrations",
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
	if err != nil {
		fmt.Println("flyway GenericContainer Error", err)
		return err
	}

	err = backoff.Retry(func() error {
		err = flywayC.Start(ctx)
		if err != nil {
			fmt.Println("flyway Start Error", err)
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
	if err != nil {
		fmt.Println("flyway start Error", err)
		return err
	}

	defer func() {
		if err = flywayC.Terminate(ctx); err != nil {
			panic(err)
		}
	}()
	return nil
}

func createDBConnection(ctx context.Context, mysqlC testcontainers.Container) (*gorm.DB, error) {
	host, err := mysqlC.Host(ctx)
	if err != nil {
		return nil, err
	}
	port, err := mysqlC.MappedPort(ctx, dbPortNat)
	if err != nil {
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
			fmt.Println("gorm.Open Error", err)
			return err
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
	if err != nil {
		return nil, err
	}
	db.Logger = logger.Discard
	return db, nil
}
