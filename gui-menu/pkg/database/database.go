package database

import (
	"time"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-sql-driver/mysql"
	"fmt"
)

type Database struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string

	Connection *sql.DB
}

func (db *Database) Init(useTLS bool) error {
	var connStr string
	if useTLS {
		rootCertPool := x509.NewCertPool()
		caCertPath := "/etc/ssl/certs/custom/isrgrootx1.pem" // Adjust this path
		caCert, err := ioutil.ReadFile(caCertPath)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate: %v", err)
		}
		if !rootCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to append CA certificate")
		}
	
		// Configure TLS settings
		tlsConfig := &tls.Config{
			RootCAs: rootCertPool,
		}
	
		// Register TLS config with MySQL driver
		if err := mysql.RegisterTLSConfig("custom", tlsConfig); err != nil {
			return fmt.Errorf("failed to register TLS config: %v", err)
		}
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?tls=custom", db.User, db.Password, db.Host, db.Port, db.DBName)
	} else {
		connStr = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", db.User, db.Password, db.Host, db.Port, db.DBName)
	}

	var err error
	db.Connection, err = sql.Open("mysql", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %v", err)
	}

	// Test the connection
	if err = db.Connection.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// See "Important settings" section.
	db.Connection.SetConnMaxLifetime(time.Minute * 3)
	db.Connection.SetMaxOpenConns(10)
	db.Connection.SetMaxIdleConns(10)

	return nil
}
