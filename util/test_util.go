package util

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/go-connections/nat"
	"github.com/go-sql-driver/mysql"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	dbName      = "mysql"
	dbPort      = 3306
	dbPortNat   = nat.Port("3306/tcp")
	mysqlImage  = "mysql:8.0"
	flywayImage = "flyway/flyway:10.17.1"
)

func NewTestDB(ctx context.Context) (*sql.DB, func()) {
	// disable testcontainers log
	testcontainers.Logger = log.New(&ioutils.NopWriter{}, "", 0)

	containerNetwork, err := network.New(ctx)
	if err != nil {
		panic(err)
	}

	mysqlC, terminateDB, err := createMySQLContainer(ctx, containerNetwork.Name)
	if err != nil {
		panic(err)
	}

	mysqlIP, err := mysqlC.ContainerIP(ctx)
	if err != nil {
		panic(err)
	}
	if err = execFlywayContainer(ctx, containerNetwork.Name, mysqlIP); err != nil {
		panic(err)
	}

	db, err := createDBConnection(ctx, mysqlC)
	if err != nil {
		panic(err)
	}
	cleanupFunc := func() {
		terminateDB()
		if err = containerNetwork.Remove(ctx); err != nil {
			panic(err)
		}
	}

	return db, cleanupFunc
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
			WaitingFor:   wait.ForLog("port: 3306  MySQL Community Server"),
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

func execFlywayContainer(ctx context.Context, networkName string, dbContainerIP string) error {
	mysqlDBUrl := fmt.Sprintf("-url=jdbc:mysql://%s:%d/%s?allowPublicKeyRetrieval=true", dbContainerIP, dbPort, dbName)
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
			WaitingFor: wait.ForLog("Successfully applied").WithOccurrence(1),
		},
		Started: true,
	})
	if err != nil {
		return err
	}

	defer func() {
		if flywayC.IsRunning() {
			if err = flywayC.Terminate(ctx); err != nil {
				panic(err)
			}
		}
	}()
	return err
}

func createDBConnection(ctx context.Context, mysqlC testcontainers.Container) (*sql.DB, error) {
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
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	return db, nil
}
