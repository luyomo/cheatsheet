package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
//	"runtime"
//	"runtime/debug"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"

	_ "net/http/pprof"
        "net/http"
)

type Config struct {
	Host     string
	Port     uint16
	User     string
	Password string
	ServerID uint32
	Flavor   string
	
	// Binlog position
	BinlogFile string
	BinlogPos  uint32
	
	// GTID
	GTIDSet string
	
	// Mode: position or gtid
	UseGTID bool
}

func main() {
	cfg := parseFlags()

	// Start pprof HTTP server on port 6060
        go func() {
            http.ListenAndServe("localhost:6060", nil)
        }()
	
	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	setupSignalHandler(cancel)
	
	// Start binlog replication
	if err := startBinlogReplication(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() *Config {
	cfg := &Config{}
	
	var port uint
	var serverID uint
        var binlogPos uint

	flag.StringVar(&cfg.Host, "host", "127.0.0.1", "MySQL host")
	flag.UintVar(&port, "port", 3306, "MySQL port")
	flag.StringVar(&cfg.User, "user", "root", "MySQL user")
	flag.StringVar(&cfg.Password, "password", "", "MySQL password")
	flag.UintVar(&serverID, "server-id", 100, "Unique server ID")
	flag.StringVar(&cfg.Flavor, "flavor", "mysql", "MySQL flavor: mysql or mariadb")
	
	// Binlog position
	flag.StringVar(&cfg.BinlogFile, "binlog-file", "mysql-bin.000001", "Binlog file name")
	flag.UintVar(&binlogPos, "binlog-pos", 4, "Binlog position")
	
	// GTID
	flag.StringVar(&cfg.GTIDSet, "gtid-set", "", "GTID set for replication")
	flag.BoolVar(&cfg.UseGTID, "gtid", false, "Use GTID replication")
	
	flag.Parse()

	cfg.Port = uint16(port)
	cfg.ServerID = uint32(serverID)
        cfg.BinlogPos = uint32(binlogPos)
	
	return cfg
}

func setupSignalHandler(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal: %v, shutting down...\n", sig)
		cancel()
	}()
}

func startBinlogReplication(ctx context.Context, cfg *Config) error {
	// Create binlog syncer config
	syncerCfg := replication.BinlogSyncerConfig{
		ServerID:  cfg.ServerID,
		Flavor:    cfg.Flavor,
		Host:      cfg.Host,
		Port:      cfg.Port,
		User:      cfg.User,
		Password:  cfg.Password,
		EventCacheCount: 2048,
	}
	
	// Create binlog syncer
	syncer := replication.NewBinlogSyncer(syncerCfg)
	defer syncer.Close()
	
	fmt.Printf("Starting binlog replication:\n")
	fmt.Printf("  Host: %s:%d\n", cfg.Host, cfg.Port)
	fmt.Printf("  User: %s\n", cfg.User)
	fmt.Printf("  ServerID: %d\n", cfg.ServerID)
	fmt.Printf("  Flavor: %s\n", cfg.Flavor)
	
	var streamer *replication.BinlogStreamer
	var err error
	
	if cfg.UseGTID && cfg.GTIDSet != "" {
		fmt.Printf("  Using GTID: %s\n", cfg.GTIDSet)
		streamer, err = startGTIDReplication(syncer, cfg)
	} else {
		fmt.Printf("  Using binlog position: %s:%d\n", cfg.BinlogFile, cfg.BinlogPos)
		streamer, err = startPositionReplication(syncer, cfg)
	}
	
	if err != nil {
		return fmt.Errorf("failed to start replication: %v", err)
	}
	
	fmt.Println("Replication started successfully, waiting for events...")
	fmt.Println("----------------------------------------")
	
	// Process binlog events
	return processEvents(ctx, streamer)
}

func startGTIDReplication(syncer *replication.BinlogSyncer, cfg *Config) (*replication.BinlogStreamer, error) {
	var flavor string
	switch cfg.Flavor {
	case "mysql":
		flavor = mysql.MySQLFlavor
	case "mariadb":
		flavor = mysql.MariaDBFlavor
	default:
		flavor = mysql.MySQLFlavor
	}
	
	gtidSet, err := mysql.ParseGTIDSet(flavor, cfg.GTIDSet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GTID set: %v", err)
	}
	
	return syncer.StartSyncGTID(gtidSet)
}

func startPositionReplication(syncer *replication.BinlogSyncer, cfg *Config) (*replication.BinlogStreamer, error) {
	position := mysql.Position{
		Name: cfg.BinlogFile,
		Pos:  cfg.BinlogPos,
	}
	
	return syncer.StartSync(position)
}

func processEvents(ctx context.Context, streamer *replication.BinlogStreamer) error {
	eventCount := 0
	
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, stopping replication")
			return nil
		default:
//			if eventCount % 1000 == 0 {
//				runtime.GC()
//				debug.FreeOSMemory()
//			}
			// Use timeout to allow checking context cancellation
			event, err := getEventWithTimeout(ctx, streamer, 2*time.Second)
			if err != nil {
				if err == context.DeadlineExceeded {
					continue
				}
				if err == context.Canceled {
					return nil
				}
				return fmt.Errorf("failed to get event: %v", err)
			}
			
			if event != nil {
				eventCount++
				fmt.Printf("=== Event #%d ===\n", eventCount)
				// time.Sleep(500 * time.Millisecond)
				event.Dump(os.Stdout)
				fmt.Println("----------------------------------------")
				event = nil
			}
		}
	}
}

func getEventWithTimeout(ctx context.Context, streamer *replication.BinlogStreamer, timeout time.Duration) (*replication.BinlogEvent, error) {
	eventCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	return streamer.GetEvent(eventCtx)
}
