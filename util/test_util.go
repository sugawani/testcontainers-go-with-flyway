package util

import (
	"context"
	"database/sql"
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

	db, err := u.createDBConnection(ctx, mysqlC)
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
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
				Consumers: []testcontainers.LogConsumer{&StdoutLogConsumer{n: u.N}},
			},
			LifecycleHooks: []testcontainers.ContainerLifecycleHooks{
				{
					PostCreates: []testcontainers.ContainerHook{
						func(ctx context.Context, container testcontainers.Container) error {
							ip, err := container.ContainerIP(ctx)
							if err != nil {
								return err
							}
							name, err := container.Name(ctx)
							if err != nil {
								return err
							}
							host, err := container.Host(ctx)
							if err != nil {
								return err
							}
							sessionID := container.SessionID()
							port, err := container.Ports(ctx)
							if err != nil {
								return err
							}
							u.infoLog(fmt.Sprintf("mysql container created, ip: %s, name: %s, host: %s, sessionID: %s, port: %v", ip, name, host, sessionID, port))
							return nil
						},
					},
					PreTerminates: []testcontainers.ContainerHook{
						func(ctx context.Context, container testcontainers.Container) error {
							ip, err := container.ContainerIP(ctx)
							if err != nil {
								return err
							}
							name, err := container.Name(ctx)
							if err != nil {
								return err
							}
							host, err := container.Host(ctx)
							if err != nil {
								return err
							}
							sessionID := container.SessionID()
							port, err := container.Ports(ctx)
							if err != nil {
								return err
							}
							u.infoLog(fmt.Sprintf("mysql container terminated, ip: %s, name: %s, host: %s, sessionID: %s, port: %v", ip, name, host, sessionID, port))
							return nil
						},
					},
				},
			},
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
	sqlDB, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		fmt.Println("failed to open sql", err)
		return nil, err
	}
	if err = sqlDB.Ping(); err != nil {
		fmt.Println("failed to ping sql", err)
		for i := range 3 {
			n := i + 1
			fmt.Printf("retry %d\n", n)
			sqlDB, err = sql.Open("mysql", cfg.FormatDSN())
			if err != nil {
				fmt.Println("failed to open sql retry...", err)
				continue
			}
			break
		}
	}
	db, err := gorm.Open(mysql2.New(mysql2.Config{Conn: sqlDB}))
	if err != nil {
		fmt.Println("failed to open gorm", err)
		return nil, err
	}
	db.Logger = logger.Discard
	return db, nil
}
