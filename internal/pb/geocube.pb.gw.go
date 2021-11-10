// Code generated by protoc-gen-grpc-gateway. DO NOT EDIT.
// source: pb/geocube.proto

/*
Package geocube is a reverse proxy.

It translates gRPC into RESTful JSON APIs.
*/
package geocube

import (
	"context"
	"io"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Suppress "imported and not used" errors
var _ codes.Code
var _ io.Reader
var _ status.Status
var _ = runtime.String
var _ = utilities.NewDoubleArray
var _ = metadata.Join

var (
	filter_Geocube_GetXYZTile_0 = &utilities.DoubleArray{Encoding: map[string]int{"instance_id": 0, "x": 1, "y": 2, "z": 3}, Base: []int{1, 1, 2, 3, 4, 0, 0, 0, 0}, Check: []int{0, 1, 1, 1, 1, 2, 3, 4, 5}}
)

func request_Geocube_GetXYZTile_0(ctx context.Context, marshaler runtime.Marshaler, client GeocubeClient, req *http.Request, pathParams map[string]string) (proto.Message, runtime.ServerMetadata, error) {
	var protoReq GetTileRequest
	var metadata runtime.ServerMetadata

	var (
		val string
		ok  bool
		err error
		_   = err
	)

	val, ok = pathParams["instance_id"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "instance_id")
	}

	protoReq.InstanceId, err = runtime.String(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "instance_id", err)
	}

	val, ok = pathParams["x"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "x")
	}

	protoReq.X, err = runtime.Int32(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "x", err)
	}

	val, ok = pathParams["y"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "y")
	}

	protoReq.Y, err = runtime.Int32(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "y", err)
	}

	val, ok = pathParams["z"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "z")
	}

	protoReq.Z, err = runtime.Int32(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "z", err)
	}

	if err := req.ParseForm(); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := runtime.PopulateQueryParameters(&protoReq, req.Form, filter_Geocube_GetXYZTile_0); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	msg, err := client.GetXYZTile(ctx, &protoReq, grpc.Header(&metadata.HeaderMD), grpc.Trailer(&metadata.TrailerMD))
	return msg, metadata, err

}

func local_request_Geocube_GetXYZTile_0(ctx context.Context, marshaler runtime.Marshaler, server GeocubeServer, req *http.Request, pathParams map[string]string) (proto.Message, runtime.ServerMetadata, error) {
	var protoReq GetTileRequest
	var metadata runtime.ServerMetadata

	var (
		val string
		ok  bool
		err error
		_   = err
	)

	val, ok = pathParams["instance_id"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "instance_id")
	}

	protoReq.InstanceId, err = runtime.String(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "instance_id", err)
	}

	val, ok = pathParams["x"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "x")
	}

	protoReq.X, err = runtime.Int32(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "x", err)
	}

	val, ok = pathParams["y"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "y")
	}

	protoReq.Y, err = runtime.Int32(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "y", err)
	}

	val, ok = pathParams["z"]
	if !ok {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "missing parameter %s", "z")
	}

	protoReq.Z, err = runtime.Int32(val)
	if err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "type mismatch, parameter: %s, error: %v", "z", err)
	}

	if err := req.ParseForm(); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := runtime.PopulateQueryParameters(&protoReq, req.Form, filter_Geocube_GetXYZTile_0); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	msg, err := server.GetXYZTile(ctx, &protoReq)
	return msg, metadata, err

}

// RegisterGeocubeHandlerServer registers the http handlers for service Geocube to "mux".
// UnaryRPC     :call GeocubeServer directly.
// StreamingRPC :currently unsupported pending https://github.com/grpc/grpc-go/issues/906.
// Note that using this registration option will cause many gRPC library features to stop working. Consider using RegisterGeocubeHandlerFromEndpoint instead.
func RegisterGeocubeHandlerServer(ctx context.Context, mux *runtime.ServeMux, server GeocubeServer) error {

	mux.Handle("GET", pattern_Geocube_GetXYZTile_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		var stream runtime.ServerTransportStream
		ctx = grpc.NewContextWithServerTransportStream(ctx, &stream)
		inboundMarshaler, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		rctx, err := runtime.AnnotateIncomingContext(ctx, mux, req, "/geocube.Geocube/GetXYZTile", runtime.WithHTTPPathPattern("/v1/catalog/mosaic/{instance_id}/{x}/{y}/{z}/png"))
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}
		resp, md, err := local_request_Geocube_GetXYZTile_0(rctx, inboundMarshaler, server, req, pathParams)
		md.HeaderMD, md.TrailerMD = metadata.Join(md.HeaderMD, stream.Header()), metadata.Join(md.TrailerMD, stream.Trailer())
		ctx = runtime.NewServerMetadataContext(ctx, md)
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}

		forward_Geocube_GetXYZTile_0(ctx, mux, outboundMarshaler, w, req, response_Geocube_GetXYZTile_0{resp}, mux.GetForwardResponseOptions()...)

	})

	return nil
}

// RegisterGeocubeHandlerFromEndpoint is same as RegisterGeocubeHandler but
// automatically dials to "endpoint" and closes the connection when "ctx" gets done.
func RegisterGeocubeHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if cerr := conn.Close(); cerr != nil {
				grpclog.Infof("Failed to close conn to %s: %v", endpoint, cerr)
			}
			return
		}
		go func() {
			<-ctx.Done()
			if cerr := conn.Close(); cerr != nil {
				grpclog.Infof("Failed to close conn to %s: %v", endpoint, cerr)
			}
		}()
	}()

	return RegisterGeocubeHandler(ctx, mux, conn)
}

// RegisterGeocubeHandler registers the http handlers for service Geocube to "mux".
// The handlers forward requests to the grpc endpoint over "conn".
func RegisterGeocubeHandler(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	return RegisterGeocubeHandlerClient(ctx, mux, NewGeocubeClient(conn))
}

// RegisterGeocubeHandlerClient registers the http handlers for service Geocube
// to "mux". The handlers forward requests to the grpc endpoint over the given implementation of "GeocubeClient".
// Note: the gRPC framework executes interceptors within the gRPC handler. If the passed in "GeocubeClient"
// doesn't go through the normal gRPC flow (creating a gRPC client etc.) then it will be up to the passed in
// "GeocubeClient" to call the correct interceptors.
func RegisterGeocubeHandlerClient(ctx context.Context, mux *runtime.ServeMux, client GeocubeClient) error {

	mux.Handle("GET", pattern_Geocube_GetXYZTile_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		inboundMarshaler, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		rctx, err := runtime.AnnotateContext(ctx, mux, req, "/geocube.Geocube/GetXYZTile", runtime.WithHTTPPathPattern("/v1/catalog/mosaic/{instance_id}/{x}/{y}/{z}/png"))
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}
		resp, md, err := request_Geocube_GetXYZTile_0(rctx, inboundMarshaler, client, req, pathParams)
		ctx = runtime.NewServerMetadataContext(ctx, md)
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}

		forward_Geocube_GetXYZTile_0(ctx, mux, outboundMarshaler, w, req, response_Geocube_GetXYZTile_0{resp}, mux.GetForwardResponseOptions()...)

	})

	return nil
}

type response_Geocube_GetXYZTile_0 struct {
	proto.Message
}

func (m response_Geocube_GetXYZTile_0) XXX_ResponseBody() interface{} {
	response := m.Message.(*GetTileResponse)
	return response.Image.Data
}

var (
	pattern_Geocube_GetXYZTile_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1, 2, 2, 1, 0, 4, 1, 5, 3, 1, 0, 4, 1, 5, 4, 1, 0, 4, 1, 5, 5, 1, 0, 4, 1, 5, 6, 2, 7}, []string{"v1", "catalog", "mosaic", "instance_id", "x", "y", "z", "png"}, ""))
)

var (
	forward_Geocube_GetXYZTile_0 = runtime.ForwardResponseMessage
)
