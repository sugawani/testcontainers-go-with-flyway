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

var (
	dbContainerName = "mysqldb"
	dbName          = "mysql"
	dbPort          = 3306
	dbPortNat       = nat.Port("3306/tcp")
	mysqlImage      = "mysql:8.0"
	flywayImage     = "flyway/flyway:10.17.1"
)

func (u Util) NewTestDB(ctx context.Context) (*gorm.DB, func()) {
	// disable testcontainers log
	//testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	var (
		containerNetwork *testcontainers.DockerNetwork
		err              error
	)
	err = backoff.Retry(func() error {
		containerNetwork, err = network.New(ctx)
		return err
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(time.Second), 10))
	if err != nil {
		panic(err)
	}

	mysqlC, cleanupFunc, err := u.createMySQLContainer(ctx, containerNetwork.Name)
	if err != nil {
		panic(err)
	}

	if err = u.execFlywayContainer(ctx, containerNetwork.Name); err != nil {
		panic(err)
	}

	db, err := u.createDBConnection(ctx, mysqlC, containerNetwork.Name)
	if err != nil {
		panic(err)
	}
	cleanupF := func() {
		cleanupFunc()
		if err = containerNetwork.Remove(ctx); err != nil {
			panic(err)
		}
	}

	return db, cleanupF
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
		if mysqlC.IsRunning() {
			if err = mysqlC.Terminate(ctx); err != nil {
				panic(err)
			}
		}
	}
	return mysqlC, cleanupFunc, nil
}

func (u Util) execFlywayContainer(ctx context.Context, networkName string) error {
	mysqlDBUrl := fmt.Sprintf("-url=jdbc:mysql://%s:%d/%s?allowPublicKeyRetrieval=true", dbContainerName, dbPort, dbName)
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
		},
		Started: true,
	})
	if err != nil {
		return err
	}

	defer func() {
		if err = flywayC.Terminate(ctx); err != nil {
			panic(err)
		}
	}()
	return err
}

func (u Util) createDBConnection(ctx context.Context, mysqlC testcontainers.Container, networkName string) (*gorm.DB, error) {
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
	db, err := gorm.Open(mysql2.Open(cfg.FormatDSN()))
	if err != nil {
		u.infoLog("failed to create db connection. tring to retry")
		if mysqlC.IsRunning() {
			u.infoLog(fmt.Sprintf("terminate mysql host: %s, port: %d", host, port.Int()))
			if err = mysqlC.Terminate(ctx); err != nil {
				u.errLog("failed to terminate mysql container", err)
				return nil, err
			}
		}
		mysqlC2, cleanup, err2 := u.createMySQLContainer(ctx, networkName)
		if err2 != nil {
			u.errLog("failed to retry create mysql container", err2)
			cleanup()
			return nil, err2
		}
		host2, err2 := mysqlC2.Host(ctx)
		if err2 != nil {
			u.errLog("failed to retry get host", err2)
			return nil, err2
		}
		port2, err2 := mysqlC2.MappedPort(ctx, dbPortNat)
		if err2 != nil {
			u.errLog("failed to retry get port", err2)
			return nil, err2
		}
		cfg2 := mysql.Config{
			DBName:    dbName,
			User:      "root",
			Addr:      fmt.Sprintf("%s:%d", host2, port2.Int()),
			Net:       "tcp",
			ParseTime: true,
		}
		db, err = gorm.Open(mysql2.Open(cfg2.FormatDSN()))
		if err != nil {
			u.errLog("failed to retry create db connection", err)
			return nil, err
		}
	}
	db.Logger = logger.Discard
	return db, nil
}
