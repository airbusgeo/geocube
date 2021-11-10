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

	"github.com/airbusgeo/geocube/interface/database"
	"github.com/airbusgeo/geocube/interface/database/pg"
	"github.com/airbusgeo/geocube/interface/database/pg/secrets"
	"github.com/airbusgeo/geocube/interface/messaging"
	"github.com/airbusgeo/geocube/interface/messaging/pubsub"
	"github.com/airbusgeo/godal"
	"github.com/airbusgeo/osio"
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
	if err := initGDAL(ctx); err != nil {
		return fmt.Errorf("init gdal: %w", err)
	}

	serverConfig, err := newServerAppConfig()
	if err != nil {
		return err
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
		// Connection to pubsub
		if serverConfig.PsEventsTopic != "" {
			publisher, err := pubsub.NewPublisher(ctx, serverConfig.Project, serverConfig.PsEventsTopic)
			if err != nil {
				return fmt.Errorf("pubsub.NewPublisher: %w", err)
			}
			defer publisher.Stop()
			eventPublisher = publisher
		}
		if serverConfig.PsConsolidationsTopic != "" {
			publisher, err := pubsub.NewPublisher(ctx, serverConfig.Project, serverConfig.PsConsolidationsTopic)
			if err != nil {
				return fmt.Errorf("pubsub.NewPublisher: %w", err)
			}
			defer publisher.Stop()
			consolidationPublisher = publisher
		}
		if eventConsumer, err = pubsub.NewConsumer(serverConfig.Project, ""); err != nil {
			return fmt.Errorf("pubsub.NewConsumer: %w", err)
		}
	}

	// Create Geocube Service
	svc, err := svc.New(ctx, db, eventPublisher, consolidationPublisher, serverConfig.IngestionStorage, serverConfig.CatalogWorkers)
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
			code, err := eventConsumer.Consume(*r, eventHandler)
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
		if serverConfig.Local {
			err = srv.ListenAndServe()
		} else {
			err = srv.ListenAndServeTLS("/tls/tls.crt", "/tls/tls.key")
		}
		if err != nil && err != http.ErrServerClosed {
			log.Logger(ctx).Fatal("srv.ListenAndServe", zap.Error(err))
		}
	}()

	<-ctx.Done()
	sctx, cncl := context.WithTimeout(context.Background(), 30*time.Second)
	defer cncl()
	return srv.Shutdown(sctx)
}

func initGDAL(ctx context.Context) error {
	os.Setenv("GDAL_DISABLE_READDIR_ON_OPEN", "EMPTY_DIR")

	godal.RegisterAll()
	if err := godal.RegisterRaster("PNG"); err != nil {
		return err
	}
	gcs, err := osio.GCSHandle(ctx)
	if err != nil {
		return err
	}
	gcsa, err := osio.NewAdapter(gcs,
		osio.BlockSize("1Mb"),
		osio.NumCachedBlocks(500))
	if err != nil {
		return err
	}
	return godal.RegisterVSIAdapter("gs://", gcsa)
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
	local := flag.Bool("local", false, "execute geocube in local environment")
	listenPort := flag.String("port", "8080", "geocube port to use")
	dbConnection := flag.String("dbConnection", "", "database connection (ex: postgresql://postgres:1234@localhost:5432/geocube)")
	dbName := flag.String("dbName", "", "database name")
	dbUser := flag.String("dbUser", "", "database user")
	dbHost := flag.String("dbHost", "", "database host")
	dbPassword := flag.String("dbPassword", "", "database password")
	project := flag.String("project", "", "project name (gcp only/not required in local usage)")
	dbSecretName := flag.String("dbSecretName", "", "database secret name")
	baSecretName := flag.String("baSecretName", "", "bearer authentication secret name")
	psEventsTopic := flag.String("psEventsTopic", "", "pubsub events topic name")
	psConsolidationsTopic := flag.String("psConsolidationsTopic", "", "pubsub consolidations topic name")
	maxConnectionAge := flag.Int("maxConnectionAge", 0, "grpc max age connection")
	ingestionStorage := flag.String("ingestionStorage", "", "ingestion storage strategy (local/gs)")
	workers := flag.Int("workers", 1, "number of parallel workers per catalog request")

	flag.Parse()

	if *listenPort == "" {
		return nil, fmt.Errorf("failed to initialize port application flag")
	}

	if *ingestionStorage == "" {
		return nil, fmt.Errorf("failed to initialize ingestion storage flag")
	}

	return &serverConfig{
		Local:                 *local,
		AppPort:               *listenPort,
		DbConnection:          *dbConnection,
		DbName:                *dbName,
		DbUser:                *dbUser,
		DbHost:                *dbHost,
		DbPassword:            *dbPassword,
		DbSecretName:          *dbSecretName,
		BearerAuthSecretName:  *baSecretName,
		MaxConnectionAge:      *maxConnectionAge,
		IngestionStorage:      *ingestionStorage,
		Project:               *project,
		PsEventsTopic:         *psEventsTopic,
		PsConsolidationsTopic: *psConsolidationsTopic,
		CatalogWorkers:        *workers,
	}, nil
}

type serverConfig struct {
	Project               string
	PsEventsTopic         string
	PsConsolidationsTopic string
	Local                 bool
	AppPort               string
	DbConnection          string
	DbName                string
	DbUser                string
	DbHost                string
	DbPassword            string
	MaxConnectionAge      int
	DbSecretName          string
	BearerAuthSecretName  string
	IngestionStorage      string
	CatalogWorkers        int
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
