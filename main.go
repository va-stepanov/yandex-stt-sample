package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	stt "github.com/yandex-cloud/go-genproto/yandex/cloud/ai/stt/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	address    = "stt.api.cloud.yandex.net:443"
	bufferSize = 4000
	// Частота отправки аудио в миллисекундах.
	// Для формата LPCM частоту можно рассчитать по формуле: CHUNK_SIZE * 1000 / ( 2 * sampleRateHertz)
	frequency = 250
)

var (
	filePath = os.Args[1]
	folderID = os.Getenv("FOLDER_ID")
	iamToken = os.Getenv("IAM_TOKEN")
)

type Credentials struct {
	token string
}

func (c Credentials) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + c.token,
	}, nil
}

func (c Credentials) RequireTransportSecurity() bool {
	return true
}

func send(spec *stt.RecognitionSpec, recStream stt.SttService_StreamingRecognizeClient) error {
	conf := &stt.RecognitionConfig{Specification: spec, FolderId: folderID}
	reqConf := &stt.StreamingRecognitionRequest_Config{Config: conf}
	request := &stt.StreamingRecognitionRequest{StreamingRequest: reqConf}

	if err := recStream.Send(request); err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, bufferSize)
	for {
		_, err := file.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return err
			}
			if err = recStream.CloseSend(); err != nil {
				return err
			}
			break
		}

		reqCont := &stt.StreamingRecognitionRequest_AudioContent{AudioContent: buffer}
		request := &stt.StreamingRecognitionRequest{StreamingRequest: reqCont}

		err = recStream.Send(request)
		if err != nil {
			return err
		}

		time.Sleep(time.Millisecond * frequency)
	}

	return nil
}

func receive(rspCh chan *stt.StreamingRecognitionResponse, recStream stt.SttService_StreamingRecognizeClient) error {
	for {
		rsp, err := recStream.Recv()
		if err != nil && err != io.EOF {
			return fmt.Errorf("unexpected error %v", err)
		} else if err == io.EOF {
			break
		} else {
			rspCh <- rsp
		}
	}
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(credentials.NewTLS(nil)),
		grpc.WithPerRPCCredentials(Credentials{token: iamToken}))
	if err != nil {
		log.Fatal(err)
		return
	}
	defer conn.Close()

	client := stt.NewSttServiceClient(conn)
	recStream, err := client.StreamingRecognize(ctx)
	if err != nil {
		log.Fatal(err)
	}

	rsp := &stt.StreamingRecognitionResponse{}
	rspCh := make(chan *stt.StreamingRecognitionResponse)
	spec := &stt.RecognitionSpec{
		AudioEncoding:   stt.RecognitionSpec_LINEAR16_PCM,
		SampleRateHertz: 8000,
		LanguageCode:    "ru-RU",
		ProfanityFilter: true,
		Model:           "general",
		PartialResults:  true,
	}

	fatalErrors := make(chan error)
	go func() {
		if err := send(spec, recStream); err != nil {
			fatalErrors <- err
		}
	}()
	go func() {
		if err := receive(rspCh, recStream); err != nil {
			fatalErrors <- err
		}
	}()

Loop:
	for {
		select {
		case rsp = <-rspCh:
			fmt.Printf("Start chunk: ")
			for _, a := range rsp.Chunks[0].Alternatives {
				fmt.Printf("alternative: %s\n", a.Text)
			}
			fmt.Printf("Is final: %v\n", rsp.Chunks[0].Final)
			if rsp.Chunks[0].Final {
				break Loop
			}
		case err := <-fatalErrors:
			log.Fatal(err)
		}
	}

	cancel()
}
