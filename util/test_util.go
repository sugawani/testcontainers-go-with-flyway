package util

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types/container"
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

var (
	dbName      = "mysql"
	dbPort      = 3306
	dbPortNat   = nat.Port("3306/tcp")
	mysqlImage  = "mysql:8.0"
	flywayImage = "flyway/flyway:10.17.1"
)

type StdoutLogConsumer struct {
	n string
}

type Util struct {
	N string
}

// Accept prints the log to stdout
func (lc *StdoutLogConsumer) Accept(l testcontainers.Log) {
	fmt.Printf("[🐱🐱🐱 %s] name: %s, log: %s\n", time.Now().Format(time.RFC3339Nano), lc.n, l.Content)
}

func (u Util) errLog(e string, err error) {
	fmt.Printf("[🐽🐽🐽 %s] name: %s, %s, error: %v\n", time.Now().Format(time.RFC3339Nano), u.N, e, err)
}

func (u Util) infoLog(e string) {
	fmt.Printf("[🐶🐶🐶 %s] name: %s, %s\n", time.Now().Format(time.RFC3339Nano), u.N, e)
}

func (u Util) NewTestDB(ctx context.Context) (*gorm.DB, func(), string, string, string) {
	// disable testcontainers log
	testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	containerNetwork, err := network.New(ctx)
	if err != nil {
		panic(err)
	}

	mysqlC, cleanupFunc, err := u.createMySQLContainer(ctx, containerNetwork.Name)
	if err != nil {
		panic(err)
	}

	ip, _ := mysqlC.ContainerIP(ctx)
	if err = u.execFlywayContainer(ctx, containerNetwork.Name, ip); err != nil {
		panic(err)
	}

	db, err := u.createDBConnection(ctx, mysqlC)
	if err != nil {
		panic(err)
	}

	host, _ := mysqlC.Host(ctx)
	port, _ := mysqlC.MappedPort(ctx, dbPortNat)

	return db, cleanupFunc, host, port.Port(), ip
}

func (u Util) createMySQLContainer(ctx context.Context, networkName string) (testcontainers.Container, func(), error) {
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
			WaitingFor:   wait.ForLog("port: 3306  MySQL Community Server"),
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

func (u Util) execFlywayContainer(ctx context.Context, networkName string, mysqlIP string) error {
	mysqlDBUrl := fmt.Sprintf("-url=jdbc:mysql://%s:%d/%s?allowPublicKeyRetrieval=true", mysqlIP, dbPort, dbName)
	flywayFn := func() error {
		_, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
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
				WaitingFor: wait.ForLog("Successfully applied|No migration necessary").AsRegexp().WithOccurrence(1),
				HostConfigModifier: func(c *container.HostConfig) {
					c.AutoRemove = true
				},
			},
			Started: true,
		})
		return err
	}
	err := backoff.Retry(func() error {
		return flywayFn()
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3))
	if err != nil {
		u.errLog("flyway GenericContainer Error", err)
		return err
	}

	return nil
}

func (u Util) createDBConnection(ctx context.Context, mysqlC testcontainers.Container) (*gorm.DB, error) {
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
			u.errLog("gorm.Open Error", err)
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
