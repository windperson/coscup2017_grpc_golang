package main

import (
	"io"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	pb "github.com/windperson/coscup2017_grpc_golang/coscup2017_grpc_proto/save_text"
	"google.golang.org/grpc/reflection"

	"time"

	"gopkg.in/oleiade/lane.v1"

	"cloud.google.com/go/speech/apiv1"

	"golang.org/x/net/context"

	"sync"

	"github.com/golang/protobuf/ptypes/timestamp"
	speechpb "google.golang.org/genproto/googleapis/cloud/speech/v1"
)

func main() {

	var wg sync.WaitGroup

	wg.Add(1)
	go serveSaveText(&wg)

	wg.Add(1)
	go invokeStreamSpeechAPI(&wg)

	wg.Wait()
}

func serveSaveText(wg *sync.WaitGroup) {
	defer wg.Done()
	listen, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	pb.RegisterSaveTextServiceServer(server, &rpcImpl)
	reflection.Register(server)

	if err := server.Serve(listen); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

type rpcServerImpl struct {
	recognizeds *lane.Queue
}

var rpcImpl = rpcServerImpl{
	recognizeds: lane.NewQueue(),
}

func (s *rpcServerImpl) SaveResult(req *pb.SaveResultRequest, stream pb.SaveTextService_SaveResultServer) error {

	logSent := log.New(os.Stderr, "", 0)

	for s.recognizeds.Head() != nil {

		var entry = s.recognizeds.Dequeue()


		sendData, ok := entry.(*pb.SaveResultResponse)
		if !ok {
			logSent.Println("should be able to cast")
			continue
		}

		logSent.Println("save: %+v", sendData)
		if err := stream.Send(sendData); err != nil {
			return err
		}

	}
	return nil
}

func invokeStreamSpeechAPI(wg *sync.WaitGroup) {

	defer wg.Done()

	for {

		bgCtx := context.Background()
		ctx, _ := context.WithDeadline(bgCtx, time.Now().Add(205*time.Second))

		client, err := speech.NewClient(ctx)
		if err != nil {
			log.Fatal(err)
		}

		stream, err := client.StreamingRecognize(ctx)
		if err != nil {
			log.Fatal(err)
		}

		exit := make(chan struct{})

		// Send the initial configuration message.
		os.Stderr.WriteString("sending init StreamingConfig...\n")

		if err := stream.Send(&speechpb.StreamingRecognizeRequest{

			StreamingRequest: &speechpb.StreamingRecognizeRequest_StreamingConfig{
				StreamingConfig: &speechpb.StreamingRecognitionConfig{

					Config: &speechpb.RecognitionConfig{
						Encoding:        speechpb.RecognitionConfig_LINEAR16,
						SampleRateHertz: 16000,
						LanguageCode:    "en-US", /* see support lang here:
						 https://cloud.google.com/speech/docs/languages
						"cmn-Hans-CN", "cmn-Hant-TW", "ja-JP" damn slow, only english the fastest.*/
					},
					SingleUtterance: false,
					InterimResults:  true,
				},
			},
		}); err != nil {
			log.Fatal(err)
		}

		go func() {

			sl := log.New(os.Stderr, "", 0)
			sl.Println("start sending to Speech API...")

			// Pipe stdin to the API.
			buf := make([]byte, 1024)

			for {
				select {
				case <-exit:
					return
				default:
					n, err := os.Stdin.Read(buf)
					if err == io.EOF {

						// Nothing else to pipe, close the stream.
						if err := stream.CloseSend(); err != nil {
							sl.Printf("Could not close stream: %v", err)
						}
						return
					}

					if err != nil {
						sl.Printf("Could not read from stdin: %v", err)
						continue
					}

					if err = stream.Send(&speechpb.StreamingRecognizeRequest{
						StreamingRequest: &speechpb.StreamingRecognizeRequest_AudioContent{
							AudioContent: buf[:n],
						},
					}); err != nil {
						sl.Printf("Could not send audio: %v", err)
						return
					}
				}
			}

		}()

		rl := log.New(os.Stderr, "", 0)

		for {
			resp, err := stream.Recv()

			if err == io.EOF {
				stream.CloseSend()
				return
			}

			if err != nil {
				stream.CloseSend()
				rl.Fatalf("Cannot stream results: %v", err)
			}

			if err := resp.Error; err != nil {
				rl.Printf("Could not recognize: %v", err)
				time.Sleep(1000 * time.Millisecond)
				rl.Println("re initialize conneciton")
				stream.CloseSend()
				close(exit)
				break
			}

			for _, result := range resp.Results {
				if result.IsFinal {
					var timeNow = time.Now()
					for _, alternate := range result.Alternatives {
						rl.Printf("\n\nGOT: { %s } ,\ncorrect= %f %%\n\n",
							alternate.Transcript, alternate.Confidence)

						var saveItem = &pb.SaveResultResponse{
							ClientId:   1,
							Recognized: alternate.Transcript,
							Timestamp: &timestamp.Timestamp{
								Seconds: timeNow.Unix(),
								Nanos:   int32(timeNow.Nanosecond()),
							},
						}

						if rpcImpl.recognizeds.Full() {
							rl.Fatalf("recognized buffer fulled")
						}
						rpcImpl.recognizeds.Enqueue(saveItem)
						rl.Printf("recognized buffer length=%d", rpcImpl.recognizeds.Size())
					}
					continue
				}
				rl.Printf("%s receive= %+v\n", time.Now().Format(time.RFC850), result)
			}
		}
	}

}
