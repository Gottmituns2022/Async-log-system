package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"strings"
	"time"
)

type Server struct {
	ACL          map[string][]string
	AdminManager *AdminManager
}

func (srv *Server) DuplexEvent(event *Event) {
	srv.AdminManager.Mu.Lock()

	for sub := range srv.AdminManager.SubscribersLog {
		//fmt.Printf("SENDING EVENT TO LOG: %v\n", event)
		sub <- event
	}

	for sub := range srv.AdminManager.SubscribersStat {
		//fmt.Printf("SENDING EVENT TO STAT: %v\n", event)
		sub <- event
	}

	srv.AdminManager.Mu.Unlock()
}

func (srv *Server) AccessControl(ctx context.Context, methodName string) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "No metadata")
	}
	consumers := md.Get("consumer")
	if len(consumers) == 0 {
		return "", status.Error(codes.Unauthenticated, "No consumer")
	}
	consumerName := consumers[0]

	consumerMethodSlice, ok := srv.ACL[consumerName]
	if !ok {
		return "", status.Error(codes.Unauthenticated, "Unknown consumer")
	}

	var methodAllowed bool
	for _, method := range consumerMethodSlice {
		if strings.Contains(method, methodName) || strings.Contains(method, "*") {
			methodAllowed = true
		}
	}
	if !methodAllowed {
		return "", status.Error(codes.Unauthenticated, "Access denied")
	}

	return consumerName, nil
}

func (srv *Server) SingleCallInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {

	method := info.FullMethod
	parts := strings.Split(method, "/")
	methodName := parts[2]
	consumerName, err := srv.AccessControl(ctx, methodName)
	if err != nil {
		return nil, err
	}

	hostName := "127.0.0.1:8082"
	peer, ok := peer.FromContext(ctx)
	if ok {
		hostName = peer.Addr.String()
	}

	event := &Event{
		Timestamp: time.Now().Unix(),
		Consumer:  consumerName,
		Method:    method,
		Host:      hostName,
	}

	srv.DuplexEvent(event)

	return handler(ctx, req)
}

func (srv *Server) StreamInterceptor(
	req interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler) error {

	method := info.FullMethod
	parts := strings.Split(method, "/")
	methodName := parts[2]

	consumerName, err := srv.AccessControl(ss.Context(), methodName)
	if err != nil {
		return err
	}

	hostName := "127.0.0.1:8082"
	peer, ok := peer.FromContext(ss.Context())
	if ok {
		hostName = peer.Addr.String()
	}
	event := &Event{
		Timestamp: time.Now().Unix(),
		Consumer:  consumerName,
		Method:    method,
		Host:      hostName,
	}

	srv.DuplexEvent(event)

	return handler(req, ss)
}

func StartMyMicroservice(ctx context.Context, listenAddr string, ACL string) error {
	ACLmap := make(map[string][]string)
	err := json.Unmarshal([]byte(ACL), &ACLmap)
	if len(ACLmap) == 0 {
		return fmt.Errorf("No data in ACL %v\n", err)
	}
	if err != nil {
		return fmt.Errorf("Cant parse ACL %v\n", err)
	}
	adminManager := NewAdminManager()
	srv := &Server{ACL: ACLmap, AdminManager: adminManager}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("Cant listen TCP %v\n", err)
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(srv.SingleCallInterceptor),
		grpc.StreamInterceptor(srv.StreamInterceptor),
	)
	RegisterBizServer(server, NewBizManager())
	RegisterAdminServer(server, adminManager)

	errChannel := make(chan error, 1)
	go func() {
		errChannel <- server.Serve(lis)
	}()
	//log.Printf("Starting server on %v\n", lis.Addr())

	go func() {
		select {
		case err = <-errChannel:
		case <-ctx.Done():
			//log.Println("Shutting down server...")
			server.GracefulStop()
		}
	}()

	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	ACLData := `{
	"logger1":          ["/main.Admin/Logging"],
	"logger2":          ["/main.Admin/Logging"],
	"stat1":            ["/main.Admin/Statistics"],
	"stat2":            ["/main.Admin/Statistics"],
	"biz_user":         ["/main.Biz/Check", "/main.Biz/Add"],
	"biz_admin":        ["/main.Biz/*"],
	"after_disconnect": ["/main.Biz/Add"]
}`

	go func() {
		time.Sleep(10 * time.Second)
		cancel()
	}()

	err := StartMyMicroservice(ctx, "127.0.0.1:8082", ACLData)
	if err != nil {
		log.Fatalf("Cant start gRPC server %v\n", err)
	}
}
