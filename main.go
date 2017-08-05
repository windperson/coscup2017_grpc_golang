package main

import (
	"net"
	"log"
	"google.golang.org/grpc"

	pb "github.com/windperson/coscup2017_grpc_golang/coscup2017_grpc_proto/save_text"
	"google.golang.org/grpc/reflection"

	"gopkg.in/oleiade/lane.v1"
)

type rpcServerImpl struct {
	recognized_results *lane.Queue
}

func (s *rpcServerImpl) SaveResult(req *pb.SaveResultRequest, stream pb.SaveTextService_SaveResultServer) error {

	for s.recognized_results.Head() != nil {

		var entry = s.recognized_results.Dequeue()

		send_data, ok := entry.(pb.SaveResultResponse)

		if !ok { continue}

		if err := stream.Send(&send_data); err != nil {
			return err
		}

	}

	return nil
}


func main() {
	listen, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	rpcImpl :=  rpcServerImpl{
		recognized_results : lane.NewQueue(),
	}

	server := grpc.NewServer()
	pb.RegisterSaveTextServiceServer(server, &rpcImpl)
	reflection.Register(server)

	if err := server.Serve(listen); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
