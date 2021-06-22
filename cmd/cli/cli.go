package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli"
	"google.golang.org/grpc/credentials"

	gcclient "github.com/airbusgeo/geocube/client/go"
)

var client gcclient.Client

func setupConnection(ctx context.Context, c *cli.Context, verbose bool) error {
	var err error
	var creds credentials.TransportCredentials
	if !c.Bool("insecure") {
		creds = credentials.NewTLS(&tls.Config{})
	}
	client, err = gcclient.Dial(ctx, c.String("srv"), creds, c.String("apikey"))
	if err != nil {
		return err
	}
	version, err := client.ServerVersion(ctx)
	if err != nil {
		return err
	}
	if verbose {
		log.Println("Connected to Geocube Server " + version)
	}
	return nil
}

func main() {
	ctx := context.Background()
	app := cli.NewApp()
	app.Name = "geocube"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "srv",
			Value: "127.0.0.1:8080",
			Usage: "geocube server endpoint",
		},
		cli.StringFlag{
			Name:  "apikey",
			Value: "",
			Usage: "distribute api key",
		},
		cli.BoolFlag{
			Name:  "insecure",
			Usage: "allow insecure grpc",
		},
	}
	app.Version = "0.2.0"
	app.Before = func(c *cli.Context) error {
		return setupConnection(ctx, c, false)
	}
	app.Commands = []cli.Command{
		{
			Name:    "records",
			Aliases: []string{"r"},
			Usage:   "manage records",
			Subcommands: []cli.Command{
				{
					Name:        "list",
					Usage:       "list records",
					Action:      cliListRecords,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records list --name France --tag tester=geocube",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name-like", Usage: "filter by name (support *, ? and (?i)-suffix for case-insensitivity)"},
						cli.StringFlag{Name: "aoi-path", Usage: "filter records that intersect geojson"},
						cli.StringFlag{Name: "from-time", Usage: "format: yyyy-MM-dd [HH:mm]"},
						cli.StringFlag{Name: "to-time", Usage: "format: yyyy-MM-dd [HH:mm]"},
						cli.StringSliceFlag{Name: "tag", Usage: "format: --tag key=value --tag key2=value2"},
						cli.Int64Flag{Name: "limit", Value: 1000},
						cli.Int64Flag{Name: "page"},
						cli.BoolFlag{Name: "with-aoi", Usage: "load and return AOI (may be big!)"},
						cli.StringSliceFlag{Name: "disp", Usage: "only display specific attributes (among name, id, aoi-id, datetime)"},
					},
				},
				{
					Name:        "create",
					Usage:       "create record",
					Action:      cliCreateRecord,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records create --aoi-id d0d6f8a4-5b9f-4ee4-a53a-75c990e8f890 --name France --time \"2020-12-10 15:55\" --tag tester=geocube",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name", Required: true},
						cli.StringFlag{Name: "aoi-id", Usage: "aoi-id or aoi-path is required"},
						cli.StringFlag{Name: "aoi-path", Usage: "json path"},
						cli.StringFlag{Name: "time", Usage: "format: yyyy-MM-dd [HH:mm]"},
						cli.StringSliceFlag{Name: "tag", Usage: "format: --tag key=value --tag key2=value2"},
					},
				},
				{
					Name:        "delete",
					Usage:       "delete record",
					Action:      cliDeleteRecord,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records delete --id 33799b96-c9e8-4256-b32f-7211e7f8d2ba",
					Flags: []cli.Flag{
						cli.StringSliceFlag{Name: "id", Usage: "format: --id id1 --id id2"},
					},
				},
				{
					Name:        "aoi",
					Usage:       "create aoi",
					Action:      cliCreateAOI,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records aoi --path france.geojson",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "path", Required: true, Usage: "json path"},
					},
				},
				{
					Name:        "get-aoi",
					Usage:       "get an aoi",
					Action:      cliGetAOI,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records get-aoi --id d0d6f8a4-5b9f-4ee4-a53a-75c990e8f890",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true},
					},
				},
				{
					Name:        "add-tags",
					Usage:       "add tags on list of records",
					Action:      cliAddRecordsTags,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records add-tags --records-id d0d6f8a4-5b9f-4ee4-a53a-75c990e8f890,d0d6f8a4-5b9f-4ee4-a53a-75c990e8f892 --tag newTag=newTagValue",
					Flags: []cli.Flag{
						cli.StringSliceFlag{Name: "records-id", Required: true, Usage: "list of record uuid"},
						cli.StringSliceFlag{Name: "tag", Required: true, Usage: "format: --tag key=value --tag key2=value2"},
					},
				},
				{
					Name:        "remove-tags",
					Usage:       "remove tags on list of records",
					Action:      cliRemoveRecordsTags,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure records remove-tags --records-id d0d6f8a4-5b9f-4ee4-a53a-75c990e8f890,d0d6f8a4-5b9f-4ee4-a53a-75c990e8f892 --tags newTag",
					Flags: []cli.Flag{
						cli.StringSliceFlag{Name: "records-id", Required: true, Usage: "list of record uuid"},
						cli.StringSliceFlag{Name: "tags", Required: true, Usage: "list of tag key to delete"},
					},
				},
			},
		},
		{
			Name:    "layouts",
			Aliases: []string{"l"},
			Usage:   "manage layout",
			Subcommands: []cli.Command{
				{
					Name:        "create",
					Usage:       "create layout",
					Action:      cliCreateLayout,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure layouts create --name singleCell --grid-string \"+grid=singlecell +proj=utm +crs=32631 +resolution=10\"",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name", Required: true},
						cli.StringFlag{Name: "grid-string", Required: true},
						cli.IntFlag{Name: "block-size", Value: 256},
						cli.IntFlag{Name: "max-records", Value: 1000},
					},
				},
				{
					Name:        "list",
					Usage:       "list layouts",
					Action:      cliListLayouts,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure layouts list --name-like singleCell",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name-like", Usage: "support *, ? and (?i)-suffix for case-insensitivity"},
					},
				},
				{
					Name:        "tiles",
					Usage:       "tiles an aoi",
					Action:      cliTileAOI,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure layouts tiles --aoi-path france.geojson --crs epsg:2154 --resolution 15 --size-x 256 --size-y 256",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "aoi-path", Required: true, Usage: "json path"},
						cli.StringFlag{Name: "crs", Required: true, Usage: "proj4, wkt, epsg crs"},
						cli.Float64Flag{Name: "resolution", Required: true, Usage: "Resolution in crs"},
						cli.IntFlag{Name: "size-x", Required: true, Usage: "Tile width"},
						cli.IntFlag{Name: "size-y", Required: true, Usage: "Tile height"},
					},
				},
			},
		},
		{
			Name:    "catalog",
			Aliases: []string{"c"},
			Usage:   "access catalog",
			Subcommands: []cli.Command{
				{
					Name:        "get",
					Usage:       "get cube returns a list of files (Gtiff by default)",
					Action:      cliGetCube,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure catalog get --instances-id dc3845d2-d473-4ed9-a916-7fc88d044966 --records-id b02d7741-0636-4a40-90ce-7880b3d2c952,e14c6bba-9126-446a-bbca-87bb7fc396b6 --compression 0",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "instances-id", Required: true, Usage: "list of variable instance uuid, coma-separated"},
						cli.StringFlag{Name: "records-id", Usage: "list of record uuid, coma-separated (cannot be used with from-time, to-time or tag)"},
						cli.StringFlag{Name: "from-time", Usage: "filter by date (format: yyyy-MM-dd [HH:mm])(cannot be used with records-id)"},
						cli.StringFlag{Name: "to-time", Usage: "filter by date (format: yyyy-MM-dd [HH:mm])(cannot be used with records-id)"},
						cli.StringSliceFlag{Name: "tag", Usage: "filter by tag (format: --tag key=value --tag key2=value2)(cannot be used with records-id)"},
						cli.StringFlag{Name: "transform", Required: true, Usage: "gdal-format transform (pix to crs): (ox,sx,0,oy,0,-sy)"},
						cli.IntFlag{Name: "size-x", Required: true},
						cli.IntFlag{Name: "size-y", Required: true},
						cli.IntFlag{Name: "compression", Value: 0, Usage: "compression level (0: uncompressed, 1 to 9: fastest to best compression)"},
						cli.StringFlag{Name: "crs", Required: true, Usage: "proj4, wkt, epsg crs"},
						cli.BoolFlag{Name: "headers-only", Usage: "returns only image headers"},
					},
				},
			},
		},
		{
			Name:    "variables",
			Aliases: []string{"v"},
			Usage:   "manage variables",
			Subcommands: []cli.Command{
				{
					Name:        "create",
					Usage:       "create variable",
					Action:      cliCreateVariable,
					Description: "ex:  ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables create --name sentinel --description sentinel --dformat int16,0,803,3723",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name", Required: true},
						cli.StringFlag{Name: "unit"},
						cli.StringFlag{Name: "description"},
						cli.StringFlag{Name: "bands", Usage: "bandName1,bandName2,..."},
						cli.StringFlag{Name: "dformat", Required: true, Usage: "dtype[bool, int8-16-32 uint8-16-32 float32-64 complex64],nodata,minvalue,maxvalue"},
						cli.StringFlag{Name: "palette"},
						cli.StringFlag{Name: "resampling-alg", Value: "BILINEAR", Usage: "NEAR BILINEAR CUBIC CUBICSPLINE LANCZOS AVERAGE MODE MAX MIN MED Q1 Q3"},
					},
				},
				{
					Name:        "instantiate",
					Usage:       "instantiate variable",
					Action:      cliInstantiateVariable,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables instantiate --name sentinel --id 236e1883-8011-46c4-9d23-d9821f6e4ea5",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true, Usage: "variable id"},
						cli.StringFlag{Name: "name", Required: true},
						cli.StringSliceFlag{Name: "metadata", Usage: "format: --metadata key=value --metadata key2=value2"},
					},
				},
				{
					Name:        "get",
					Usage:       "get variable",
					Description: "Retrieve the variable given its id, its name or one of its instance id (ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables get --name sentinel)",
					Action:      cliGetVariable,
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Usage: "One of id, name or instance-id"},
						cli.StringFlag{Name: "name"},
						cli.StringFlag{Name: "instance-id"},
					},
				},
				{
					Name:        "list",
					Usage:       "list variables",
					Action:      cliListVariables,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables list --name-like sentinel",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name-like", Usage: "support *, ? and (?i)-suffix for case-insensitivity"},
						cli.Int64Flag{Name: "limit", Value: 1000},
						cli.Int64Flag{Name: "page"},
					},
				},
				{
					Name:        "update",
					Usage:       "update variable",
					Action:      cliUpdateVariable,
					Description: "ex:  ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables update --name sentinel --resampling-alg CUBIC",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true, Usage: "Variable ID"},
						cli.StringFlag{Name: "name"},
						cli.StringFlag{Name: "description"},
						cli.StringFlag{Name: "unit"},
						cli.StringFlag{Name: "palette"},
						cli.StringFlag{Name: "resampling-alg", Usage: "NEAR BILINEAR CUBIC CUBICSPLINE LANCZOS AVERAGE MODE MAX MIN MED Q1 Q3"},
					},
				},
				{
					Name:        "update-i",
					Usage:       "update instance",
					Action:      cliUpdateInstance,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables update-i --name sentinel --metadata key=Value",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true, Usage: "Instance ID"},
						cli.StringFlag{Name: "name"},
						cli.StringSliceFlag{Name: "metadata", Usage: "Add or update : --metadata addkey=value --metadata key=newvalue ..."},
						cli.StringSliceFlag{Name: "del-key", Usage: "Delete metadata key : --del-key key1 --del-key key2 ..."},
					},
				},
				{
					Name:        "delete",
					Usage:       "delete variable and all its instances (iif pending)",
					Action:      cliDeleteVariable,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables delete --id 7e485fdb-48e9-498a-8571-0941dc635680",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true, Usage: "Variable ID"},
					},
				},
				{
					Name:        "delete-i",
					Usage:       "delete instance (iif pending)",
					Action:      cliDeleteInstance,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure variables delete-i --id 7e485fdb-48e9-498a-8571-0941dc635682",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true, Usage: "Instance ID"},
					},
				},
			},
		},

		{
			Name:    "palettes",
			Aliases: []string{"p"},
			Usage:   "manage palettes",
			Subcommands: []cli.Command{
				{
					Name:        "create",
					Usage:       "create palettes as a ramp of RGBA colors",
					Action:      cliCreatePalette,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure palettes create --name newPalette --color 0,0,0,0,255",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name", Required: true},
						cli.StringSliceFlag{Name: "color", Required: true, Usage: "rampVal[0-1],r,g,b,a (exemple : --color 0,0,0,0,255 --color 0.5,255,0,0,255 --color 1.0,255,255,0,255)"},
						cli.BoolFlag{Name: "replace", Usage: "Replace palette if already exists"},
					},
				},
			},
		},
		{
			Name:    "operation",
			Aliases: []string{"o"},
			Usage:   "manage datasets",
			Subcommands: []cli.Command{
				{
					Name:        "index",
					Usage:       "index dataset",
					Action:      cliIndexDataset,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation index --record-id 88add3c3-551a-45d8-82f5-3e86f5c60197 --instance-id 3f65b379-9ac4-48b2-ab96-71c3904cff02 --dformat Int16,-10001,-10000,10000  --real-min=-1 --real-max=1 --bands 1 --uri /home/gcollot/Bureau/Geocube/tiffs/dc3845d2-d473-4ed9-a916-7fc88d044966_SENTINEL2B_2018-08-01",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "uri", Required: true},
						cli.BoolFlag{Name: "managed"},
						cli.StringFlag{Name: "record-id", Required: true},
						cli.StringFlag{Name: "instance-id", Required: true},
						cli.StringFlag{Name: "subdir", Usage: "Container subdir"},
						cli.StringFlag{Name: "bands", Usage: "band1,band2...", Value: "1"},
						cli.StringFlag{Name: "dformat", Required: true, Usage: "dtype[auto, int8-16-32 uint8-16-32 float32-64 complex64],nodata,minvalue,maxvalue"},
						cli.Float64Flag{Name: "real-min", Required: true, Usage: "real value of the min (maps to dformat.minvalue)"},
						cli.Float64Flag{Name: "real-max", Required: true, Usage: "real value of the max (maps to dformat.maxvalue)"},
						cli.Float64Flag{Name: "exponent", Value: 1, Usage: "for non-linear scaling between dformat and real-range (1: linear scaling)"},
					},
				},
				{
					Name:        "config",
					Usage:       "configure consolidation",
					Action:      cliConfigConsolidation,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation config --variable-id b8d77e7d-f7ff-4483-96ec-a263a6eb8a53 --dformat Int16,-10001,-10000,10000",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "variable-id", Required: true, Usage: "uuid of a variable"},
						cli.StringFlag{Name: "dformat", Required: true, Usage: "dtype[int8-16-32 uint8-16-32 float32-64 complex64],nodata,minvalue,maxvalue"},
						cli.Float64Flag{Name: "exponent", Value: 1, Usage: "for non-linear scaling between dformat and variable.dformat (1: linear scaling)"},
						cli.BoolFlag{Name: "bands-interleave", Usage: "Interleave bands"},
						cli.IntFlag{Name: "compression", Value: 1, Usage: "0: No, 1: Lossless, 2: Lossy"},
						cli.BoolFlag{Name: "overviews", Usage: "Create overviews"},
						cli.StringFlag{Name: "downsampling-alg", Value: "NEAR", Usage: "NEAR BILINEAR CUBIC CUBICSPLINE LANCZOS AVERAGE MODE MAX MIN MED Q1 Q3 (for overviews)"},
						cli.IntFlag{Name: "storage-class", Value: 0, Usage: "0: STANDARD, 1:INFREQUENT, 2: ARCHIVE, 3:DEEPARCHIVE"},
					},
				},
				{
					Name:        "consolidate",
					Usage:       "consolidate datasets",
					Action:      cliConsolidateDatasets,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation consolidate --name consolidation --instance-id d886fc5f-eabb-4c53-99d6-ae80e66e3b15 --layout-id 05688435-1d45-4fea-a799-a470928b4f4e --records-id  c3419e67-3aef-4cc1-95be-970dfcf7aa25",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name", Required: true},
						cli.StringFlag{Name: "instance-id", Required: true, Usage: "uuid of variable instance"},
						cli.StringFlag{Name: "layout-id", Required: true},
						cli.StringFlag{Name: "records-id", Usage: "list of record uuid, coma-separated (cannot be used with from-time, to-time or tag)"},
						cli.StringFlag{Name: "from-time", Usage: "filter by date (format: yyyy-MM-dd [HH:mm])(cannot be used with records-id)"},
						cli.StringFlag{Name: "to-time", Usage: "filter by date (format: yyyy-MM-dd [HH:mm])(cannot be used with records-id)"},
						cli.StringSliceFlag{Name: "tag", Usage: "filter by tag (format: --tag key=value --tag key2=value2)(cannot be used with records-id)"},
						cli.BoolFlag{Name: "wait", Usage: "wait for the consolidation to finish."},
					},
				},
				{
					Name:        "list",
					Usage:       "list jobs",
					Action:      cliListJobs,
					Description: "ex:  ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation list --name-like consolidation",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name-like", Usage: "support *, ? and (?i)-suffix for case-insensitivity"},
					},
				},
				{
					Name:        "get",
					Usage:       "get job",
					Action:      cliGetJob,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation list --id a87cc346-5e28-4558-ae38-861dcb571dcc",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true},
					},
				},
				{
					Name:        "retry",
					Usage:       "retry job",
					Action:      cliRetryJob,
					Description: "ex:  ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation retry job --id 2f5e8595-52fa-45d9-bc07-1dce5fd8f0da --force-any-state",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true},
						cli.BoolFlag{Name: "wait", Usage: "wait for the consolidation to finish."},
						cli.BoolFlag{Name: "force-any-state", Usage: "Force the retry even when the job is not in a failed state"},
					},
				},
				{
					Name:        "cancel",
					Usage:       "cancel job",
					Action:      cliCancelJob,
					Description: "ex: ./cmd/cli/cli --srv 127.0.0.1:8080 --insecure operation cancel job --id f9db0080-ca61-4d05-b978-979736c45c1f",
					Flags: []cli.Flag{
						cli.StringFlag{Name: "id", Required: true},
						cli.BoolFlag{Name: "wait", Usage: "wait for the consolidation to finish."},
					},
				},
				{
					Name:   "clean",
					Usage:  "clean terminated jobs",
					Action: cliCleanJobs,
					Flags: []cli.Flag{
						cli.StringFlag{Name: "name-like", Usage: "support *, ? and (?i)-suffix for case-insensitivity"},
						cli.StringFlag{Name: "state", Usage: "DONE or FAILED"},
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func cliListRecords(c *cli.Context) {
	var (
		fromT, toT time.Time
		aoi        gcclient.AOI
		err        error
	)
	if c.IsSet("from-time") {
		fromT = mustParseTime(c.String("from-time"))
	}
	if c.IsSet("to-time") {
		toT = mustParseTime(c.String("to-time"))
	}
	if c.IsSet("aoi-path") {
		aoi, err = gcclient.AOIFromFile(c.String("aoi-path"))
		if err != nil {
			log.Fatal(err)
		}
	}

	records, err := client.ListRecords(c.String("name-like"), mustParseDict(c.StringSlice("tag")), aoi, fromT, toT, c.Int("limit"), c.Int("page"), c.Bool("with-aoi"))
	if err != nil {
		log.Fatal(err)
	}
	attributes := c.StringSlice("disp")
	if len(attributes) == 0 {
		for _, record := range records {
			fmt.Println(record.ToString())
		}
	} else {
		for _, record := range records {
			s := []string{}
			for _, attr := range attributes {
				switch attr {
				case "id":
					s = append(s, record.ID)
				case "name":
					s = append(s, record.Name)
				case "aoi-id":
					s = append(s, record.AOIID)
				case "datetime":
					s = append(s, record.Time.Format("2006-01-02T15:04:05"))
				}
			}
			fmt.Println(strings.Join(s, " "))
		}
	}
}

func cliAddRecordsTags(c *cli.Context) {
	recordsUpdated, err := client.AddRecordsTags(c.StringSlice("records-id"), mustParseDict(c.StringSlice("tag")))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Records tags updated: %d", recordsUpdated)
}

func cliRemoveRecordsTags(c *cli.Context) {
	recordsUpdated, err := client.RemoveRecordsTags(c.StringSlice("records-id"), c.StringSlice("tags"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Records tags updated: %d", recordsUpdated)
}

func cliCreateAOI(c *cli.Context) {
	aoi, err := gcclient.AOIFromFile(c.String("path"))
	if err != nil {
		log.Fatal(err)
	}
	id, err := client.CreateAOI(aoi)
	if id == "" {
		log.Fatal(err)
	}

	fmt.Printf("AOI with id=%s", id)
}

func cliGetAOI(c *cli.Context) {
	aoiID := c.String("id")
	aoi, err := client.GetAOI(aoiID)
	if err != nil {
		log.Fatal(err)
	}

	gaoi, err := gcclient.GeometryFromAOI(aoi).MarshalJSON()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("AOI[%s]: %s", aoiID, gaoi)
}

func cliCreateRecord(c *cli.Context) {
	t := time.Now()
	if c.IsSet("time") {
		t = mustParseTime(c.String("time"))
	}

	aoiID := c.String("aoi-id")
	if aoiID == "" {
		g, err := gcclient.AOIFromFile(c.String("aoi-path"))
		if err != nil {
			log.Fatalf("Unable to create aoi from file %s: %v", c.String("aoi-path"), err)
		}
		id, err := client.CreateAOI(g)
		if id == "" {
			log.Fatalf("Unable to create aoi: %v", err)
		}
		aoiID = id
	}

	id, err := client.CreateRecords(c.String("name"), aoiID, []time.Time{t}, mustParseDict(c.StringSlice("tag")))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Records with id=%s", id)
}

func cliDeleteRecord(c *cli.Context) {
	nb, err := client.DeleteRecords(c.StringSlice("id"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d deleted records", nb)
}

func cliCreateLayout(c *cli.Context) {
	var gridFlags []string
	gridParameters := make(map[string]string)
	for _, s := range strings.Split(c.String("grid-string"), " ") {
		if s == "" {
			continue
		}
		if s[0] != '+' {
			log.Fatal("format error (missing '+'): " + s)
		}
		s := s[1:]
		kv := strings.SplitN(s, "=", 2)
		if len(kv) == 1 {
			gridFlags = append(gridFlags, kv[0])
		} else {
			gridParameters[kv[0]] = kv[1]
		}
	}

	id, err := client.CreateLayout(c.String("name"), gridFlags, gridParameters, c.Int64("block-size"), c.Int64("block-size"), c.Int64("max-records"))

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Layout with id=%s", id)
}

func cliListLayouts(c *cli.Context) {
	layouts, err := client.ListLayouts(c.String("name-like"))
	if err != nil {
		log.Fatal(err)
	}
	for _, layout := range layouts {
		log.Print(layout.ToString())
	}
}

func cliTileAOI(c *cli.Context) {
	aoi, err := gcclient.AOIFromFile(c.String("aoi-path"))
	if err != nil {
		log.Fatal(err)
	}

	tiles, err := client.TileAOI(aoi, c.String("crs"), float32(c.Float64("resolution")), int32(c.Int("size-x")), int32(c.Int("size-y")))
	if err != nil {
		log.Fatal(err)
	}

	for _, t := range tiles {
		fmt.Printf("(%f, %f, %f, %f, %f, %f)*[%d, %d] crs=%s\n",
			t.Transform[0], t.Transform[1], t.Transform[2], t.Transform[3], t.Transform[4], t.Transform[5],
			t.Width, t.Height, t.CRS)
	}
}

func cliGetCube(c *cli.Context) {
	var (
		err    error
		cubeit *gcclient.CubeIterator
	)
	instancesID := toSlice(c.String("instances-id"), ",")
	tr := c.String("transform")
	if len(tr) > 2 && tr[0] == '(' && tr[len(tr)-1] == ')' {
		tr = tr[1 : len(tr)-1]
	}
	pixToCrsS := toSlice(tr, ",")
	if len(pixToCrsS) != 6 {
		log.Fatal("Transform must have 6 coma-separeted parameters")
	}
	p2c := [6]float64{}
	for i := 0; i < 6; i++ {
		p2c[i], err = strconv.ParseFloat(pixToCrsS[i], 64)
		if err != nil {
			log.Fatalf("Invalid value for transform: %s (%v)", pixToCrsS[i], err)
		}
	}
	headersOnly := c.Bool("headers-only")

	outputFormat := gcclient.Format_GTiff
	outputExt := "tif"

	if c.IsSet("records-id") {
		recordsID := toSlice(c.String("records-id"), ",")
		cubeit, err = client.GetCubeFromRecords(instancesID, recordsID, c.String("crs"), p2c, c.Int64("size-x"), c.Int64("size-y"), outputFormat, c.Int("compression"), headersOnly)

	} else {
		var fromT, toT time.Time
		if c.IsSet("from-time") {
			fromT = mustParseTime(c.String("from-time"))
		}
		if c.IsSet("to-time") {
			toT = mustParseTime(c.String("to-time"))
		}
		tags := mustParseDict(c.StringSlice("tag"))
		cubeit, err = client.GetCube(instancesID, tags, fromT, toT, c.String("crs"), p2c, c.Int64("size-x"), c.Int64("size-y"), outputFormat, c.Int("compression"), headersOnly)
	}

	if err != nil {
		log.Fatal(err)
	}

	i := 1
	fmt.Printf("Getting %d images from %d datasets\n", cubeit.Header().Count, cubeit.Header().NbDatasets)
	for cubeit.Next() {
		img := cubeit.Value()
		minTime, maxTime := img.Records[0].Time, img.Records[0].Time
		for _, r := range img.Records {
			if r.Time.Before(minTime) {
				minTime = r.Time
			}
			if r.Time.After(maxTime) {
				maxTime = r.Time
			}
		}
		recordTimes := minTime.Format("20060102T150405")
		if minTime != maxTime {
			recordTimes += "_" + maxTime.Format("20060102T150405")
		}

		if img.Err != "" {
			fmt.Printf("Image %d (%s): %s\n", i, recordTimes, img.Err)
		} else {
			shape := img.Shape
			if headersOnly {
				fmt.Printf("Image %d (%s): %dx%dx%d (id=%s)\n", i, recordTimes, shape[0], shape[1], shape[2], img.Records[0].ID)
			} else {
				fmt.Printf("Image %d (%s): %dx%dx%d\n", i, recordTimes, shape[0], shape[1], shape[2])
				if err := ioutil.WriteFile(fmt.Sprintf("%s_%s."+outputExt, img.Records[0].Name, recordTimes), img.Data, os.ModePerm); err != nil {
					log.Fatal(err)
				}
			}
		}
		i++
	}
	if cubeit.Err() != nil {
		log.Fatal(cubeit.Err().Error())
	}
}

func cliCreateVariable(c *cli.Context) {
	dformat, err := gcclient.ToPbDFormat(c.String("dformat"))
	if err != nil {
		log.Fatalf("Parse dformat: %v", err)
	}
	id, err := client.CreateVariable(
		c.String("name"), c.String("unit"), c.String("description"),
		dformat, strings.Split(c.String("bands"), ","),
		c.String("palette"), c.String("resampling-alg"))

	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Variable with id=%s", id)
}

func cliInstantiateVariable(c *cli.Context) {
	id, err := client.InstantiateVariable(c.String("id"), c.String("name"), mustParseDict(c.StringSlice("metadata")))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Instance with id=%s", id)
}

func cliGetVariable(c *cli.Context) {
	if c.IsSet("id") {
		variable, err := client.GetVariable(c.String("id"))
		if err != nil {
			log.Fatal(err)
		}
		log.Print(variable.ToString())
	} else if c.IsSet("instance-id") {
		variable, err := client.GetVariableFromInstanceID(c.String("instance-id"))
		if err != nil {
			log.Fatal(err)
		}
		log.Print(variable.ToString())
	} else if c.IsSet("name") {
		variable, err := client.GetVariableFromName(c.String("name"))
		if err != nil {
			log.Fatal(err)
		}
		log.Print(variable.ToString())
	} else {
		cli.ShowCommandHelpAndExit(c, c.Command.Name, 0)
	}

}

func cliListVariables(c *cli.Context) {
	variables, err := client.ListVariables(c.String("name-like"), c.Int("limit"), c.Int("page"))
	if err != nil {
		log.Fatal(err)
	}
	for i := range variables {
		log.Print(variables[i].ToString())
	}
}

func getIfSet(c *cli.Context, key string) *string {
	if c.IsSet(key) {
		v := c.String(key)
		return &v
	}
	return nil
}

func cliUpdateVariable(c *cli.Context) {
	if err := client.UpdateVariable(c.String("id"), getIfSet(c, "name"), getIfSet(c, "unit"), getIfSet(c, "description"),
		getIfSet(c, "palette"), getIfSet(c, "resamplingAlg")); err != nil {
		log.Fatal(err)
	}
}

func cliUpdateInstance(c *cli.Context) {
	if err := client.UpdateInstance(c.String("id"), getIfSet(c, "name"), mustParseDict(c.StringSlice("metadata")), c.StringSlice("rem-key")); err != nil {
		log.Fatal(err)
	}
}

func cliDeleteVariable(c *cli.Context) {
	id := c.String("id")
	if c.Bool("all") {
		variable, err := client.GetVariable(id)
		if err != nil {
			log.Fatal(err)
		}
		for _, instance := range variable.Instances {
			if err = client.DeleteInstance(instance.Id); err != nil {
				log.Fatal(err)
			}
		}
	}
	if err := client.DeleteVariable(id); err != nil {
		log.Fatal(err)
	}
}

func cliDeleteInstance(c *cli.Context) {
	id := c.String("id")
	if err := client.DeleteInstance(id); err != nil {
		log.Fatal(err)
	}
}

func cliCreatePalette(c *cli.Context) {
	// Parse color
	colors := make([]gcclient.ColorPoint, len(c.StringSlice("color")))
	var err error
	for i, c := range c.StringSlice("color") {
		cs := toSlice(c, ",")
		if len(cs) != 5 {
			log.Fatal("--color must be val[0,1],r,g,b,a with r,g,b,a in [0,255]: " + c)
		}
		var rgba [4]int
		var val float64
		if val, err = strconv.ParseFloat(cs[0], 32); err != nil || val < 0 || val > 1 {
			log.Fatalf("--color val must be in [0,1]: %s %v", cs[0], err)
		}
		for j := 0; j < 4; j++ {
			if rgba[j], err = strconv.Atoi(cs[j+1]); err != nil || rgba[j] < 0 || rgba[j] > 255 {
				log.Fatalf("--color rgba must be in [0,255]: %s %v", cs[j+1], err)
			}
		}
		colors[i] = gcclient.ColorPoint{Value: float32(val), R: uint32(rgba[0]), G: uint32(rgba[1]), B: uint32(rgba[2]), A: uint32(rgba[3])}

	}

	if err := client.CreatePalette(c.String("name"), colors, c.Bool("replace")); err != nil {
		log.Fatal(err)
	}
}

func cliIndexDataset(c *cli.Context) {
	dformat, err := gcclient.ToPbDFormat(c.String("dformat"))
	if err != nil {
		log.Fatal(err)
	}
	strbands := strings.Split(c.String("bands"), ",")
	bands := make([]int64, len(strbands))
	for i, b := range strbands {
		ib, err := strconv.Atoi(b)
		bands[i] = int64(ib)
		if err != nil {
			log.Fatalf("Invalid dataset.bands: %v", err)
		}
	}

	err = client.IndexDataset(c.String("uri"), c.Bool("managed"), c.String("subdir"), c.String("record-id"),
		c.String("instance-id"), bands, dformat, c.Float64("real-min"), c.Float64("real-max"), c.Float64("exponent"))

	if err != nil {
		log.Fatal(err)
	}
}

func cliConfigConsolidation(c *cli.Context) {
	dformat, err := gcclient.ToPbDFormat(c.String("dformat"))
	if err != nil {
		log.Fatal(err)
	}
	err = client.ConfigConsolidation(c.String("variable-id"), dformat, c.Float64("exponent"), c.Bool("bands-interleave"), c.Int("compression"), c.Bool("overviews"), c.String("downsampling-alg"), c.Int("storage-class"))
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Done")
}

func waitJob(id string) {
	lastState := ""
	for {
		job, err := client.GetJob(id)
		if err != nil {
			log.Fatal(err)
		}
		if job.State != lastState {
			log.Print(job.ToString())
		}
		lastState = job.State
		switch job.State {
		case "DONE", "FAILED", "CONSOLIDATIONFAILED", "DONEBUTUNTIDY", "INITIALISATIONFAILED":
			return
		}
		time.Sleep(time.Second)
	}
}

func cliConsolidateDatasets(c *cli.Context) {
	var id string
	var err error
	if c.IsSet("records-id") {
		id, err = client.ConsolidateDatasetsFromRecords(c.String("name"), c.String("instance-id"), c.String("layout-id"), toSlice(c.String("records-id"), ","))
	} else {
		var fromT, toT time.Time
		if c.IsSet("from-time") {
			fromT = mustParseTime(c.String("from-time"))
		}
		if c.IsSet("to-time") {
			toT = mustParseTime(c.String("to-time"))
		}
		tags := mustParseDict(c.StringSlice("tag"))
		id, err = client.ConsolidateDatasetsFromFilters(c.String("name"), c.String("instance-id"), c.String("layout-id"), tags, fromT, toT)
	}

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Job with id=%s", id)

	if c.Bool("wait") {
		waitJob(id)
	}
}

func cliGetJob(c *cli.Context) {
	j, err := client.GetJob(c.String("id"))
	if err != nil {
		log.Fatal(err)
	}
	log.Print(j.ToString())
}

func cliListJobs(c *cli.Context) {
	jobs, err := client.ListJobs(c.String("name-like"))
	if err != nil {
		log.Fatal(err)
	}
	for _, job := range jobs {
		log.Print(job.ToString())
	}
}

func cliRetryJob(c *cli.Context) {
	id := c.String("id")
	if err := client.RetryJob(id, c.Bool("force-any-state")); err != nil {
		log.Fatal(err)
	}

	if c.Bool("wait") {
		waitJob(id)
	}
}

func cliCancelJob(c *cli.Context) {
	id := c.String("id")
	if err := client.CancelJob(id); err != nil {
		log.Fatal(err)
	}

	if c.Bool("wait") {
		waitJob(id)
	}
}

func cliCleanJobs(c *cli.Context) {
	nb, err := client.CleanJobs(c.String("name-like"), c.String("state"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d jobs cleaned", nb)
}

func mustParseTime(ts string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", ts)
	if err != nil {
		var e error
		if t, e = time.Parse("2006-01-02", ts); e != nil {
			log.Fatal("Unrecognized time: " + ts + "(" + err.Error() + ")")
		}
	}
	return t
}

func mustParseDict(ls []string) map[string]string {
	res := map[string]string{}
	for _, s := range ls {
		kv := strings.SplitN(s, "=", 2)
		if len(kv) != 2 {
			log.Fatal("Wrong dict format: must be key=value")
		}
		res[kv[0]] = kv[1]
	}
	return res
}

func toSlice(s, sep string) []string {
	slice := strings.Split(s, sep)
	for i := range slice {
		slice[i] = strings.Trim(slice[i], " ")
	}
	return slice
}
