package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	groupingv1 "lp/cmd/data/v1/grouping"

	"google.golang.org/grpc"
)

// groupingServer - gRPC implementation of GroupingService.
type groupingServer struct {
	groupingv1.UnimplementedGroupingServiceServer
}

func main() {
	addr := ":50051"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen: %v\n", err)
		os.Exit(1)
	}

	srv := grpc.NewServer()
	groupingv1.RegisterGroupingServiceServer(srv, &groupingServer{})

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		fmt.Println("shutting down gRPC server...")
		srv.GracefulStop()
	}()

	fmt.Printf("gRPC server listening on %s\n", addr)
	if err := srv.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
