package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/airbusgeo/geocube/cmd"
	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/interface/database/pg"
	"github.com/airbusgeo/geocube/interface/database/pg/secrets"
	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/messaging/pgqueue"
	"github.com/airbusgeo/geocube/interface/messaging/pubsub"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/airbusgeo/geocube/internal/geocube"
	geogrpc "github.com/airbusgeo/geocube/internal/grpc"
	"github.com/airbusgeo/geocube/internal/log"
	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/svc"
	"github.com/airbusgeo/geocube/internal/utils"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	runerr := make(chan error)

	go func() {
		runerr <- run(ctx)
	}()

	for {
		select {
		case err := <-runerr:
			if err != nil {
				log.Logger(ctx).Fatal("run error", zap.Error(err))
			}
			return
		case <-quit:
			cancel()
			go func() {
				time.Sleep(30 * time.Second)
				runerr <- fmt.Errorf("did not terminate after 30 seconds")
			}()
		}
	}
}

func isGrpcRequest(r *http.Request) bool {
	return r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc")
}

func isFromPubsub(req *http.Request) bool {
	//Fixme?
	return true
}

func handleError(ctx context.Context, w http.ResponseWriter, req *http.Request, code int, err error) {
	w.Header().Add("Content-Type", "text/plain")

	if utils.Temporary(err) {
		log.Logger(ctx).Warn("temporary error: "+err.Error(), zap.Error(err))
		w.WriteHeader(code)
	} else {
		log.Logger(ctx).Error("error: "+err.Error(), zap.Error(err))
		if isFromPubsub(req) {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(code)
		}
	}
	fmt.Fprint(w, err.Error())
}

func run(ctx context.Context) error {
	serverConfig, err := newServerAppConfig()
	if err != nil {
		return err
	}

	if err := cmd.InitGDAL(ctx, serverConfig.GDALConfig); err != nil {
		return fmt.Errorf("init gdal: %w", err)
	}

	// Connect to database
	var db database.GeocubeDBBackend
	{
		// Connection to postgresql
		dbConnection, err := PgConnString(ctx, serverConfig)
		if err != nil {
			return fmt.Errorf("pg.GetConnString: %w", err)
		}
		if db, err = pg.New(ctx, dbConnection); err != nil {
			return fmt.Errorf("pg.new: %w", err)
		}
	}

	// Create Messaging Service
	var eventPublisher, consolidationPublisher messaging.Publisher
	var eventConsumer messaging.Consumer
	{
		if serverConfig.PgqDbConnection != "" {
			// Connection to pgqueue
			db, w, err := pgqueue.SqlConnect(ctx, serverConfig.PgqDbConnection)
			if err != nil {
				return fmt.Errorf("pgqueue.Open: %w", err)

			}
			if serverConfig.EventsQueue != "" {
				eventPublisher = pgqueue.NewPublisher(w, serverConfig.EventsQueue)
				consumer := pgqueue.NewConsumer(db, serverConfig.EventsQueue)
				defer consumer.Stop()
				eventConsumer = consumer
			}
			if serverConfig.ConsolidationsQueue != "" {
				consolidationPublisher = pgqueue.NewPublisher(w, serverConfig.ConsolidationsQueue)
			}
		} else if serverConfig.Project != "" {
			// Connection to pubsub
			if serverConfig.EventsQueue != "" {
				publisher, err := pubsub.NewPublisher(ctx, serverConfig.Project, serverConfig.EventsQueue)
				if err != nil {
					return fmt.Errorf("pubsub.NewPublisher: %w", err)
				}
				defer publisher.Stop()
				eventPublisher = publisher
			}
			if serverConfig.ConsolidationsQueue != "" {
				publisher, err := pubsub.NewPublisher(ctx, serverConfig.Project, serverConfig.ConsolidationsQueue)
				if err != nil {
					return fmt.Errorf("pubsub.NewPublisher: %w", err)
				}
				defer publisher.Stop()
				consolidationPublisher = publisher
			}
		}
	}

	// Create Geocube Service
	svc, err := svc.New(ctx, db, eventPublisher, consolidationPublisher, serverConfig.IngestionStorage, serverConfig.CancelledConsolidationStorage, serverConfig.CatalogWorkers)
	if err != nil {
		return fmt.Errorf("svc.new: %w", err)
	}

	eventHandler := func(ctx context.Context, m *messaging.Message) error {
		evt, err := geocube.UnmarshalEvent(bytes.NewReader(m.Data))
		if err != nil {
			return err
		}
		return svc.HandleEvent(ctx, evt)
	}

	grpcServer := newGrpcServer(svc, svc, serverConfig.MaxConnectionAge)

	log.Logger(ctx).Info("Geocube v" + geogrpc.GeocubeServerVersion)

	gwmuxHandler := newGatewayHandler(ctx, svc, serverConfig.MaxConnectionAge)

	muxHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isGrpcRequest(r) {
			grpcServer.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/push") {
			if err := authenticate([]string{eventTokenKey}, []string{r.Header.Get(utils.AuthorizationHeader)}); err != nil {
				w.WriteHeader(401)
				fmt.Fprint(w, err.Error())
				return
			}
			code, err := messaging.Consume(*r, eventHandler)
			if err != nil {
				handleError(ctx, w, r, code, err)
			} else {
				w.WriteHeader(code)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/catalog") {
			w.Header().Add("Access-Control-Allow-Origin", "*")
			if r.Method == "OPTIONS" {
				w.Header().Add("Access-Control-Allow-Methods", "OPTIONS, GET")
				w.Header().Add("Access-Control-Allow-Headers", utils.AuthorizationHeader+","+utils.ESRIAuthorizationHeader)
				w.WriteHeader(200)
				return
			}
			if err := authenticate([]string{userTokenKey}, []string{r.Header.Get(utils.AuthorizationHeader), r.Header.Get(utils.ESRIAuthorizationHeader)}); err != nil {
				w.WriteHeader(401)
				fmt.Fprint(w, err.Error())
				return
			}
			r.Header.Add("Accept", "image/png")
			gwmuxHandler.ServeHTTP(w, r)
			return
		}
		fmt.Fprintf(w, "ok")
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", serverConfig.AppPort),
		Handler: h2c.NewHandler(muxHandler, &http2.Server{}),
	}

	bearerAuths = map[string]utils.TokenAuth{}
	if serverConfig.BearerAuthSecretName != "" {
		if bearerAuths, err = loadBearerAuths(ctx, serverConfig.Project, serverConfig.BearerAuthSecretName); err != nil {
			return err
		}
	}

	go func() {
		var err error
		if serverConfig.TLS && !serverConfig.Local {
			err = srv.ListenAndServeTLS("/tls/tls.crt", "/tls/tls.key")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Logger(ctx).Fatal("srv.ListenAndServe", zap.Error(err))
		}
	}()

	go func() {
		if eventConsumer != nil {
			if err := eventConsumer.Pull(ctx, eventHandler); err != nil {
				log.Logger(ctx).Fatal("eventConsumer.Pull", zap.Error(err))
			}
		}
	}()

	<-ctx.Done()
	sctx, cncl := context.WithTimeout(context.Background(), 30*time.Second)
	defer cncl()
	return srv.Shutdown(sctx)
}

func getMaxConnectionAge(maxConnectionAge int) int {
	if maxConnectionAge < 60 {
		maxConnectionAge = 15 * 60
	}
	return maxConnectionAge
}

func newGrpcServer(svc geogrpc.GeocubeService, asvc geogrpc.GeocubeServiceAdmin, maxConnectionAgeValue int) *grpc.Server {

	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      time.Duration(getMaxConnectionAge(maxConnectionAgeValue)) * time.Second,
			MaxConnectionAgeGrace: time.Minute}),
		grpc.UnaryInterceptor(authUnaryInterceptor),
		grpc.StreamInterceptor(authStreamInterceptor)}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterGeocubeServer(grpcServer, geogrpc.New(svc, getMaxConnectionAge(maxConnectionAgeValue)))
	pb.RegisterAdminServer(grpcServer, geogrpc.NewAdmin(asvc))
	return grpcServer
}

func newGatewayHandler(ctx context.Context, svc geogrpc.GeocubeService, maxConnectionAgeValue int) *runtime.ServeMux {
	gwmux := runtime.NewServeMux(runtime.WithMarshalerOption("image/png", pngMarshaler{}))
	pb.RegisterGeocubeHandlerServer(ctx, gwmux, geogrpc.New(svc, getMaxConnectionAge(maxConnectionAgeValue)))
	return gwmux
}

type pngMarshaler struct{}

func (pm pngMarshaler) Marshal(v interface{}) ([]byte, error) {
	if bytes, ok := v.([]byte); ok {
		return bytes, nil
	}
	jsonpb := runtime.JSONPb{}
	return jsonpb.Marshal(v)
}

func (pm pngMarshaler) ContentType(v interface{}) string {
	return "image/png"
}

func (pm pngMarshaler) Unmarshal(data []byte, v interface{}) error {
	return nil
}

func (pm pngMarshaler) NewDecoder(r io.Reader) runtime.Decoder {
	return nil
}

func (pm pngMarshaler) NewEncoder(w io.Writer) runtime.Encoder {
	return nil
}

func newServerAppConfig() (*serverConfig, error) {
	serverConfig := serverConfig{}
	// Configuration
	flag.StringVar(&serverConfig.AppPort, "port", "8080", "geocube port to use")
	flag.BoolVar(&serverConfig.TLS, "tls", false, "enable TLS protocol")
	flag.IntVar(&serverConfig.MaxConnectionAge, "maxConnectionAge", 0, "grpc max age connection")
	flag.IntVar(&serverConfig.CatalogWorkers, "workers", 1, "number of parallel workers per catalog request")
	flag.StringVar(&serverConfig.CancelledConsolidationStorage, "cancelledJobs", "", "storage where cancelled jobs are referenced. Must be reachable by the Consolidation Workers and the Geocube with read/write permissions")
	flag.StringVar(&serverConfig.IngestionStorage, "ingestionStorage", "", "path to the storage where ingested and consolidated datasets will be stored. Must be reachable with read/write/delete permissions. (local/gs)")

	// BearerAuth
	flag.StringVar(&serverConfig.BearerAuthSecretName, "baSecretName", "", "name of the secret that stores the bearer authentication (admin & user) (gcp only)")

	// Database
	flag.StringVar(&serverConfig.DbConnection, "dbConnection", "", "database connection (ex: postgresql://user:password@localhost:5432/geocube)")
	flag.StringVar(&serverConfig.DbName, "dbName", "", "database name (to connect with User, Host & Password)")
	flag.StringVar(&serverConfig.DbUser, "dbUser", "", "database user (see dbName)")
	flag.StringVar(&serverConfig.DbHost, "dbHost", "", "database host (see dbName)")
	flag.StringVar(&serverConfig.DbPassword, "dbPassword", "", "database password (see dbName)")
	flag.StringVar(&serverConfig.DbSecretName, "dbSecretName", "", "name of the secret that stores credentials to connect to the database (gcp only)")

	// Messaging
	flag.StringVar(&serverConfig.Project, "project", "", "project name (gcp only/not required in local usage)")
	flag.StringVar(&serverConfig.PgqDbConnection, "pgqConnection", "", "url of the postgres database to enable pgqueue messaging system (pgqueue only)")
	flag.StringVar(&serverConfig.EventsQueue, "eventsQueue", "", "name of the pgqueue or the pubsub topic to send the asynchronous job events")
	flag.StringVar(&serverConfig.ConsolidationsQueue, "consolidationsQueue", "", "name of the pgqueue or the pubsub topic to send the consolidation orders")

	// GDAL
	serverConfig.GDALConfig = cmd.GDALConfigFlags()

	flag.Parse()

	serverConfig.GDALConfig.RegisterPNG = true

	if serverConfig.AppPort == "" {
		return nil, fmt.Errorf("failed to initialize --port application flag")
	}

	if serverConfig.CancelledConsolidationStorage == "" && serverConfig.IngestionStorage != "" {
		return nil, fmt.Errorf("missing --cancelledJobs storage flag")
	}

	return &serverConfig, nil
}

type serverConfig struct {
	Project                       string
	EventsQueue                   string
	ConsolidationsQueue           string
	PgqDbConnection               string
	Local                         bool
	TLS                           bool
	AppPort                       string
	DbConnection                  string
	DbName                        string
	DbUser                        string
	DbHost                        string
	DbPassword                    string
	MaxConnectionAge              int
	DbSecretName                  string
	BearerAuthSecretName          string
	IngestionStorage              string
	CancelledConsolidationStorage string
	CatalogWorkers                int
	GDALConfig                    *cmd.GDALConfig
}

func PgConnString(ctx context.Context, serverConfig *serverConfig) (string, error) {
	if serverConfig.DbConnection != "" {
		return serverConfig.DbConnection, nil
	}

	if serverConfig.DbPassword != "" {
		return pg.ConnStringFromId(serverConfig.DbName, serverConfig.DbUser, serverConfig.DbHost, serverConfig.DbPassword)
	}

	if serverConfig.DbSecretName == "" {
		return "", fmt.Errorf("missing secretName flag")
	}

	if serverConfig.Project == "" {
		return "", fmt.Errorf("missing project flag")
	}

	scl, err := secrets.NewClient(ctx)
	if err != nil {
		return "", fmt.Errorf("gsecrets.new: %w", err)
	}
	credsb, err := scl.GetSecret(ctx, serverConfig.Project, serverConfig.DbSecretName)
	if err != nil {
		return "", fmt.Errorf("getsecret %s/%s: %w", serverConfig.Project, serverConfig.DbSecretName, err)
	}

	dec := json.NewDecoder(bytes.NewReader(credsb))
	dec.DisallowUnknownFields()
	creds := pg.Credentials{}
	if err = dec.Decode(&creds); err != nil {
		return "", fmt.Errorf("json.decode credentials: %w", err)
	}

	return pg.ConnStringFromCredentials(serverConfig.DbName, serverConfig.DbUser, serverConfig.DbHost, creds)
}
