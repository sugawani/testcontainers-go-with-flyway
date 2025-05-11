package util

import (
	"context"
	"fmt"

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
	dbContainerName = "mysql"
	dbName          = "mysql"
	dbPort          = 3306
	dbPortNat       = nat.Port("3306/tcp")
	mysqlImage      = "mysql:8.0"
	flywayImage     = "flyway/flyway:11"
)

func NewTestDB(ctx context.Context) (*gorm.DB, error) {
	containerNetwork, err := network.New(ctx)
	if err != nil {
		return nil, err
	}

	mysqlC, err := createMySQLContainer(ctx, containerNetwork.Name)
	if err != nil {
		return nil, err
	}

	if err = execFlywayContainer(ctx, containerNetwork.Name); err != nil {
		return nil, err
	}

	db, err := createDBConnection(ctx, mysqlC)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createMySQLContainer(ctx context.Context, networkName string) (testcontainers.Container, error) {
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
		return nil, err
	}

	return mysqlC, nil
}

func execFlywayContainer(ctx context.Context, networkName string) error {
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
	db, err := gorm.Open(mysql2.New(mysql2.Config{DSN: cfg.FormatDSN()}))
	if err != nil {
		return nil, err
	}
	db.Logger = logger.Discard
	return db, nil
}
